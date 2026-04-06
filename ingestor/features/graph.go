package features

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"graph-fraud/ingestor"
)

// TxEdge is a directed spend edge: From is the funding tx, To spends that output
// (see GRAPH_PATTERNS.md: A to B means B spends an output of A).
type TxEdge struct {
	From   string
	To     string
	Amount int64 // satoshis
}

// Subgraph holds partial transaction graph fragment (edge list).
type Subgraph struct {
	Edges       []TxEdge
}

const subgraphChanCap = 1024

var (
	// SubgraphRegistryMu guards Subgraphs.
	SubgraphRegistryMu sync.Mutex
	Subgraphs          map[int]*Subgraph

	// nextGraphID is strictly increasing for every new graph object (subgraphs and future merges).
	nextGraphID atomic.Uint64
	// numGraphs counts subgraphs that have been registered and queued (merge workers decrement when combining).
	numGraphs atomic.Int64

	pipelineOnce sync.Once
	subgraphIDs  chan int
)

func initPipeline() {
	pipelineOnce.Do(func() {
		Subgraphs = make(map[int]*Subgraph)
		subgraphIDs = make(chan int, subgraphChanCap)
		nextGraphID.Store(0)
	})
}

// AllocGraphID returns the next unique graph id. Use for new subgraphs and for merged graphs.
func AllocGraphID() int {
	initPipeline()
	return int(nextGraphID.Add(1))
}

// ActiveSubgraphCount is the number of subgraphs registered
func ActiveSubgraphCount() int64 {
	initPipeline()
	return numGraphs.Load()
}

// SubgraphIDQueue returns the receive side of the subgraph id queue
func SubgraphIDQueue() <-chan int {
	initPipeline()
	return subgraphIDs
}

// EnqueueSubgraphID sends a graph id on the merge queue (e.g. after combining two subgraphs). Same channel ProcessBlock writes to.
func EnqueueSubgraphID(ctx context.Context, id int) error {
	initPipeline()
	select {
	case subgraphIDs <- id:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

const zeroTxid = "0000000000000000000000000000000000000000000000000000000000000000"

type txWire struct {
	Hash   string `json:"hash"`
	Inputs []struct {
		PrevOut *struct {
			Hash  string `json:"hash"`
			Value int64  `json:"value"` // satoshis (blockchain.info prev_out)
		} `json:"prev_out"`
	} `json:"inputs"`
}

type edgeKey struct {
	From string
	To   string
}

// buildSubgraph parses block txs into spend edges (funding tx → spending tx).
// Multiple inputs that map to the same (From, To) pair add their prev_out values into Amount.
func buildSubgraph(fb *ingestor.FetchedBlock) (*Subgraph, error) {
	agg := make(map[edgeKey]int64)

	for _, raw := range fb.Block.Tx {
		var tw txWire
		if err := json.Unmarshal(raw, &tw); err != nil {
			return nil, fmt.Errorf("decode tx: %w", err)
		}
		if tw.Hash == "" {
			continue
		}
		for _, in := range tw.Inputs {
			if in.PrevOut == nil {
				continue
			}
			from := in.PrevOut.Hash
			if from == "" || from == zeroTxid {
				continue
			}
			k := edgeKey{From: from, To: tw.Hash}
			agg[k] += in.PrevOut.Value
		}
	}

	edges := make([]TxEdge, 0, len(agg))
	for k, amt := range agg {
		edges = append(edges, TxEdge{From: k.From, To: k.To, Amount: amt})
	}

	return &Subgraph{
		Edges: edges,
	}, nil
}

// ProcessBlock builds a subgraph for one block, assigns a new graph id, stores it, increments the
// active subgraph counter, and enqueues the id for merge workers.
func ProcessBlock(ctx context.Context, fb *ingestor.FetchedBlock) error {
	initPipeline()

	sg, err := buildSubgraph(fb)
	if err != nil {
		return err
	}

	id := AllocGraphID()

	SubgraphRegistryMu.Lock()
	Subgraphs[id] = sg
	SubgraphRegistryMu.Unlock()

	numGraphs.Add(1)

	select {
	case subgraphIDs <- id:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}