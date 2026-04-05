package ingestor

import (
	"context"
	"fmt"
	"sync"
)

// Config controls how many recent blocks to pull and how much parallelism to use.
type Config struct {
	// BlockCount is how many confirmed blocks ending at the chain tip to ingest (must be >= 1).
	BlockCount int
	// Workers is the number of concurrent block fetches (goroutines).
	Workers int
}

// PlanQueue computes the ordered list of block heights to query: from
// (tip - BlockCount + 1) through tip inclusive. This is the full work queue before
// any network calls.
func PlanQueue(tip int, blockCount int) ([]int, error) {
	if blockCount < 1 {
		return nil, fmt.Errorf("blockCount must be >= 1, got %d", blockCount)
	}
	start := tip - blockCount + 1
	if start < 0 {
		return nil, fmt.Errorf("window start height %d would be negative for tip %d and count %d", start, tip, blockCount)
	}
	out := make([]int, 0, blockCount)
	for h := start; h <= tip; h++ {
		out = append(out, h)
	}
	return out, nil
}

// DiscoverQueue is step one: resolve the chain tip and build the full ordered queue
// of block heights that still need HTTP fetches for this run (the window
// [tip-blockCount+1, tip]).
func DiscoverQueue(ctx context.Context, c *Client, blockCount int) (tip int, queue []int, err error) {
	tip, err = c.TipHeight(ctx)
	if err != nil {
		return 0, nil, err
	}
	queue, err = PlanQueue(tip, blockCount)
	if err != nil {
		return 0, nil, err
	}
	return tip, queue, nil
}

// IngestResult holds all blocks fetched in one run, keyed by height for convenience.
type IngestResult struct {
	ByHeight map[int]FetchedBlock
	Errors   []IngestError
}

// IngestError records a height that failed after retries are exhausted (see worker).
type IngestError struct {
	Height int
	Err    error
}

// Run builds the block height queue from the current tip and Config, then runs a
// worker pool: each worker pops the next height, resolves hash, and loads the full
// block (including all transactions in JSON form). Transaction parsing / graph
// edges are not implemented here — they live in each Block.Tx slice on the result.
func Run(ctx context.Context, c *Client, cfg Config) (*IngestResult, error) {
	if cfg.Workers < 1 {
		return nil, fmt.Errorf("workers must be >= 1, got %d", cfg.Workers)
	}
	_, queue, err := DiscoverQueue(ctx, c, cfg.BlockCount)
	if err != nil {
		return nil, err
	}

	jobs := make(chan int, len(queue))
	for _, h := range queue {
		jobs <- h
	}
	close(jobs)

	var mu sync.Mutex
	byHeight := make(map[int]FetchedBlock, len(queue))
	var errs []IngestError

	var wg sync.WaitGroup
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for height := range jobs {
				hash, err := c.BlockHashAtHeight(ctx, height)
				if err != nil {
					mu.Lock()
					errs = append(errs, IngestError{Height: height, Err: err})
					mu.Unlock()
					continue
				}
				blk, err := c.BlockByHash(ctx, hash)
				if err != nil {
					mu.Lock()
					errs = append(errs, IngestError{Height: height, Err: err})
					mu.Unlock()
					continue
				}
				mu.Lock()
				byHeight[height] = FetchedBlock{Height: height, Block: blk}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	return &IngestResult{ByHeight: byHeight, Errors: errs}, nil
}
