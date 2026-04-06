package anomaly

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"graph-fraud/ingestor/features"
)

// Options configures reporting. The primary artifact is DashboardJSONPath (top-1 subgraphs for React).
type Options struct {
	// DashboardJSONPath writes BuildDashboardExport (top 1 per metric + small subgraphs).
	DashboardJSONPath string
	// AllMetricsJSONPath, if non-empty, writes the full per-node metrics array (all txids).
	AllMetricsJSONPath string
	// MaxStarEdges caps edges serialized for fan_in / fan_out / gather_scatter (not longest_hop_path).
	// 0 uses DefaultMaxStarEdges (64). Negative means no cap (can be huge).
	MaxStarEdges int
	Quiet          bool
	Writer         io.Writer
}

// Report builds dashboard export and optional full metrics JSON; writes a short text summary unless Quiet.
func Report(g *features.Subgraph, opt Options) error {
	w := opt.Writer
	if w == nil {
		w = os.Stdout
	}

	analysis := analyzeSpendGraph(g)
	dash := buildDashboardExportFrom(analysis, DashboardConfig{MaxStarEdges: opt.MaxStarEdges})

	if opt.DashboardJSONPath != "" {
		data, err := json.MarshalIndent(dash, "", "  ")
		if err != nil {
			return fmt.Errorf("anomaly: dashboard json: %w", err)
		}
		if err := os.WriteFile(opt.DashboardJSONPath, data, 0o644); err != nil {
			return fmt.Errorf("anomaly: write %s: %w", opt.DashboardJSONPath, err)
		}
	}

	if opt.AllMetricsJSONPath != "" {
		data, err := json.MarshalIndent(analysis.rows, "", "  ")
		if err != nil {
			return fmt.Errorf("anomaly: metrics json: %w", err)
		}
		if err := os.WriteFile(opt.AllMetricsJSONPath, data, 0o644); err != nil {
			return fmt.Errorf("anomaly: write %s: %w", opt.AllMetricsJSONPath, err)
		}
	}

	if opt.Quiet {
		return nil
	}

	fmt.Fprintf(w, "anomaly: nodes=%d edges=%d dag=%v meta_longest_edges=%d\n",
		dash.Meta.NodeCount, dash.Meta.EdgeCount, dash.Meta.IsDAG, dash.Meta.LongestPathEdgeCount)
	if dash.TopFanIn != nil {
		fmt.Fprintf(w, "  top_fan_in: center=%s degree=%d\n", dash.TopFanIn.CenterTxid, dash.TopFanIn.MetricValue)
	}
	if dash.TopFanOut != nil {
		fmt.Fprintf(w, "  top_fan_out: center=%s degree=%d\n", dash.TopFanOut.CenterTxid, dash.TopFanOut.MetricValue)
	}
	if dash.TopGatherScatter != nil {
		fmt.Fprintf(w, "  top_gather_scatter: center=%s hub_score=%d\n", dash.TopGatherScatter.CenterTxid, dash.TopGatherScatter.MetricValue)
	}
	if dash.LongestHopPath != nil && dash.LongestHopPath.Subgraph != nil && len(dash.LongestHopPath.Subgraph.Nodes) > 0 {
		ns := dash.LongestHopPath.Subgraph.Nodes
		fmt.Fprintf(w, "  longest_hop_path: edges=%d first=%s last=%s\n",
			dash.LongestHopPath.EdgeCount,
			ns[0].TxID,
			ns[len(ns)-1].TxID,
		)
	} else if !dash.Meta.IsDAG {
		fmt.Fprintf(w, "  longest_hop_path: skipped (not a DAG)\n")
	}

	return nil
}
