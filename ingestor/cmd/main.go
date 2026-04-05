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
	blocks := flag.Int("blocks", 3, "number of recent confirmed blocks to fetch (from chain tip)")
	workers := flag.Int("workers", 4, "parallel worker goroutines")
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

	res, err := ingestor.RunWithHandler(ctx, c, ingestor.Config{
		BlockCount: *blocks,
		Workers:    *workers,
	}, features.ProcessBlock)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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
}
