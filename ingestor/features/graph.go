package features

import (
	"context"
	"fmt"

	"graph-fraud/ingestor"
)

// Working plan:
// Map from graph id to graph object
// Each go routine worker takes two graph ids from the map and merges them into a new graph object with a new id
// New id is pushed to channel
// Worker pool is used to process the channel
// Once channel is empty and var wg sync.WaitGroup from ingestor is closed, return the final graph object


// ProcessBlock builds subgraph / graph features for a single fetched block.
// Called from ingestor.RunWithHandler as soon as each block finishes downloading.
func ProcessBlock(ctx context.Context, fb *ingestor.FetchedBlock) error {
	_ = ctx
	txs := fb.Block.Tx
	fmt.Printf("[graph] block_hash=%s height=%d num_tx=%d\n", fb.Block.Hash, fb.Height, len(txs))
	_ = txs // graph: iterate txs and decode for edges, etc.
	return nil
}

