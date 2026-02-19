import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import YAML from "yaml";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, "..", "..");
const workflowPath = path.join(repoRoot, ".github", "workflows", "release-local.yml");

function run(name, fn) {
  try {
    fn();
    process.stdout.write(`ok - ${name}\n`);
  } catch (err) {
    process.stderr.write(`not ok - ${name}\n`);
    throw err;
  }
}

function loadWorkflow() {
  const raw = fs.readFileSync(workflowPath, "utf8");
  return YAML.parse(raw);
}

run("linux e2e matrix includes snapshot backend axis", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["e2e-linux-happy"];
  assert.ok(job, "missing e2e-linux-happy job");
  assert.deepEqual(job.strategy?.matrix?.snapshot_backend, ["copy", "btrfs"]);
});

run("linux e2e passes snapshot backend to run-scenario", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["e2e-linux-happy"];
  const runStep = (job.steps || []).find((step) => step.name === "Run happy-path scenario");
  assert.ok(runStep, "missing run step");
  assert.match(String(runStep.run || ""), /--snapshot-backend "\$\{\{ matrix\.snapshot_backend \}\}"/);
});

run("linux diagnostics artifacts are backend-specific", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["e2e-linux-happy"];
  const uploadStep = (job.steps || []).find((step) => step.name === "Upload Linux E2E diagnostics");
  assert.ok(uploadStep, "missing upload step");
  assert.match(String(uploadStep.with?.name || ""), /\$\{\{ matrix\.snapshot_backend \}\}/);
});

run("linux btrfs profile installs required tooling", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["e2e-linux-happy"];
  const step = (job.steps || []).find((item) => item.name === "Install btrfs tooling");
  assert.ok(step, "missing btrfs tooling step");
  assert.equal(String(step.if || "").trim(), "matrix.snapshot_backend == 'btrfs'");
  assert.match(String(step.run || ""), /apt-get install -y btrfs-progs/);
});
