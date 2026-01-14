import path from "node:path";
import { fileURLToPath } from "node:url";

import { ensureDir, isWindows, run } from "./_lib.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const repoRoot = path.resolve(__dirname, "..");
const workspace = process.argv[2] ? path.resolve(process.argv[2]) : repoRoot;
const engineRoot = path.join(repoRoot, "backend", "local-engine-go");

const distBin = path.resolve(workspace, "dist", "bin");
ensureDir(distBin);

const binaryName = isWindows() ? "sqlrs-engine.exe" : "sqlrs-engine";
const output = path.join(distBin, binaryName);

await run({ cmd: ["go", "build", "-o", output, "./cmd/sqlrs-engine"], cwd: engineRoot });

console.log(`Built sqlrs-engine into: ${output}`);
