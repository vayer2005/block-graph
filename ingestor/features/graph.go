package features

import (
	"context"
	"fmt"

	"graph-fraud/ingestor"
)

// SubscribeBlocks receives block heights on ch until ch closes or ctx ends.
// result must be the ingest output: the subscriber looks up each height in
// result.ByHeight and reads the full Block (including Block.Tx) from that map
// entry—only small ints cross the channel, not transaction payloads.
func SubscribeBlocks(ctx context.Context, ch <-chan int, result *ingestor.IngestResult) {
	for {
		select {
		case <-ctx.Done():
			return
		case h, ok := <-ch:
			if !ok {
				return
			}
			fb, ok := result.ByHeight[h]
			if !ok {
				continue
			}
			txs := fb.Block.Tx // []json.RawMessage — full tx JSON; lives in result, not sent on ch
			fmt.Printf("[graph] block_hash=%s height=%d num_tx=%d\n", fb.Block.Hash, h, len(txs))
			_ = txs // graph: iterate txs and decode for edges, etc.
		}
	}
}
