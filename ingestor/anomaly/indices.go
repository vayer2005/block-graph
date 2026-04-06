package anomaly

import (
	"sort"

	"graph-fraud/ingestor/features"
)

// SpendGraph is an adjacency view of a merged spend DAG (txid nodes, directed edges).
type SpendGraph struct {
	Nodes   []string
	Out     map[string][]string
	In      map[string][]string
	OutSats map[string]int64
	InSats  map[string]int64
	// EdgeAmt holds aggregated satoshis for each directed edge (from -> to).
	EdgeAmt map[string]map[string]int64
}

// BuildSpendGraph builds unique-neighbor adjacency and per-node sat totals from a subgraph.
func BuildSpendGraph(g *features.Subgraph) *SpendGraph {
	if g == nil {
		return &SpendGraph{
			Out:     map[string][]string{},
			In:      map[string][]string{},
			OutSats: map[string]int64{},
			InSats:  map[string]int64{},
			EdgeAmt: map[string]map[string]int64{},
		}
	}

	outSet := make(map[string]map[string]struct{})
	inSet := make(map[string]map[string]struct{})
	nodeSet := make(map[string]struct{})
	outSats := make(map[string]int64)
	inSats := make(map[string]int64)
	edgeAmt := make(map[string]map[string]int64)

	addOut := func(from, to string) {
		if from == to {
			return
		}
		if outSet[from] == nil {
			outSet[from] = make(map[string]struct{})
		}
		outSet[from][to] = struct{}{}
	}
	addIn := func(from, to string) {
		if from == to {
			return
		}
		if inSet[to] == nil {
			inSet[to] = make(map[string]struct{})
		}
		inSet[to][from] = struct{}{}
	}

	for _, e := range g.Edges {
		if e.From == "" || e.To == "" {
			continue
		}
		nodeSet[e.From] = struct{}{}
		nodeSet[e.To] = struct{}{}
		addOut(e.From, e.To)
		addIn(e.From, e.To)
		outSats[e.From] += e.Amount
		inSats[e.To] += e.Amount
		if edgeAmt[e.From] == nil {
			edgeAmt[e.From] = make(map[string]int64)
		}
		edgeAmt[e.From][e.To] += e.Amount
	}

	nodes := make([]string, 0, len(nodeSet))
	for id := range nodeSet {
		nodes = append(nodes, id)
	}
	sort.Strings(nodes)

	toSlice := func(m map[string]map[string]struct{}) map[string][]string {
		out := make(map[string][]string, len(m))
		for u, neigh := range m {
			s := make([]string, 0, len(neigh))
			for v := range neigh {
				s = append(s, v)
			}
			sort.Strings(s)
			out[u] = s
		}
		return out
	}

	return &SpendGraph{
		Nodes:   nodes,
		Out:     toSlice(outSet),
		In:      toSlice(inSet),
		OutSats: outSats,
		InSats:  inSats,
		EdgeAmt: edgeAmt,
	}
}

// EdgeAmount returns aggregated satoshis on edge from->to, or 0 if absent.
func (sg *SpendGraph) EdgeAmount(from, to string) int64 {
	if sg == nil || sg.EdgeAmt == nil {
		return 0
	}
	if m := sg.EdgeAmt[from]; m != nil {
		return m[to]
	}
	return 0
}

func (sg *SpendGraph) edgeCount() int {
	n := 0
	for _, tos := range sg.Out {
		n += len(tos)
	}
	return n
}
