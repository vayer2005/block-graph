package features

import (
	"container/heap"
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

// mergeHeapEntry is one queued subgraph: id plus edge count for heap ordering.
// The heap is a min-heap on edges (then id), so merges tend to combine smaller graphs first.
type mergeHeapEntry struct {
	id    int
	edges int
}

type mergeEntryHeap []mergeHeapEntry

func (h mergeEntryHeap) Len() int { return len(h) }
func (h mergeEntryHeap) Less(i, j int) bool {
	if h[i].edges != h[j].edges {
		return h[i].edges < h[j].edges
	}
	return h[i].id < h[j].id
}
func (h mergeEntryHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *mergeEntryHeap) Push(x any) {
	*h = append(*h, x.(mergeHeapEntry))
}

func (h *mergeEntryHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

var (
	mergeCoordinatorOnce sync.Once
	mergeDone            chan struct{}
	mergeCompleteOnce    sync.Once
	ingestFinished       atomic.Bool

	mergeHeap     *mergeEntryHeap
	mergeHeapMu   sync.Mutex
	mergeHeapCond *sync.Cond
)

func initMergeHeap() {
	h := &mergeEntryHeap{}
	mergeHeap = h
	mergeHeapCond = sync.NewCond(&mergeHeapMu)
}

func pushSubgraphMergeHeap(id, edgeCount int) {
	mergeHeapMu.Lock()
	heap.Push(mergeHeap, mergeHeapEntry{id: id, edges: edgeCount})
	mergeHeapCond.Broadcast()
	mergeHeapMu.Unlock()
}

// StartMergeCoordinator starts worker goroutines that pop the two entries with smallest edge counts
// from the merge heap, merge them, and push the new id (with its edge count) back until one graph remains.
// Call NotifyIngestFinished after ingest finishes, then WaitMerged.
// workers must be >= 1.
func StartMergeCoordinator(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	initPipeline()
	mergeCoordinatorOnce.Do(func() {
		mergeDone = make(chan struct{})

		go func() {
			<-ctx.Done()
			mergeHeapMu.Lock()
			mergeHeapCond.Broadcast()
			mergeHeapMu.Unlock()
		}()

		for i := 0; i < workers; i++ {
			go mergeWorkerLoop(ctx)
		}
	})
}

// NotifyIngestFinished marks ingest as complete (all ProcessBlock calls returned).
// Merge workers use this to stop waiting for more ids and to finish when only one subgraph remains.
func NotifyIngestFinished() {
	initPipeline()
	ingestFinished.Store(true)
	mergeHeapMu.Lock()
	mergeHeapCond.Broadcast()
	mergeHeapMu.Unlock()
}

// WaitMerged blocks until the merge workers have reduced to a single graph or ctx is done.
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

// trySignalMergeCompleteLocked reports whether merging is finished (ingest done and 0 or 1 graphs left).
// Caller must hold mergeHeapMu. On true, mergeDone is closed once.
func trySignalMergeCompleteLocked() bool {
	if !ingestFinished.Load() {
		return false
	}
	n := numGraphs.Load()
	h := mergeHeap.Len()
	if n == 0 && h == 0 {
		signalMergeComplete()
		return true
	}
	if n == 1 && h == 1 {
		signalMergeComplete()
		return true
	}
	return false
}

func mergeWorkerLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		mergeHeapMu.Lock()
		if trySignalMergeCompleteLocked() {
			mergeHeapMu.Unlock()
			mergeHeapCond.Broadcast()
			return
		}

		for mergeHeap.Len() < 2 && !ingestFinished.Load() {
			mergeHeapCond.Wait()
			if ctx.Err() != nil {
				mergeHeapMu.Unlock()
				return
			}
		}

		if mergeHeap.Len() < 2 {
			if trySignalMergeCompleteLocked() {
				mergeHeapMu.Unlock()
				mergeHeapCond.Broadcast()
				return
			}
			mergeHeapMu.Unlock()
			continue
		}

		ea := heap.Pop(mergeHeap).(mergeHeapEntry)
		eb := heap.Pop(mergeHeap).(mergeHeapEntry)
		mergeHeapMu.Unlock()

		newID, mergedEdges, err := mergePairIDs(ctx, ea.id, eb.id)

		mergeHeapMu.Lock()
		if err != nil {
			sa, sb, ok := edgeCountsForIDs(ea.id, eb.id)
			if ok {
				heap.Push(mergeHeap, mergeHeapEntry{id: ea.id, edges: sa})
				heap.Push(mergeHeap, mergeHeapEntry{id: eb.id, edges: sb})
			}
			mergeHeapCond.Broadcast()
			mergeHeapMu.Unlock()
			continue
		}
		heap.Push(mergeHeap, mergeHeapEntry{id: newID, edges: mergedEdges})
		mergeHeapCond.Broadcast()
		if trySignalMergeCompleteLocked() {
			mergeHeapMu.Unlock()
			mergeHeapCond.Broadcast()
			return
		}
		mergeHeapMu.Unlock()
	}
}

func edgeCountsForIDs(a, b int) (ea, eb int, ok bool) {
	SubgraphRegistryMu.Lock()
	defer SubgraphRegistryMu.Unlock()
	ga, okA := Subgraphs[a]
	gb, okB := Subgraphs[b]
	if !okA || !okB || ga == nil || gb == nil {
		return 0, 0, false
	}
	return len(ga.Edges), len(gb.Edges), true
}

func mergePairIDs(ctx context.Context, a, b int) (newID int, mergedEdges int, err error) {
	_ = ctx
	SubgraphRegistryMu.Lock()
	ga, okA := Subgraphs[a]
	gb, okB := Subgraphs[b]
	SubgraphRegistryMu.Unlock()
	if !okA || !okB {
		return 0, 0, fmt.Errorf("merge: missing subgraph id a=%d b=%d", a, b)
	}

	merged := MergeSubgraphs(ga, gb)
	newID = AllocGraphID()
	mergedEdges = len(merged.Edges)

	SubgraphRegistryMu.Lock()
	delete(Subgraphs, a)
	delete(Subgraphs, b)
	Subgraphs[newID] = merged
	SubgraphRegistryMu.Unlock()

	numGraphs.Add(-1)

	fmt.Printf("Merged subgraphs %d and %d into %d with %d edges\n", a, b, newID, mergedEdges)

	return newID, mergedEdges, nil
}
