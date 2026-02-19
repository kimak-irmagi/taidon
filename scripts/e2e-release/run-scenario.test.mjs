import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  buildInitCommand,
  resolveSnapshotBackend
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
