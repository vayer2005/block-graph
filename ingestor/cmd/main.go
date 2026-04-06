// Live CLI: runs the ingestor against blockchain.info (see ingestor.DefaultAPIBase).
// Run from repo root: go run ./ingestor/cmd/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"graph-fraud/ingestor"
	"graph-fraud/ingestor/features"
)

func main() {

	time_begin := time.Now()
	blocks := flag.Int("blocks", 20, "number of recent confirmed blocks to fetch (from chain tip)")
	workers := flag.Int("workers", 15, "parallel worker goroutines")
	timeout := flag.Duration("timeout", 5*time.Minute, "overall deadline for the run")
	flag.Parse()

	if *blocks < 1 {
		fmt.Fprintln(os.Stderr, "-blocks must be >= 1")
		os.Exit(1)
	}
	if *workers < 1 {
		fmt.Fprintln(os.Stderr, "-workers must be >= 1")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	c := ingestor.NewClient()

	// Merge coordinator must run before ingest so subgraph ids are consumed from the queue.
	features.StartMergeCoordinator(ctx, 15)

	res, err := ingestor.RunWithHandler(ctx, c, ingestor.Config{
		BlockCount: *blocks,
		Workers:    *workers,
	}, features.ProcessBlock)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	features.NotifyIngestFinished()
	if werr := features.WaitMerged(ctx); werr != nil {
		fmt.Fprintln(os.Stderr, "merge:", werr)
		os.Exit(1)
	}
	if id, g, ok := features.FinalMergedGraph(); ok {
		fmt.Printf("merged graph id=%d edges=%d\n", id, len(g.Edges))
	}

	fmt.Printf("fetched %d blocks", len(res.ByHeight))
	if len(res.Errors) > 0 {
		fmt.Printf(" (%d errors)\n", len(res.Errors))
		for _, e := range res.Errors {
			fmt.Fprintf(os.Stderr, "  height %d: %v\n", e.Height, e.Err)
		}
	} else {
		fmt.Println()
	}

	time_end := time.Now()
	fmt.Printf("Time taken: %s\n", time_end.Sub(time_begin))
}
