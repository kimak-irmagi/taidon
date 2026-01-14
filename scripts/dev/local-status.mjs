import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { isWindows, run } from "../_lib.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const repoRoot = path.resolve(__dirname, "..", "..");
const workspace = process.argv[2] ? path.resolve(process.argv[2]) : repoRoot;
const node = process.execPath;

await run({ cmd: [node, path.join(repoRoot, "scripts", "build-engine-go.mjs"), workspace], cwd: repoRoot });
await run({ cmd: [node, path.join(repoRoot, "scripts", "build-cli-go.mjs"), workspace], cwd: repoRoot });

const distBin = path.resolve(workspace, "dist", "bin");
const engineBin = path.join(distBin, isWindows() ? "sqlrs-engine.exe" : "sqlrs-engine");
const cliBin = path.join(distBin, isWindows() ? "sqlrs.exe" : "sqlrs");

if (!fs.existsSync(engineBin)) {
  throw new Error(`engine binary missing: ${engineBin}`);
}
if (!fs.existsSync(cliBin)) {
  throw new Error(`cli binary missing: ${cliBin}`);
}

const workRoot = path.join(workspace, "sqlrs-work");
const env = { ...process.env, SQLRS_DAEMON_PATH: engineBin };

if (isWindows()) {
  env.APPDATA = path.join(workRoot, "config");
  env.LOCALAPPDATA = path.join(workRoot, "state");
} else {
  env.XDG_CONFIG_HOME = path.join(workRoot, "config");
  env.XDG_STATE_HOME = path.join(workRoot, "state");
  env.XDG_CACHE_HOME = path.join(workRoot, "cache");
}

await run({ cmd: [cliBin, "status"], cwd: workspace, env, tee: true });
