/** Mirrors ingestor/anomaly JSON (subset used by the UI). */

export interface DashboardMeta {
  node_count: number;
  edge_count: number;
  is_dag: boolean;
  longest_path_edge_count_global: number;
}

export interface NodeView {
  txid: string;
  fan_in: number;
  fan_out: number;
  in_sats: number;
  out_sats: number;
  path_order?: number;
}

export interface EdgeView {
  from: string;
  to: string;
  amount_sat: number;
}

export interface SubgraphJSON {
  nodes: NodeView[];
  edges: EdgeView[];
  edges_truncated?: boolean;
  total_edges_available?: number;
}

export interface MetricHighlight {
  kind: string;
  metric_value: number;
  center_txid: string;
  metric_detail?: string;
  subgraph: SubgraphJSON;
}

export interface PathHighlight {
  kind: string;
  edge_count: number;
  subgraph: SubgraphJSON;
}

export interface AnomalyDashboard {
  meta: DashboardMeta;
  top_fan_in?: MetricHighlight;
  top_fan_out?: MetricHighlight;
  top_gather_scatter?: MetricHighlight;
  longest_hop_path?: PathHighlight;
}

/** Center = blue + larger in the WebGL view; peripheral = purple. */
export type PointRole = "center" | "peripheral";

export interface PlacedPoint {
  position: [number, number, number];
  txid: string;
  label: string;
  role: PointRole;
}

export interface PlacedEdge {
  from: [number, number, number];
  to: [number, number, number];
}

/** Points + line segments for one panel. */
export interface LayoutScene {
  points: PlacedPoint[];
  edges: PlacedEdge[];
}
