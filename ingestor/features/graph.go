package features

import (
	"context"
	"fmt"

	"graph-fraud/ingestor"
)

// ProcessBlock builds subgraph / graph features for a single fetched block.
// Called from ingestor.RunWithHandler as soon as each block finishes downloading.
func ProcessBlock(ctx context.Context, fb *ingestor.FetchedBlock) error {
	_ = ctx
	txs := fb.Block.Tx
	fmt.Printf("[graph] block_hash=%s height=%d num_tx=%d\n", fb.Block.Hash, fb.Height, len(txs))
	_ = txs // graph: iterate txs and decode for edges, etc.
	return nil
}

