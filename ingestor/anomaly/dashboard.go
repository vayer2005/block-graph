package anomaly

import (
	"sort"

	"graph-fraud/ingestor/features"
)

// DashboardExport is JSON-friendly for a React dashboard: top-1 per metric with small subgraphs.
type DashboardExport struct {
	Meta DashboardMeta `json:"meta"`

	TopFanIn         *MetricHighlight `json:"top_fan_in,omitempty"`
	TopFanOut        *MetricHighlight `json:"top_fan_out,omitempty"`
	TopGatherScatter *MetricHighlight `json:"top_gather_scatter,omitempty"`
	LongestHopPath   *PathHighlight   `json:"longest_hop_path,omitempty"`
}

// DashboardMeta summarizes the merged graph used for baselines.
type DashboardMeta struct {
	NodeCount            int  `json:"node_count"`
	EdgeCount            int  `json:"edge_count"`
	IsDAG                bool `json:"is_dag"`
	LongestPathEdgeCount int  `json:"longest_path_edge_count_global"`
}

// MetricHighlight is one winner (top 1) for fan-in, fan-out, or gather-scatter (hub).
type MetricHighlight struct {
	Kind         string        `json:"kind"` // fan_in | fan_out | gather_scatter
	MetricValue  int64         `json:"metric_value"`
	CenterTxid   string        `json:"center_txid"`
	MetricDetail string        `json:"metric_detail,omitempty"` 
	Subgraph     *SubgraphJSON `json:"subgraph"`
}

// PathHighlight is the globally longest directed path (in edge count) with amounts on each hop.
type PathHighlight struct {
	Kind      string        `json:"kind"` // longest_hop_path
	EdgeCount int           `json:"edge_count"`
	Subgraph  *SubgraphJSON `json:"subgraph"`
}

// SubgraphJSON is a small induced edge list plus node metadata for visualization.
// For fan_in / fan_out / gather_scatter, edges may be truncated to MaxStarEdges (see DashboardConfig);
// TotalEdgesAvailable and EdgesTruncated document that. longest_hop_path is never truncated.
type SubgraphJSON struct {
	Nodes               []NodeView `json:"nodes"`
	Edges               []EdgeView `json:"edges"`
	EdgesTruncated      bool       `json:"edges_truncated,omitempty"`
	TotalEdgesAvailable int        `json:"total_edges_available,omitempty"`
}

// DashboardConfig controls how much of each star/hub highlight is serialized (not the longest path).
type DashboardConfig struct {
	// MaxStarEdges is the max number of edges to include for top_fan_in, top_fan_out, and top_gather_scatter.
	// 0 uses DefaultMaxStarEdges. Negative means no limit (all incident edges; large JSON).
	MaxStarEdges int
}

// DefaultMaxStarEdges limits star/hub subgraph size in the dashboard JSON.
const DefaultMaxStarEdges = 64

// NodeView is one txid node with optional path order along a chain.
type NodeView struct {
	TxID   string `json:"txid"`
	FanIn  int    `json:"fan_in"`
	FanOut int    `json:"fan_out"`
	InSats int64  `json:"in_sats"`
	OutSats int64 `json:"out_sats"`
	// PathOrder is set along longest_hop_path (0-based index along the path); nil for star/hub subgraphs.
	PathOrder *int `json:"path_order,omitempty"`
}

// EdgeView is one directed spend edge with aggregated satoshis.
type EdgeView struct {
	From      string `json:"from"`
	To        string `json:"to"`
	AmountSat int64  `json:"amount_sat"`
}

// spendAnalysis is one pass over the merged subgraph: adjacency, DAG check, longest paths, per-node metrics.
type spendAnalysis struct {
	sg        *SpendGraph
	topo      TopoResult
	longest   map[string]int
	globalMax int
	rows      []NodeMetrics
}

func analyzeSpendGraph(g *features.Subgraph) *spendAnalysis {
	sg := BuildSpendGraph(g)
	topo := TopologicalSort(sg)
	var longest map[string]int
	var globalMax int
	if topo.IsDAG {
		var ok bool
		longest, globalMax, ok = LongestPathDepth(sg, topo)
		if !ok {
			longest = nil
		}
	}
	rows := Analyze(sg, longest)
	return &spendAnalysis{sg: sg, topo: topo, longest: longest, globalMax: globalMax, rows: rows}
}

func buildDashboardExportFrom(a *spendAnalysis, cfg DashboardConfig) *DashboardExport {
	maxStar, unlimited := normalizeMaxStarEdges(cfg.MaxStarEdges)

	out := &DashboardExport{
		Meta: DashboardMeta{},
	}
	out.Meta.NodeCount = len(a.sg.Nodes)
	out.Meta.EdgeCount = a.sg.edgeCount()
	out.Meta.IsDAG = a.topo.IsDAG
	out.Meta.LongestPathEdgeCount = a.globalMax

	byID := metricsByTxID(a.rows)

	out.TopFanIn = buildTopFanIn(a.sg, byID, maxStar, unlimited)
	out.TopFanOut = buildTopFanOut(a.sg, byID, maxStar, unlimited)
	out.TopGatherScatter = buildTopGatherScatter(a.sg, byID, maxStar, unlimited)
	if a.topo.IsDAG && a.longest != nil {
		out.LongestHopPath = buildLongestHopPath(a.sg, byID, a.longest, a.globalMax)
	}

	return out
}

// BuildDashboardExport computes top-1 highlights and subgraphs from a merged subgraph.
func BuildDashboardExport(g *features.Subgraph, cfg DashboardConfig) *DashboardExport {
	return buildDashboardExportFrom(analyzeSpendGraph(g), cfg)
}

func normalizeMaxStarEdges(n int) (max int, unlimited bool) {
	if n < 0 {
		return 0, true
	}
	if n == 0 {
		return DefaultMaxStarEdges, false
	}
	return n, false
}

func sortEdgesByAmountDesc(edges []EdgeView) []EdgeView {
	out := append([]EdgeView(nil), edges...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].AmountSat != out[j].AmountSat {
			return out[i].AmountSat > out[j].AmountSat
		}
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

func limitStarEdges(edges []EdgeView, max int, unlimited bool) ([]EdgeView, int, bool) {
	total := len(edges)
	if unlimited || total <= max {
		return edges, total, false
	}
	s := sortEdgesByAmountDesc(edges)
	return s[:max], total, true
}

func limitHubEdges(inE, outE []EdgeView, max int, unlimited bool) ([]EdgeView, int, bool) {
	total := len(inE) + len(outE)
	if total == 0 {
		return nil, 0, false
	}
	if unlimited || total <= max {
		edges := append(append([]EdgeView{}, inE...), outE...)
		sortEdgesLex(edges)
		return edges, total, false
	}
	merged := append(append([]EdgeView{}, inE...), outE...)
	merged = sortEdgesByAmountDesc(merged)
	top := merged[:max]
	sortEdgesLex(top)
	return top, total, true
}

func sortEdgesLex(edges []EdgeView) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
}

func metricsByTxID(rows []NodeMetrics) map[string]NodeMetrics {
	m := make(map[string]NodeMetrics, len(rows))
	for _, r := range rows {
		m[r.TxID] = r
	}
	return m
}

func nodeView(m NodeMetrics, pathOrder *int) NodeView {
	return NodeView{
		TxID:      m.TxID,
		FanIn:     m.FanIn,
		FanOut:    m.FanOut,
		InSats:    m.InSats,
		OutSats:   m.OutSats,
		PathOrder: pathOrder,
	}
}

func buildTopFanIn(sg *SpendGraph, byID map[string]NodeMetrics, maxStar int, unlimited bool) *MetricHighlight {
	w := pickTop1(rowsSlice(byID), func(a, b NodeMetrics) bool {
		if a.FanIn != b.FanIn {
			return a.FanIn > b.FanIn
		}
		return a.TxID < b.TxID
	})
	if w == nil || w.FanIn == 0 {
		return nil
	}
	center := w.TxID
	var full []EdgeView
	for _, p := range sg.In[center] {
		full = append(full, EdgeView{From: p, To: center, AmountSat: sg.EdgeAmount(p, center)})
	}
	edges, total, trunc := limitStarEdges(full, maxStar, unlimited)
	sortEdgesLex(edges)
	nodes := collectNodesForEdges(sg, byID, edges, center)
	return &MetricHighlight{
		Kind:         "fan_in",
		MetricValue:  int64(w.FanIn),
		CenterTxid:   center,
		MetricDetail: "fan_in_degree",
		Subgraph: &SubgraphJSON{
			Nodes:               nodes,
			Edges:               edges,
			EdgesTruncated:      trunc,
			TotalEdgesAvailable: total,
		},
	}
}

func buildTopFanOut(sg *SpendGraph, byID map[string]NodeMetrics, maxStar int, unlimited bool) *MetricHighlight {
	w := pickTop1(rowsSlice(byID), func(a, b NodeMetrics) bool {
		if a.FanOut != b.FanOut {
			return a.FanOut > b.FanOut
		}
		return a.TxID < b.TxID
	})
	if w == nil || w.FanOut == 0 {
		return nil
	}
	center := w.TxID
	var full []EdgeView
	for _, c := range sg.Out[center] {
		full = append(full, EdgeView{From: center, To: c, AmountSat: sg.EdgeAmount(center, c)})
	}
	edges, total, trunc := limitStarEdges(full, maxStar, unlimited)
	sortEdgesLex(edges)
	nodes := collectNodesForEdges(sg, byID, edges, center)
	return &MetricHighlight{
		Kind:         "fan_out",
		MetricValue:  int64(w.FanOut),
		CenterTxid:   center,
		MetricDetail: "fan_out_degree",
		Subgraph: &SubgraphJSON{
			Nodes:               nodes,
			Edges:               edges,
			EdgesTruncated:      trunc,
			TotalEdgesAvailable: total,
		},
	}
}

func buildTopGatherScatter(sg *SpendGraph, byID map[string]NodeMetrics, maxStar int, unlimited bool) *MetricHighlight {
	var candidates []NodeMetrics
	for _, m := range byID {
		if m.FanIn > 1 && m.FanOut > 1 {
			candidates = append(candidates, m)
		}
	}
	w := pickTop1(candidates, func(a, b NodeMetrics) bool {
		if a.HubScore != b.HubScore {
			return a.HubScore > b.HubScore
		}
		return a.TxID < b.TxID
	})
	if w == nil {
		w = pickTop1(rowsSlice(byID), func(a, b NodeMetrics) bool {
			if a.HubScore != b.HubScore {
				return a.HubScore > b.HubScore
			}
			return a.TxID < b.TxID
		})
	}
	if w == nil || w.HubScore == 0 {
		return nil
	}
	center := w.TxID
	var inE, outE []EdgeView
	for _, p := range sg.In[center] {
		inE = append(inE, EdgeView{From: p, To: center, AmountSat: sg.EdgeAmount(p, center)})
	}
	for _, c := range sg.Out[center] {
		outE = append(outE, EdgeView{From: center, To: c, AmountSat: sg.EdgeAmount(center, c)})
	}
	edges, total, trunc := limitHubEdges(inE, outE, maxStar, unlimited)
	nodes := collectNodesForEdges(sg, byID, edges, center)
	return &MetricHighlight{
		Kind:         "gather_scatter",
		MetricValue:  w.HubScore,
		CenterTxid:   center,
		MetricDetail: "hub_score_fan_in_times_fan_out",
		Subgraph: &SubgraphJSON{
			Nodes:               nodes,
			Edges:               edges,
			EdgesTruncated:      trunc,
			TotalEdgesAvailable: total,
		},
	}
}

func buildLongestHopPath(sg *SpendGraph, byID map[string]NodeMetrics, longest map[string]int, globalMax int) *PathHighlight {
	path, ok := traceLongestPath(sg, longest, globalMax)
	if !ok || len(path) == 0 {
		return nil
	}
	edgeCount := len(path) - 1
	if edgeCount < 0 {
		edgeCount = 0
	}
	var edges []EdgeView
	for i := 0; i+1 < len(path); i++ {
		u, v := path[i], path[i+1]
		edges = append(edges, EdgeView{From: u, To: v, AmountSat: sg.EdgeAmount(u, v)})
	}
	nodes := make([]NodeView, 0, len(path))
	for i, txid := range path {
		m := byID[txid]
		ord := i
		nodes = append(nodes, nodeView(m, &ord))
	}
	return &PathHighlight{
		Kind:      "longest_hop_path",
		EdgeCount: edgeCount,
		Subgraph: &SubgraphJSON{
			Nodes:               nodes,
			Edges:               edges,
			EdgesTruncated:      false,
			TotalEdgesAvailable: len(edges),
		},
	}
}

// traceLongestPath picks the lexicographically smallest endpoint among nodes with longest==globalMax, then backtracks.
func traceLongestPath(sg *SpendGraph, longest map[string]int, globalMax int) ([]string, bool) {
	if sg == nil || longest == nil {
		return nil, false
	}
	var end string
	found := false
	for _, v := range sg.Nodes {
		if longest[v] != globalMax {
			continue
		}
		if !found || v < end {
			end = v
			found = true
		}
	}
	if !found {
		return nil, false
	}
	rev := []string{end}
	cur := end
	for longest[cur] > 0 {
		parents := sg.In[cur]
		var best string
		has := false
		for _, u := range parents {
			if longest[u]+1 != longest[cur] {
				continue
			}
			if !has || u < best {
				best = u
				has = true
			}
		}
		if !has {
			return nil, false
		}
		rev = append(rev, best)
		cur = best
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, true
}

func collectNodesForEdges(sg *SpendGraph, byID map[string]NodeMetrics, edges []EdgeView, center string) []NodeView {
	seen := make(map[string]struct{})
	var ids []string
	add := func(id string) {
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	add(center)
	for _, e := range edges {
		add(e.From)
		add(e.To)
	}
	sort.Strings(ids)
	nodes := make([]NodeView, 0, len(ids))
	for _, id := range ids {
		nodes = append(nodes, nodeView(byID[id], nil))
	}
	return nodes
}

func rowsSlice(byID map[string]NodeMetrics) []NodeMetrics {
	s := make([]NodeMetrics, 0, len(byID))
	for _, m := range byID {
		s = append(s, m)
	}
	return s
}

func pickTop1(rows []NodeMetrics, better func(a, b NodeMetrics) bool) *NodeMetrics {
	if len(rows) == 0 {
		return nil
	}
	best := rows[0]
	for i := 1; i < len(rows); i++ {
		if better(rows[i], best) {
			best = rows[i]
		}
	}
	return &best
}
