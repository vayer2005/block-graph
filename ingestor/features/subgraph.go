package features

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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
	Edges []TxEdge
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

// coinbasePrevOutN is the prev_out output index Bitcoin uses for coinbase inputs.
const coinbasePrevOutN uint64 = 4294967295

type txHead struct {
	Hash    string `json:"hash"`
	TxIndex uint64 `json:"tx_index"`
}

type prevOutWire struct {
	Hash    string `json:"hash"`
	Value   int64  `json:"value"`
	TxIndex uint64 `json:"tx_index"` // global index of the tx that created this output
	N       uint64 `json:"n"`        // output index within that tx (coinbase uses coinbasePrevOutN)
}

type txWire struct {
	Hash    string `json:"hash"`
	TxIndex uint64 `json:"tx_index"`
	Inputs  []struct {
		PrevOut *prevOutWire `json:"prev_out"`
	} `json:"inputs"`
}

type edgeKey struct {
	From string
	To   string
}

// buildSubgraph parses block txs into spend edges (funding tx → spending tx).
// Multiple inputs that map to the same (From, To) pair add their prev_out values into Amount.
//
// blockchain.info rawblock txs usually omit prev_out.hash; the spent funding tx is identified by
// prev_out.tx_index (global). We map tx_index → txid for txs in this block and use decimal
// tx_index strings for parents not present in the block (cross-block spends).
func buildSubgraph(fb *ingestor.FetchedBlock) (*Subgraph, error) {
	txIndexToHash := make(map[uint64]string, len(fb.Block.Tx))
	for _, raw := range fb.Block.Tx {
		var head txHead
		if err := json.Unmarshal(raw, &head); err != nil {
			return nil, fmt.Errorf("decode tx head: %w", err)
		}
		if head.Hash != "" && head.TxIndex != 0 {
			txIndexToHash[head.TxIndex] = head.Hash
		}
	}

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
			po := in.PrevOut
			if po.N == coinbasePrevOutN {
				continue
			}
			from := po.Hash
			if from == "" || from == zeroTxid {
				from = txIndexToHash[po.TxIndex]
			}
			if from == "" {
				if po.TxIndex == 0 {
					continue
				}
				from = strconv.FormatUint(po.TxIndex, 10)
			}
			if from == zeroTxid {
				continue
			}
			k := edgeKey{From: from, To: tw.Hash}
			agg[k] += po.Value
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
	fmt.Printf("Subgraph built for id %d with %d edges\n", id, len(sg.Edges))
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
