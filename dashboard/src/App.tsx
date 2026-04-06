import { useMemo, useState } from "react";
import type { AnomalyDashboard, LayoutScene } from "./types";
import {
  layoutFanIn,
  layoutFanOut,
  layoutGatherScatter,
  layoutLongestPathFlow,
} from "./layouts";
import { ScenePanel } from "./ScenePanel";
import dashboardJson from "../../anomaly-dashboard.json";
import "./app.css";

const dashboard = dashboardJson as AnomalyDashboard;

const emptyScene: LayoutScene = { points: [], edges: [] };

export default function App() {
  const [generated, setGenerated] = useState(false);

  const fanIn = useMemo(() => {
    if (!generated || !dashboard.top_fan_in) return emptyScene;
    return layoutFanIn(dashboard.top_fan_in);
  }, [generated]);

  const fanOut = useMemo(() => {
    if (!generated || !dashboard.top_fan_out) return emptyScene;
    return layoutFanOut(dashboard.top_fan_out);
  }, [generated]);

  const gatherScatter = useMemo(() => {
    if (!generated || !dashboard.top_gather_scatter) return emptyScene;
    return layoutGatherScatter(dashboard.top_gather_scatter);
  }, [generated]);

  const longestHop = useMemo(() => {
    if (!generated || !dashboard.longest_hop_path) return emptyScene;
    return layoutLongestPathFlow(dashboard.longest_hop_path);
  }, [generated]);

  return (
    <div className="app">
      <header className="header">
        <div>
          <h1>Anomaly patterns</h1>
          <p className="sub">
            Data: <code>anomaly-dashboard.json</code> — merged graph{" "}
            <strong>{dashboard.meta.node_count.toLocaleString()}</strong> nodes,{" "}
            <strong>{dashboard.meta.edge_count.toLocaleString()}</strong> edges, DAG:{" "}
            {String(dashboard.meta.is_dag)}, longest path edges:{" "}
            {dashboard.meta.longest_path_edge_count_global}
          </p>
        </div>
        <button
          type="button"
          className="generate-btn"
          onClick={() => setGenerated(true)}
        >
          Generate visualizations
        </button>
      </header>

      {!generated ? (
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
