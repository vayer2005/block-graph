package features

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// MergeSubgraphs returns the union of two subgraphs: edges with the same (From, To) have Amount summed.
func MergeSubgraphs(a, b *Subgraph) *Subgraph {
	if a == nil && b == nil {
		return &Subgraph{}
	}
	if a == nil {
		return cloneSubgraph(b)
	}
	if b == nil {
		return cloneSubgraph(a)
	}

	agg := make(map[edgeKey]int64)
	for _, e := range a.Edges {
		agg[edgeKey{From: e.From, To: e.To}] += e.Amount
	}
	for _, e := range b.Edges {
		agg[edgeKey{From: e.From, To: e.To}] += e.Amount
	}
	out := make([]TxEdge, 0, len(agg))
	for k, v := range agg {
		out = append(out, TxEdge{From: k.From, To: k.To, Amount: v})
	}
	return &Subgraph{Edges: out}
}

func cloneSubgraph(s *Subgraph) *Subgraph {
	if s == nil {
		return &Subgraph{}
	}
	edges := make([]TxEdge, len(s.Edges))
	copy(edges, s.Edges)
	return &Subgraph{Edges: edges}
}

var (
	mergeCoordinatorOnce sync.Once
	mergeDone            chan struct{}
	mergeCompleteOnce    sync.Once
	ingestFinished       atomic.Bool
	wakeCoordinator      chan struct{} // buffered: unblocks coordinator after Notify or merge
)

// StartMergeCoordinator starts a coordinator and worker goroutines that merge subgraphs
// until a single graph remains. Call NotifyIngestFinished after ingest finishes, then WaitMerged.
// workers must be >= 1.
func StartMergeCoordinator(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	initPipeline()
	mergeCoordinatorOnce.Do(func() {
		mergeDone = make(chan struct{})
		wakeCoordinator = make(chan struct{}, 1)

		pairCh := make(chan [2]int, workers*2)
		mergeResults := make(chan int, workers*2)

		for i := 0; i < workers; i++ {
			go mergeWorker(ctx, pairCh, mergeResults)
		}
		go coordinatorLoop(ctx, pairCh, mergeResults)
	})
}

// NotifyIngestFinished marks ingest as complete (all ProcessBlock calls returned).
// The merge coordinator uses this to finish when only one subgraph remains.
func NotifyIngestFinished() {
	initPipeline()
	ingestFinished.Store(true)
	select {
	case wakeCoordinator <- struct{}{}:
	default:
	}
}

// WaitMerged blocks until the merge coordinator has reduced to a single graph or ctx is done.
func WaitMerged(ctx context.Context) error {
	initPipeline()
	if mergeDone == nil {
		return fmt.Errorf("merge: StartMergeCoordinator was not called")
	}
	select {
	case <-mergeDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// FinalMergedGraph returns the id and subgraph when exactly one graph remains in the registry.
func FinalMergedGraph() (id int, g *Subgraph, ok bool) {
	initPipeline()
	SubgraphRegistryMu.Lock()
	defer SubgraphRegistryMu.Unlock()
	if len(Subgraphs) != 1 {
		return 0, nil, false
	}
	for i, sg := range Subgraphs {
		return i, sg, true
	}
	return 0, nil, false
}

func signalMergeComplete() {
	mergeCompleteOnce.Do(func() {
		if mergeDone != nil {
			close(mergeDone)
		}
	})
}

func coordinatorLoop(ctx context.Context, pairCh chan [2]int, mergeResults chan int) {
	var q []int
	var mu sync.Mutex

	trySchedule := func() {
		for {
			mu.Lock()
			if len(q) < 2 {
				mu.Unlock()
				return
			}
			a, b := q[0], q[1]
			q = q[2:]
			mu.Unlock()
			select {
			case pairCh <- [2]int{a, b}:
			case <-ctx.Done():
				return
			}
		}
	}

	tryFinalize := func() bool {
		mu.Lock()
		ok := ingestFinished.Load() && numGraphs.Load() == 1 && len(q) == 1
		mu.Unlock()
		if ok {
			signalMergeComplete()
			return true
		}
		return false
	}

	for {
		mu.Lock()
		n := numGraphs.Load()
		lq := len(q)
		mu.Unlock()
		if ingestFinished.Load() && n == 0 && lq == 0 {
			// No subgraphs produced (e.g. all block fetches failed).
			signalMergeComplete()
			return
		}
		if ingestFinished.Load() && n == 1 && lq == 1 {
			signalMergeComplete()
			return
		}

		select {
		case id := <-subgraphIDs:
			mu.Lock()
			q = append(q, id)
			mu.Unlock()
			trySchedule()
			if tryFinalize() {
				return
			}

		case newID := <-mergeResults:
			mu.Lock()
			q = append(q, newID)
			mu.Unlock()
			trySchedule()
			if tryFinalize() {
				return
			}

		case <-wakeCoordinator:
			trySchedule()
			if tryFinalize() {
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func mergeWorker(ctx context.Context, pairCh <-chan [2]int, mergeResults chan<- int) {
	for {
		select {
		case pair, ok := <-pairCh:
			if !ok {
				return
			}
			newID, err := mergePairIDs(ctx, pair[0], pair[1])
			if err != nil {
				continue
			}
			select {
			case mergeResults <- newID:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func mergePairIDs(ctx context.Context, a, b int) (int, error) {
	_ = ctx
	SubgraphRegistryMu.Lock()
	ga, okA := Subgraphs[a]
	gb, okB := Subgraphs[b]
	SubgraphRegistryMu.Unlock()
	if !okA || !okB {
		return 0, fmt.Errorf("merge: missing subgraph id a=%d b=%d", a, b)
	}

	merged := MergeSubgraphs(ga, gb)
	newID := AllocGraphID()

	SubgraphRegistryMu.Lock()
	delete(Subgraphs, a)
	delete(Subgraphs, b)
	Subgraphs[newID] = merged
	SubgraphRegistryMu.Unlock()

	numGraphs.Add(-1)

	fmt.Printf("Merged subgraphs %d and %d into %d with %d edges\n", a, b, newID, len(merged.Edges))
	select {
	case wakeCoordinator <- struct{}{}:
	default:
	}

	return newID, nil
}
