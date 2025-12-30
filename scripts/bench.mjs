// scripts/bench.mjs
import yargs from "yargs";
import { hideBin } from "yargs/helpers";
import path from "node:path";
import { run } from "./_lib.mjs";

async function main() {
  const args = await yargs(hideBin(process.argv))
    .option("workspace", { type: "string", demandOption: true })
    .option("repo", { type: "string", default: process.cwd() })
    .option("build", { type: "boolean", default: true })
    .option("examples", {
      type: "string",
      default: "chinook",
      describe: "Comma-separated list: chinook,sakila,postgrespro-demo"
    })
    .option("storages", {
      type: "string",
      default: "plain",
      describe: "Comma-separated list: plain,btrfs,zfs"
    })
    .option("snapshots", {
      type: "string",
      default: "off",
      describe: "Comma-separated list: off,stop"
    })
    .option("image", { type: "string", default: "postgres:17" })
    .parse();
    // On Windows we expect running benchmarks inside WSL2.
  if (process.platform === "win32") {
    const storages = args.storages.split(",").map((s) => s.trim()).filter(Boolean);
    const nonPlain = storages.filter((s) => s !== "plain");
    if (nonPlain.length > 0) {
      console.error(
        [
          "This benchmark should be run inside WSL2 on Windows (Docker Desktop + WSL integration).",
          `Requested storages: ${storages.join(", ")}`,
          "Run it like:",
          "  node scripts/wsl-run.mjs --distro Ubuntu -- pnpm bench -- <same args>",
          "Or open an Ubuntu (WSL) terminal and run pnpm bench there.",
        ].join("\n")
      );
      process.exit(2);
    }
  }

  const repoRoot = path.resolve(args.repo);
  const ws = path.resolve(args.workspace);

  const examples = args.examples.split(",").map((s) => s.trim()).filter(Boolean);
  const storages = args.storages.split(",").map((s) => s.trim()).filter(Boolean);
  const snaps = args.snapshots.split(",").map((s) => s.trim()).filter(Boolean);

  if (args.build) {
    await run({ cmd: ["node", path.join(repoRoot, "scripts", "build-cli.mjs"), ws] });
  }

  for (const ex of examples) {
    for (const st of storages) {
      for (const sn of snaps) {
        console.log(`==> ${ex} / ${st} / ${sn}`);
        await run({
          cmd: [
            "node",
            path.join(repoRoot, "scripts", "run-one.mjs"),
            "--workspace", ws,
            "--repo", repoRoot,
            "--build", "false",
            "--example", ex,
            "--storage", st,
            "--snapshots", sn,
            "--image", args.image
          ]
        });
      }
    }
  }
}

main().catch((e) => {
  console.error(e?.stack || String(e));
  process.exit(1);
});
