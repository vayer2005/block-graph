import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { anomalyDashboardPlugin } from "./vite-plugin-anomaly-dashboard";

const dashboardDir = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react(), anomalyDashboardPlugin(dashboardDir)],
});
