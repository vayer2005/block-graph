import fs from "node:fs";
import path from "node:path";
import { spawn } from "node:child_process";
import type { Plugin } from "vite";

/** Repo root (parent of `dashboard/`). */
export function anomalyDashboardPlugin(dashboardDir: string): Plugin {
  const repoRoot = path.resolve(dashboardDir, "..");
  const jsonPath = path.join(repoRoot, "anomaly-dashboard.json");
  const publicJson = path.join(dashboardDir, "public", "anomaly-dashboard.json");

  return {
    name: "anomaly-dashboard",
    buildStart() {
      if (fs.existsSync(jsonPath)) {
        fs.mkdirSync(path.dirname(publicJson), { recursive: true });
        fs.copyFileSync(jsonPath, publicJson);
      }
    },
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const pathname = req.url?.split("?")[0] ?? "";

        if (pathname === "/api/refresh-dashboard" && req.method === "POST") {
          res.setHeader("Content-Type", "application/json");
          try {
            fs.writeFileSync(jsonPath, "", "utf8");
          } catch (e) {
            res.statusCode = 500;
            res.end(
              JSON.stringify({
                ok: false,
                error: `clear ${jsonPath}: ${String(e)}`,
              }),
            );
            return;
          }

          const child = spawn("go", ["run", "./ingestor/cmd/"], {
            cwd: repoRoot,
            env: process.env,
            stdio: ["ignore", "pipe", "pipe"],
          });
          let stderr = "";
          child.stderr?.on("data", (chunk: Buffer) => {
            stderr += chunk.toString();
          });

          const code: number = await new Promise((resolve) => {
            child.on("error", () => resolve(-1));
            child.on("close", (c) => resolve(c ?? -1));
          });

          if (code !== 0) {
            res.statusCode = 500;
            res.end(
              JSON.stringify({
                ok: false,
                error: `go run exited ${code}`,
                stderr: stderr.slice(-8000),
              }),
            );
            return;
          }

          res.statusCode = 200;
          res.end(JSON.stringify({ ok: true }));
          return;
        }

        if (pathname === "/anomaly-dashboard.json" && req.method === "GET") {
          if (!fs.existsSync(jsonPath)) {
            res.statusCode = 404;
            res.end(JSON.stringify({ error: "anomaly-dashboard.json not found" }));
            return;
          }
          res.setHeader("Content-Type", "application/json");
          fs.createReadStream(jsonPath)
            .on("error", () => {
              res.statusCode = 500;
              res.end();
            })
            .pipe(res);
          return;
        }

        next();
      });
    },
  };
}
