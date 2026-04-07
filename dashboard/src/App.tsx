import { useMemo, useState } from "react";
import type { AnomalyDashboard, LayoutScene } from "./types";
import {
  layoutFanIn,
  layoutFanOut,
  layoutGatherScatter,
  layoutLongestPathFlow,
} from "./layouts";
import { ScenePanel } from "./ScenePanel";
import "./app.css";

const emptyScene: LayoutScene = { points: [], edges: [] };

async function loadDashboard(): Promise<AnomalyDashboard> {
  if (import.meta.env.DEV) {
    const refresh = await fetch("/api/refresh-dashboard", { method: "POST" });
    const refreshBody = (await refresh.json()) as {
      ok?: boolean;
      error?: string;
      stderr?: string;
    };
    if (!refresh.ok || !refreshBody.ok) {
      const detail = [refreshBody.error, refreshBody.stderr].filter(Boolean).join("\n");
      throw new Error(detail || `refresh failed (${refresh.status})`);
    }
  }
  const r = await fetch(`/anomaly-dashboard.json?t=${Date.now()}`);
  if (!r.ok) {
    throw new Error(`load ${r.url}: ${r.status}`);
  }
  return (await r.json()) as AnomalyDashboard;
}

export default function App() {
  const [dashboard, setDashboard] = useState<AnomalyDashboard | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loadingBlocks, setLoadingBlocks] = useState(false);

  async function handleGenerate() {
    setLoadError(null);
    setLoadingBlocks(true);
    try {
      const data = await loadDashboard();
      setDashboard(data);
    } catch (e) {
      setLoadError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoadingBlocks(false);
    }
  }

  const fanIn = useMemo(() => {
    if (!dashboard?.top_fan_in) return emptyScene;
    return layoutFanIn(dashboard.top_fan_in);
  }, [dashboard]);

  const fanOut = useMemo(() => {
    if (!dashboard?.top_fan_out) return emptyScene;
    return layoutFanOut(dashboard.top_fan_out);
  }, [dashboard]);

  const gatherScatter = useMemo(() => {
    if (!dashboard?.top_gather_scatter) return emptyScene;
    return layoutGatherScatter(dashboard.top_gather_scatter);
  }, [dashboard]);

  const longestHop = useMemo(() => {
    if (!dashboard?.longest_hop_path) return emptyScene;
    return layoutLongestPathFlow(dashboard.longest_hop_path);
  }, [dashboard]);

  return (
    <div className="app">
      {loadingBlocks && (
        <div className="loading-overlay" role="dialog" aria-modal="true" aria-busy="true">
          <div className="loading-card">
            <div className="loading-spinner" aria-hidden />
            <p className="loading-title">Blocks are being fetched</p>
            <p className="loading-sub">
              Fetching most recent Bitcoin blocks. this may take a
              minute.
            </p>
          </div>
        </div>
      )}

      <header className="header">
        <div>
          <h1>Anomaly patterns in most recent Bitcoin blocks</h1>
          <p className="sub">
            Data: {" "}
            {dashboard ? (
              <>
                <strong>{dashboard.meta.node_count.toLocaleString()}</strong> nodes,{" "}
                <strong>{dashboard.meta.edge_count.toLocaleString()}</strong> edges, DAG:{" "}
                {String(dashboard.meta.is_dag)}, longest path edges:{" "}
                {dashboard.meta.longest_path_edge_count_global}
              </>
            ) : loadError ? (
              <>could not load ({loadError})</>
            ) : (
              <>click the button below to fetch blocks and load metrics</>
            )}
          </p>
        </div>
        <button
          type="button"
          className="generate-btn"
          disabled={loadingBlocks}
          onClick={handleGenerate}
        >
          Generate visualizations
        </button>
      </header>

      {loadError && !loadingBlocks && (
        <p className="hint error-text">
          Failed to refresh dashboard data. Fix the error above and try again.
        </p>
      )}

      {!dashboard ? (
        <p className="hint">
          Click <strong>Generate visualizations</strong> to clear the JSON, run the ingestor, and lay
          out the WebGL scenes.
        </p>
      ) : (
        <div className="grid grid-4">
          <ScenePanel title="top_fan_in" points={fanIn.points} edges={fanIn.edges} />
          <ScenePanel title="top_fan_out" points={fanOut.points} edges={fanOut.edges} />
          <ScenePanel title="top_gather_scatter" points={gatherScatter.points} edges={gatherScatter.edges} />
          <ScenePanel title="longest_hop_path" points={longestHop.points} edges={longestHop.edges} />
        </div>
      )}
    </div>
  );
}
