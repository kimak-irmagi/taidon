// scripts/run-one.mjs
import path from "node:path";
import fs from "node:fs";
import yargs from "yargs";
import { hideBin } from "yargs/helpers";
import { ensureDir, run, sqlrsPath, nowIsoSafe } from "./_lib.mjs";

function examplePaths(repoRoot, example) {
  const base = path.join(repoRoot, "examples", example);
  const prepare = path.join(base, "prepare.sql");
  const queries = path.join(base, "queries.sql");

  if (!fs.existsSync(base)) throw new Error(`Example not found: ${base}`);
  if (!fs.existsSync(prepare)) throw new Error(`prepare.sql not found: ${prepare}`);
  if (!fs.existsSync(queries)) throw new Error(`queries.sql not found: ${queries}`);

  return { base, prepare, queries };
}

async function main() {
  const args = await yargs(hideBin(process.argv))
    .option("repo", { type: "string", default: process.cwd() })
    .option("build", { type: "boolean", default: true, describe: "Build CLI into workspace before run" })
    .option("example", { type: "string", demandOption: true, choices: ["chinook", "sakila", "flights"] })
    .option("storage", { type: "string", demandOption: true, choices: ["plain", "btrfs", "zfs"] })
    .option("image", { type: "string", default: "postgres:17" })
    .option("snapshots", { type: "string", default: "on", choices: ["on", "off"] })
    .option("sqlrsWorkspace", { type: "string", default: "./sqlrs-work" })
    .parse();

  const repoRoot = path.resolve(args.repo);
  const ws = repoRoot;

  if (args.build) {
    await run({ cmd: ["node", path.join(repoRoot, "scripts", "build-cli.mjs"), ws] });
  }

  const { prepare, queries } = examplePaths(repoRoot, args.example);

  const stamp = nowIsoSafe();
  const outDir = path.join(ws, "results", `${stamp}-${args.example}-${args.storage}`);
  ensureDir(outDir);

  const sqlrs = sqlrsPath(ws);

  // NOTE: strict policy: user must pass -c/-f explicitly. We'll use -f for queries.sql.
  const cmd = [
    sqlrs,
    "--from", args.image,
    "--storage", args.storage,
    "--snapshots", args.snapshots,
    "--prepare", "--",
    "psql", "-f", prepare,
    "--run", "--",
    "psql", "-f", queries
  ];
  console.log(`Running: ${cmd.join(" ")}`);
  await run({ cmd, cwd: repoRoot, stdoutFile: path.join(outDir, "run.log"), tee: true });

  console.log(`OK: ${outDir}`);
}

main().catch((e) => {
  console.error(e?.stack || String(e));
  process.exit(1);
});
