import { useEffect, useMemo, useState } from "react";
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
  const [loadingBlocks, setLoadingBlocks] = useState(true);
  const [generated, setGenerated] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoadingBlocks(true);
      setLoadError(null);
      try {
        const data = await loadDashboard();
        if (!cancelled) {
          setDashboard(data);
        }
      } catch (e) {
        if (!cancelled) {
          setLoadError(e instanceof Error ? e.message : String(e));
        }
      } finally {
        if (!cancelled) {
          setLoadingBlocks(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const fanIn = useMemo(() => {
    if (!generated || !dashboard?.top_fan_in) return emptyScene;
    return layoutFanIn(dashboard.top_fan_in);
  }, [generated, dashboard]);

  const fanOut = useMemo(() => {
    if (!generated || !dashboard?.top_fan_out) return emptyScene;
    return layoutFanOut(dashboard.top_fan_out);
  }, [generated, dashboard]);

  const gatherScatter = useMemo(() => {
    if (!generated || !dashboard?.top_gather_scatter) return emptyScene;
    return layoutGatherScatter(dashboard.top_gather_scatter);
  }, [generated, dashboard]);

  const longestHop = useMemo(() => {
    if (!generated || !dashboard?.longest_hop_path) return emptyScene;
    return layoutLongestPathFlow(dashboard.longest_hop_path);
  }, [generated, dashboard]);

  return (
    <div className="app">
      {loadingBlocks && (
        <div className="loading-overlay" role="dialog" aria-modal="true" aria-busy="true">
          <div className="loading-card">
            <div className="loading-spinner" aria-hidden />
            <p className="loading-title">Blocks are being fetched</p>
            <p className="loading-sub">
              Clearing <code>anomaly-dashboard.json</code> and running the ingestor — this may take a
              minute.
            </p>
          </div>
        </div>
      )}

      <header className="header">
        <div>
          <h1>Anomaly patterns</h1>
          <p className="sub">
            Data: <code>anomaly-dashboard.json</code> — merged graph{" "}
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
              <>…</>
            )}
          </p>
        </div>
        <button
          type="button"
          className="generate-btn"
          disabled={!dashboard || loadingBlocks}
          onClick={() => setGenerated(true)}
        >
          Generate visualizations
        </button>
      </header>

      {loadError && !loadingBlocks && (
        <p className="hint error-text">
          Failed to refresh dashboard data. Fix the error above and reload the page.
        </p>
      )}

      {!dashboard || loadError ? null : !generated ? (
        <p className="hint">Click the button to lay out WebGL scenes from the JSON highlights.</p>
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
