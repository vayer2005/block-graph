import type { MetricHighlight, NodeView, PathHighlight, PlacedEdge, PlacedPoint, LayoutScene } from "./types";

/** Deterministic pseudo-random in [0, 1) from string (stable layout). */
function hash01(s: string, salt: number): number {
  let h = salt;
  for (let i = 0; i < s.length; i++) {
    h = Math.imul(31, h) + s.charCodeAt(i);
  }
  return ((h >>> 0) % 10000) / 10000;
}

function nodeLabel(n: NodeView): string {
  const id = n.txid.length > 16 ? `${n.txid.slice(0, 8)}…${n.txid.slice(-6)}` : n.txid;
  return `${id}\nfan_in: ${n.fan_in}  fan_out: ${n.fan_out}\nin: ${n.in_sats} sats  out: ${n.out_sats} sats`;
}

function nodeByTxid(nodes: NodeView[], txid: string): NodeView | undefined {
  return nodes.find((n) => n.txid === txid);
}

/** Wider grid for fan-in / fan-out — fewer columns, larger steps so nodes do not stack. */
const GRID_COLS = 7;
const GRID_COL_STEP = 0.78;
const GRID_ROW_STEP = 1.2;
const GRID_JITTER = 0.62;

/** Right-side cluster positions for many txids (fan-in / fan-out). */
function placeRightCluster(txids: string[], baseX: number): Map<string, [number, number, number]> {
  const pos = new Map<string, [number, number, number]>();
  txids.forEach((txid, i) => {
    const row = Math.floor(i / GRID_COLS);
    const col = i % GRID_COLS;
    const jx = (hash01(txid, 1) - 0.5) * GRID_JITTER;
    const jy = (hash01(txid, 2) - 0.5) * GRID_JITTER;
    const x = baseX + (col - GRID_COLS / 2) * GRID_COL_STEP + jx;
    const y = (row - Math.floor(txids.length / GRID_COLS / 2)) * GRID_ROW_STEP + jy;
    pos.set(txid, [x, y, 0]);
  });
  return pos;
}

/**
 * Gather-scatter: nodes on a wide arc so edges to the hub fan out at different angles
 * (reduces overlapping into a single wedge).
 */
function placeGatherArc(
  txids: string[],
  side: "left" | "right",
): Map<string, [number, number, number]> {
  const pos = new Map<string, [number, number, number]>();
  const sorted = [...txids].sort();
  const n = sorted.length;
  if (n === 0) return pos;

  const centerAngle = side === "left" ? Math.PI : 0;
  const spread = Math.min(1.95, 0.55 + n * 0.028);
  const rings = Math.max(3, Math.ceil(n / 10));
  const baseR = 6.2 + Math.min(2.4, n * 0.035);

  for (let i = 0; i < n; i++) {
    const txid = sorted[i];
    const t = n === 1 ? 0.5 : i / (n - 1);
    const angle = centerAngle + (t - 0.5) * spread;
    const ring = i % rings;
    const r = baseR + ring * 1.05;
    const jx = (hash01(txid, 31) - 0.5) * 0.7;
    const jy = (hash01(txid, 32) - 0.5) * 0.7;
    const jz = (hash01(txid, 33) - 0.5) * 0.45;
    const x = r * Math.cos(angle) + jx;
    const y = r * Math.sin(angle) + jy;
    pos.set(txid, [x, y, jz]);
  }
  return pos;
}

const CENTER_LEFT = -3.8;
const CLUSTER_RIGHT = 5.6;

/** Fan-in: blue hub on the right; predecessor cluster on the left; edges point inward (→ center). */
const FAN_IN_CENTER_X = 3.8;
const FAN_IN_CLUSTER_X = -5.6;

export function layoutFanIn(h: MetricHighlight): LayoutScene {
  const { subgraph, center_txid } = h;
  const center = nodeByTxid(subgraph.nodes, center_txid);
  const pos = new Map<string, [number, number, number]>();
  pos.set(center_txid, [FAN_IN_CENTER_X, 0, 0]);

  const preds = new Set<string>();
  for (const e of subgraph.edges) {
    if (e.to === center_txid) preds.add(e.from);
  }
  const predList = [...preds];
  const leftCluster = placeRightCluster(predList, FAN_IN_CLUSTER_X);
  predList.forEach((t) => {
    const p = leftCluster.get(t);
    if (p) pos.set(t, p);
  });

  for (const n of subgraph.nodes) {
    if (!pos.has(n.txid)) {
      pos.set(n.txid, [FAN_IN_CLUSTER_X + (hash01(n.txid, 9) - 0.5) * 2.4, (hash01(n.txid, 10) - 0.5) * 6.5, 0]);
    }
  }

  const points: PlacedPoint[] = [];
  if (center) {
    points.push({
      position: pos.get(center_txid)!,
      txid: center_txid,
      label: `CENTER (${h.kind})\n${nodeLabel(center)}\nmetric: ${h.metric_value}`,
      role: "center",
    });
  }
  for (const n of subgraph.nodes) {
    if (n.txid === center_txid) continue;
    const p = pos.get(n.txid)!;
    points.push({
      position: p,
      txid: n.txid,
      label: nodeLabel(n),
      role: "peripheral",
    });
  }

  const edges: PlacedEdge[] = [];
  for (const e of subgraph.edges) {
    if (e.to !== center_txid) continue;
    const a = pos.get(e.from);
    const b = pos.get(e.to);
    if (a && b) edges.push({ from: a, to: b });
  }

  return { points, edges };
}

/** Fan-out: blue center left; cluster on the right; edges point outward (center → leaves). */
export function layoutFanOut(h: MetricHighlight): LayoutScene {
  const { subgraph, center_txid } = h;
  const center = nodeByTxid(subgraph.nodes, center_txid);
  const pos = new Map<string, [number, number, number]>();
  pos.set(center_txid, [CENTER_LEFT, 0, 0]);

  const succs = new Set<string>();
  for (const e of subgraph.edges) {
    if (e.from === center_txid) succs.add(e.to);
  }
  const succList = [...succs];
  const right = placeRightCluster(succList, CLUSTER_RIGHT);
  succList.forEach((t) => {
    const p = right.get(t);
    if (p) pos.set(t, p);
  });

  for (const n of subgraph.nodes) {
    if (!pos.has(n.txid)) {
      pos.set(n.txid, [CLUSTER_RIGHT + hash01(n.txid, 11) * 1.2, (hash01(n.txid, 12) - 0.5) * 6.5, 0]);
    }
  }

  const points: PlacedPoint[] = [];
  if (center) {
    points.push({
      position: pos.get(center_txid)!,
      txid: center_txid,
      label: `CENTER (${h.kind})\n${nodeLabel(center)}\nmetric: ${h.metric_value}`,
      role: "center",
    });
  }
  for (const n of subgraph.nodes) {
    if (n.txid === center_txid) continue;
    points.push({
      position: pos.get(n.txid)!,
      txid: n.txid,
      label: nodeLabel(n),
      role: "peripheral",
    });
  }

  const edges: PlacedEdge[] = [];
  for (const e of subgraph.edges) {
    if (e.from !== center_txid) continue;
    const a = pos.get(e.from);
    const b = pos.get(e.to);
    if (a && b) edges.push({ from: a, to: b });
  }

  return { points, edges };
}

/** Gather-scatter: center middle; predecessors left, successors right; all subgraph edges drawn. */
export function layoutGatherScatter(h: MetricHighlight): LayoutScene {
  const { subgraph, center_txid } = h;
  const center = nodeByTxid(subgraph.nodes, center_txid);
  const pos = new Map<string, [number, number, number]>();
  pos.set(center_txid, [0, 0, 0]);

  const preds = new Set<string>();
  const succs = new Set<string>();
  for (const e of subgraph.edges) {
    if (e.to === center_txid) preds.add(e.from);
    if (e.from === center_txid) succs.add(e.to);
  }

  const predList = [...preds].sort();
  const succList = [...succs].sort();
  placeGatherArc(predList, "left").forEach((p, k) => pos.set(k, p));
  placeGatherArc(succList, "right").forEach((p, k) => pos.set(k, p));

  for (const n of subgraph.nodes) {
    if (pos.has(n.txid)) continue;
    const side = hash01(n.txid, 20) > 0.5 ? 1 : -1;
    const angle = side === 1 ? hash01(n.txid, 21) * 1.2 - 0.6 : Math.PI + hash01(n.txid, 21) * 1.2 - 0.6;
    const r = 6.5 + hash01(n.txid, 22) * 2.5;
    pos.set(n.txid, [r * Math.cos(angle), r * Math.sin(angle), (hash01(n.txid, 23) - 0.5) * 0.5]);
  }

  const points: PlacedPoint[] = [];
  if (center) {
    points.push({
      position: [0, 0, 0],
      txid: center_txid,
      label: `CENTER (${h.kind})\n${nodeLabel(center)}\nmetric: ${h.metric_value}`,
      role: "center",
    });
  }
  for (const n of subgraph.nodes) {
    if (n.txid === center_txid) continue;
    points.push({
      position: pos.get(n.txid)!,
      txid: n.txid,
      label: nodeLabel(n),
      role: "peripheral",
    });
  }

  const edges: PlacedEdge[] = [];
  for (const e of subgraph.edges) {
    const a = pos.get(e.from);
    const b = pos.get(e.to);
    if (a && b) edges.push({ from: a, to: b });
  }

  return { points, edges };
}

/** Longest path: flow layout; consecutive hops as edges. */
export function layoutLongestPathFlow(p: PathHighlight): LayoutScene {
  const nodes = [...p.subgraph.nodes].sort((a, b) => (a.path_order ?? 0) - (b.path_order ?? 0));
  if (nodes.length === 0) {
    return { points: [], edges: [] };
  }
  const outPoints: PlacedPoint[] = [];
  const span = 14;
  const amp = 1.2;
  const freq = 0.45;
  const positions: [number, number, number][] = [];

  nodes.forEach((node, i) => {
    const t = nodes.length > 1 ? i / (nodes.length - 1) : 0;
    const x = (t - 0.5) * span;
    const y = Math.sin(t * Math.PI * freq * 3) * amp + (hash01(node.txid, 7) - 0.5) * 0.15;
    const z = (hash01(node.txid, 8) - 0.5) * 0.1;
    const pos: [number, number, number] = [x, y, z];
    positions.push(pos);
    outPoints.push({
      position: pos,
      txid: node.txid,
      label: `${nodeLabel(node)}\npath_order: ${node.path_order ?? i}\nedge_count (path): ${p.edge_count}`,
      role: "peripheral",
    });
    if (hash01(node.txid, 9) > 0.55) {
      outPoints.push({
        position: [x + 0.08, y + 0.12, z],
        txid: node.txid,
        label: nodeLabel(node),
        role: "peripheral",
      });
    }
  });

  const edges: PlacedEdge[] = [];
  for (let i = 0; i + 1 < positions.length; i++) {
    edges.push({ from: positions[i], to: positions[i + 1] });
  }

  return { points: outPoints, edges };
}
