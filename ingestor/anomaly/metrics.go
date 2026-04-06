package anomaly

import (
	"sort"
)

// NodeMetrics holds structural scores for one transaction id (GRAPH_PATTERNS.md).
type NodeMetrics struct {
	TxID              string `json:"txid"`
	FanIn             int    `json:"fan_in"`
	FanOut            int    `json:"fan_out"`
	HubScore          int64  `json:"hub_score"` // FanIn * FanOut as gather-scatter proxy
	InSats            int64  `json:"in_sats"`
	OutSats           int64  `json:"out_sats"`
	LongestPathEnding int    `json:"longest_path_ending"` // edge count on longest path ending here
	ScatterGatherHint bool   `json:"scatter_gather_hint"` // FanIn > 1 && FanOut > 1
}

// Analyze computes per-node metrics using a precomputed longest-path map.
func Analyze(sg *SpendGraph, longestEnding map[string]int) []NodeMetrics {
	if sg == nil {
		return nil
	}
	out := make([]NodeMetrics, 0, len(sg.Nodes))
	for _, id := range sg.Nodes {
		fi := len(sg.In[id])
		fo := len(sg.Out[id])
		var hub int64
		if fi > 0 && fo > 0 {
			hub = int64(fi) * int64(fo)
		}
		lp := 0
		if longestEnding != nil {
			lp = longestEnding[id]
		}
		out = append(out, NodeMetrics{
			TxID:              id,
			FanIn:             fi,
			FanOut:            fo,
			HubScore:          hub,
			InSats:            sg.InSats[id],
			OutSats:           sg.OutSats[id],
			LongestPathEnding: lp,
			ScatterGatherHint: fi > 1 && fo > 1,
		})
	}
	return out
}

func percentileThreshold(sortedAsc []int, p float64) int {
	if len(sortedAsc) == 0 || p <= 0 {
		return 0
	}
	if p >= 100 {
		return sortedAsc[len(sortedAsc)-1]
	}
	idx := int(float64(len(sortedAsc)-1) * p / 100.0)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sortedAsc) {
		idx = len(sortedAsc) - 1
	}
	return sortedAsc[idx]
}

func sortedIntsCopy(vals []int) []int {
	cp := append([]int(nil), vals...)
	sort.Ints(cp)
	return cp
}

func sortedInt64Copy(vals []int64) []int64 {
	cp := append([]int64(nil), vals...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	return cp
}

func percentileThresholdInt64(sortedAsc []int64, p float64) int64 {
	if len(sortedAsc) == 0 || p <= 0 {
		return 0
	}
	if p >= 100 {
		return sortedAsc[len(sortedAsc)-1]
	}
	idx := int(float64(len(sortedAsc)-1) * p / 100.0)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sortedAsc) {
		idx = len(sortedAsc) - 1
	}
	return sortedAsc[idx]
}

// rankBy returns indices into rows sorted by metric descending (tie-break txid).
func rankBy(rows []NodeMetrics, less func(a, b NodeMetrics) bool) []int {
	idx := make([]int, len(rows))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(i, j int) bool {
		a, b := rows[idx[i]], rows[idx[j]]
		if less(a, b) {
			return false
		}
		if less(b, a) {
			return true
		}
		return a.TxID < b.TxID
	})
	return idx
}
