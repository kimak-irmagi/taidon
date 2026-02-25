import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  buildInitCommand,
  resolveFlowRuns,
  resolveSnapshotBackend,
  resolveStorePlan
} from "./run-scenario.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

function run(name, fn) {
  try {
    fn();
    process.stdout.write(`ok - ${name}\n`);
  } catch (err) {
    process.stderr.write(`not ok - ${name}\n`);
    throw err;
  }
}

run("explicit btrfs backend is passed into init command", () => {
  const sqlrsPath = path.join(__dirname, "fake-sqlrs");
  const workspaceDir = path.join(__dirname, "tmp-workspace");
  const enginePath = path.join(__dirname, "fake-engine");
  const storeDir = path.join(__dirname, "tmp-store");

  const cmd = buildInitCommand({
    sqlrsPath,
    workspaceDir,
    enginePath,
    storeDir,
    snapshotBackend: resolveSnapshotBackend("btrfs")
  });

  const snapshotIndex = cmd.indexOf("--snapshot");
  assert.notEqual(snapshotIndex, -1);
  assert.equal(cmd[snapshotIndex + 1], "btrfs");
});

run("default snapshot backend is copy", () => {
  assert.equal(resolveSnapshotBackend(undefined), "copy");
  assert.equal(resolveSnapshotBackend(""), "copy");
});

run("unknown snapshot backend is rejected", () => {
  assert.throws(() => resolveSnapshotBackend("zfs"), /Invalid --snapshot-backend: zfs/);
});

run("default flow runs is 1", () => {
  assert.equal(resolveFlowRuns(undefined), 1);
  assert.equal(resolveFlowRuns(""), 1);
});

run("flow runs parses positive integer", () => {
  assert.equal(resolveFlowRuns("2"), 2);
});

run("invalid flow runs is rejected", () => {
  assert.throws(() => resolveFlowRuns("0"), /Invalid --flow-runs: 0/);
  assert.throws(() => resolveFlowRuns("-1"), /Invalid --flow-runs: -1/);
  assert.throws(() => resolveFlowRuns("abc"), /Invalid --flow-runs: abc/);
});

run("btrfs backend uses dedicated loopback btrfs store plan", () => {
  const outDir = path.join(path.sep, "tmp", "e2e-out");
  const plan = resolveStorePlan("btrfs", outDir);
  assert.equal(plan.mountType, "btrfs-loop");
  assert.equal(plan.mountDir, path.join(outDir, "btrfs-store"));
  assert.equal(plan.storeDir, path.join(outDir, "btrfs-store", "store"));
  assert.equal(plan.imagePath, path.join(outDir, "btrfs-store.img"));
});

run("non-btrfs backend uses plain directory store plan", () => {
  const outDir = path.join(path.sep, "tmp", "e2e-out");
  const plan = resolveStorePlan("copy", outDir);
  assert.equal(plan.mountType, "plain-dir");
  assert.equal(plan.storeDir, path.join(outDir, "store"));
});
