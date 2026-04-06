package anomaly

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"graph-fraud/ingestor/features"
)

func subgraph(edges ...features.TxEdge) *features.Subgraph {
	return &features.Subgraph{Edges: append([]features.TxEdge(nil), edges...)}
}

func TestBuildSpendGraph_dedupNeighbors(t *testing.T) {
	g := subgraph(
		features.TxEdge{From: "a", To: "b", Amount: 1},
		features.TxEdge{From: "a", To: "b", Amount: 2},
	)
	sg := BuildSpendGraph(g)
	if len(sg.Out["a"]) != 1 || sg.Out["a"][0] != "b" {
		t.Fatalf("Out[a]=%v", sg.Out["a"])
	}
	if sg.OutSats["a"] != 3 {
		t.Fatalf("OutSats[a]=%d", sg.OutSats["a"])
	}
	if sg.EdgeAmount("a", "b") != 3 {
		t.Fatalf("EdgeAmount=%d", sg.EdgeAmount("a", "b"))
	}
}

func TestTopologicalSort_chain(t *testing.T) {
	g := subgraph(
		features.TxEdge{From: "a", To: "b", Amount: 1},
		features.TxEdge{From: "b", To: "c", Amount: 1},
		features.TxEdge{From: "c", To: "d", Amount: 1},
	)
	sg := BuildSpendGraph(g)
	topo := TopologicalSort(sg)
	if !topo.IsDAG {
		t.Fatal("expected DAG")
	}
	pos := make(map[string]int)
	for i, u := range topo.Order {
		pos[u] = i
	}
	if pos["a"] >= pos["b"] || pos["b"] >= pos["c"] || pos["c"] >= pos["d"] {
		t.Fatalf("order %v", topo.Order)
	}
	longest, global, ok := LongestPathDepth(sg, topo)
	if !ok || global != 3 {
		t.Fatalf("global=%d ok=%v longest=%v", global, ok, longest)
	}
	if longest["d"] != 3 {
		t.Fatalf("longest[d]=%d", longest["d"])
	}
	path, ok := traceLongestPath(sg, longest, global)
	if !ok || len(path) != 4 {
		t.Fatalf("path=%v ok=%v", path, ok)
	}
	if path[0] != "a" || path[3] != "d" {
		t.Fatalf("path=%v", path)
	}
}

func TestBuildDashboardExport_chain(t *testing.T) {
	g := subgraph(
		features.TxEdge{From: "a", To: "b", Amount: 100},
		features.TxEdge{From: "b", To: "c", Amount: 200},
	)
	d := BuildDashboardExport(g, DashboardConfig{})
	if d.LongestHopPath == nil || d.LongestHopPath.EdgeCount != 2 {
		t.Fatalf("longest %+v", d.LongestHopPath)
	}
	if len(d.LongestHopPath.Subgraph.Edges) != 2 {
		t.Fatalf("edges=%d", len(d.LongestHopPath.Subgraph.Edges))
	}
	if d.LongestHopPath.Subgraph.Edges[0].AmountSat != 100 || d.LongestHopPath.Subgraph.Edges[1].AmountSat != 200 {
		t.Fatalf("amounts %+v", d.LongestHopPath.Subgraph.Edges)
	}
}

func TestBuildDashboardExport_hourglass(t *testing.T) {
	g := subgraph(
		features.TxEdge{From: "p1", To: "h", Amount: 1},
		features.TxEdge{From: "p2", To: "h", Amount: 2},
		features.TxEdge{From: "h", To: "c1", Amount: 3},
		features.TxEdge{From: "h", To: "c2", Amount: 4},
	)
	d := BuildDashboardExport(g, DashboardConfig{})
	if d.TopGatherScatter == nil {
		t.Fatal("expected gather_scatter")
	}
	if d.TopGatherScatter.CenterTxid != "h" {
		t.Fatalf("center=%s", d.TopGatherScatter.CenterTxid)
	}
	if len(d.TopGatherScatter.Subgraph.Edges) != 4 {
		t.Fatalf("want 4 edges, got %d", len(d.TopGatherScatter.Subgraph.Edges))
	}
}

func TestFanOutStar(t *testing.T) {
	var edges []features.TxEdge
	for _, x := range []string{"b", "c", "d", "e"} {
		edges = append(edges, features.TxEdge{From: "hub", To: x, Amount: 1})
	}
	sg := BuildSpendGraph(subgraph(edges...))
	topo := TopologicalSort(sg)
	longest, _, _ := LongestPathDepth(sg, topo)
	rows := Analyze(sg, longest)
	var hub *NodeMetrics
	for i := range rows {
		if rows[i].TxID == "hub" {
			hub = &rows[i]
			break
		}
	}
	if hub == nil || hub.FanOut != 4 || hub.FanIn != 0 {
		t.Fatalf("hub metrics %+v", hub)
	}
	d := BuildDashboardExport(subgraph(edges...), DashboardConfig{})
	if d.TopFanOut == nil || d.TopFanOut.CenterTxid != "hub" || d.TopFanOut.MetricValue != 4 {
		t.Fatalf("top fan out %+v", d.TopFanOut)
	}
}

func TestFanInStar(t *testing.T) {
	var edges []features.TxEdge
	for _, x := range []string{"a", "b", "c"} {
		edges = append(edges, features.TxEdge{From: x, To: "sink", Amount: 1})
	}
	sg := BuildSpendGraph(subgraph(edges...))
	topo := TopologicalSort(sg)
	longest, _, _ := LongestPathDepth(sg, topo)
	rows := Analyze(sg, longest)
	var sink *NodeMetrics
	for i := range rows {
		if rows[i].TxID == "sink" {
			sink = &rows[i]
			break
		}
	}
	if sink == nil || sink.FanIn != 3 || sink.FanOut != 0 {
		t.Fatalf("sink metrics %+v", sink)
	}
	d := BuildDashboardExport(subgraph(edges...), DashboardConfig{})
	if d.TopFanIn == nil || d.TopFanIn.CenterTxid != "sink" || d.TopFanIn.MetricValue != 3 {
		t.Fatalf("top fan in %+v", d.TopFanIn)
	}
}

func TestHourglass_hub(t *testing.T) {
	g := subgraph(
		features.TxEdge{From: "p1", To: "h", Amount: 1},
		features.TxEdge{From: "p2", To: "h", Amount: 1},
		features.TxEdge{From: "h", To: "c1", Amount: 1},
		features.TxEdge{From: "h", To: "c2", Amount: 1},
	)
	sg := BuildSpendGraph(g)
	topo := TopologicalSort(sg)
	longest, _, _ := LongestPathDepth(sg, topo)
	rows := Analyze(sg, longest)
	var hub *NodeMetrics
	for i := range rows {
		if rows[i].TxID == "h" {
			hub = &rows[i]
			break
		}
	}
	if hub == nil {
		t.Fatal("missing h")
	}
	if hub.FanIn != 2 || hub.FanOut != 2 || hub.HubScore != 4 || !hub.ScatterGatherHint {
		t.Fatalf("hub %+v", hub)
	}
}

func TestReport_smoke(t *testing.T) {
	g := subgraph(
		features.TxEdge{From: "a", To: "b", Amount: 1},
	)
	var buf bytes.Buffer
	err := Report(g, Options{Writer: &buf, Quiet: false})
	if err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, "nodes=2") || !strings.Contains(s, "dag=true") {
		t.Fatalf("output: %q", s)
	}
}

func TestStarSubgraphTruncation(t *testing.T) {
	var edges []features.TxEdge
	for i := 0; i < 100; i++ {
		edges = append(edges, features.TxEdge{From: fmt.Sprintf("p%03d", i), To: "sink", Amount: int64(i)})
	}
	d := BuildDashboardExport(subgraph(edges...), DashboardConfig{MaxStarEdges: 7})
	if d.TopFanIn == nil {
		t.Fatal("expected top_fan_in")
	}
	sg := d.TopFanIn.Subgraph
	if sg.TotalEdgesAvailable != 100 || !sg.EdgesTruncated || len(sg.Edges) != 7 {
		t.Fatalf("got total=%d trunc=%v len(edges)=%d", sg.TotalEdgesAvailable, sg.EdgesTruncated, len(sg.Edges))
	}
}

func TestPercentileThreshold(t *testing.T) {
	v := []int{1, 2, 3, 4, 5}
	if percentileThreshold(sortedIntsCopy(v), 50) != 3 {
		t.Fatal()
	}
	if percentileThresholdInt64(sortedInt64Copy([]int64{10, 20, 30}), 100) != 30 {
		t.Fatal()
	}
}
