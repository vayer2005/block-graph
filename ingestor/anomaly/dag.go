package anomaly

import (
	"container/heap"
)

// TopoResult holds a topological order and whether the graph is a DAG over all nodes.
type TopoResult struct {
	Order   []string
	IsDAG   bool
	CycleOK bool // false if a directed cycle was detected
}

type minStringHeap []string

func (h minStringHeap) Len() int           { return len(h) }
func (h minStringHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h minStringHeap) Swap(i, j int)    { h[i], h[j] = h[j], h[i] }

func (h *minStringHeap) Push(x any) { *h = append(*h, x.(string)) }

func (h *minStringHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// TopologicalSort runs Kahn's algorithm with a min-heap so the order is deterministic.
// If len(Order) < |V|, the graph has a directed cycle.
func TopologicalSort(sg *SpendGraph) TopoResult {
	if sg == nil || len(sg.Nodes) == 0 {
		return TopoResult{Order: nil, IsDAG: true, CycleOK: true}
	}

	indeg := make(map[string]int, len(sg.Nodes))
	for _, u := range sg.Nodes {
		indeg[u] = 0
	}
	for _, tos := range sg.Out {
		for _, v := range tos {
			indeg[v]++
		}
	}

	var h minStringHeap
	for _, u := range sg.Nodes {
		if indeg[u] == 0 {
			h = append(h, u)
		}
	}
	heap.Init(&h)

	order := make([]string, 0, len(sg.Nodes))
	for h.Len() > 0 {
		u := heap.Pop(&h).(string)
		order = append(order, u)
		for _, v := range sg.Out[u] {
			indeg[v]--
			if indeg[v] == 0 {
				heap.Push(&h, v)
			}
		}
	}

	if len(order) != len(sg.Nodes) {
		return TopoResult{Order: order, IsDAG: false, CycleOK: false}
	}
	return TopoResult{Order: order, IsDAG: true, CycleOK: true}
}

// LongestPathDepth returns the longest directed path length (in edges) ending at each node.
// globalMax is the maximum over all nodes. Requires a DAG topological order.
func LongestPathDepth(sg *SpendGraph, topo TopoResult) (longest map[string]int, globalMax int, ok bool) {
	if !topo.IsDAG || sg == nil {
		return nil, 0, false
	}
	longest = make(map[string]int, len(sg.Nodes))
	for _, u := range sg.Nodes {
		longest[u] = 0
	}
	for _, u := range topo.Order {
		du := longest[u]
		for _, v := range sg.Out[u] {
			if du+1 > longest[v] {
				longest[v] = du + 1
			}
		}
	}
	globalMax = 0
	for _, d := range longest {
		if d > globalMax {
			globalMax = d
		}
	}
	return longest, globalMax, true
}
