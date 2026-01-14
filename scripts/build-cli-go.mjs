import path from "node:path";
import { fileURLToPath } from "node:url";

import { ensureDir, isWindows, run } from "./_lib.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const repoRoot = path.resolve(__dirname, "..");
const workspace = process.argv[2] ? path.resolve(process.argv[2]) : repoRoot;
const cliRoot = path.join(repoRoot, "frontend", "cli-go");

const distBin = path.resolve(workspace, "dist", "bin");
ensureDir(distBin);

const binaryName = isWindows() ? "sqlrs.exe" : "sqlrs";
const output = path.join(distBin, binaryName);

await run({ cmd: ["go", "build", "-o", output, "./cmd/sqlrs"], cwd: cliRoot });

console.log(`Built sqlrs (go) into: ${output}`);
