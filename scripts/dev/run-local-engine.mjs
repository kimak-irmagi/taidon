import fs from "node:fs";
import path from "node:path";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

import { ensureDir, isWindows, run } from "../_lib.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const repoRoot = path.resolve(__dirname, "..", "..");
const workspace = process.argv[2] ? path.resolve(process.argv[2]) : repoRoot;

const distBin = path.resolve(workspace, "dist", "bin");
const engineBin = path.join(distBin, isWindows() ? "sqlrs-engine.exe" : "sqlrs-engine");

if (!fs.existsSync(engineBin)) {
  const node = process.execPath;
  await run({ cmd: [node, path.join(repoRoot, "scripts", "build-engine-go.mjs"), workspace], cwd: repoRoot });
}

const workRoot = path.join(workspace, "sqlrs-work");
const stateHome = path.join(workRoot, "state");
const stateDir = path.join(stateHome, "sqlrs");
const runDir = path.join(stateDir, "run");
const engineJSON = path.join(stateDir, "engine.json");

ensureDir(runDir);

const args = [
  "--listen",
  "127.0.0.1:0",
  "--run-dir",
  runDir,
  "--write-engine-json",
  engineJSON,
  "--idle-timeout",
  "30s"
];

console.log(`Starting sqlrs-engine: ${engineBin}`);
const child = spawn(engineBin, args, { stdio: "inherit" });

child.on("exit", (code) => {
  process.exit(code ?? 0);
});
