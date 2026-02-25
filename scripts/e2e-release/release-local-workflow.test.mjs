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

run("happy e2e matrix includes platform and snapshot backend axes", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["e2e-happy"];
  assert.ok(job, "missing e2e-happy job");
  assert.deepEqual(job.strategy?.matrix?.platform, ["linux", "windows"]);
  assert.deepEqual(job.strategy?.matrix?.snapshot_backend, ["copy", "btrfs"]);
});

run("happy e2e excludes unsupported windows scenario cells", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["e2e-happy"];
  const excluded = job.strategy?.matrix?.exclude || [];
  assert.ok(
    excluded.some((entry) => entry.platform === "windows" && entry.scenario === "hp-psql-sakila"),
    "missing windows/sakila exclusion"
  );
  assert.ok(
    !excluded.some((entry) => entry.platform === "windows" && entry.snapshot_backend === "copy"),
    "windows/copy should be enabled"
  );
});

run("linux e2e cell passes snapshot backend to run-scenario", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["e2e-happy"];
  const runStep = (job.steps || []).find((step) => step.name === "Run happy-path scenario (linux)");
  assert.ok(runStep, "missing run step");
  assert.match(String(runStep.run || ""), /--snapshot-backend "\$\{\{ matrix\.snapshot_backend \}\}"/);
  assert.match(String(runStep.run || ""), /--flow-runs "2"/);
});

run("e2e diagnostics artifacts are backend and platform specific", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["e2e-happy"];
  const uploadStep = (job.steps || []).find((step) => step.name === "Upload Linux E2E diagnostics");
  assert.ok(uploadStep, "missing upload step");
  assert.match(String(uploadStep.with?.name || ""), /\$\{\{ matrix\.platform \}\}/);
  assert.match(String(uploadStep.with?.name || ""), /\$\{\{ matrix\.snapshot_backend \}\}/);
});

run("linux btrfs profile installs required tooling", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["e2e-happy"];
  const step = (job.steps || []).find((item) => item.name === "Install btrfs tooling");
  assert.ok(step, "missing btrfs tooling step");
  assert.equal(String(step.if || "").trim(), "matrix.platform == 'linux' && matrix.snapshot_backend == 'btrfs'");
  assert.match(String(step.run || ""), /apt-get install -y btrfs-progs/);
});

run("windows e2e cell provisions WSL and docker prerequisites", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["e2e-happy"];
  const wslStep = (job.steps || []).find((item) => item.name === "Set up WSL");
  const dockerStep = (job.steps || []).find((item) => item.name === "Set up Docker on host");
  const runStep = (job.steps || []).find((item) => item.name === "Run happy-path scenario (windows host)");
  assert.ok(wslStep, "missing WSL setup step");
  assert.ok(dockerStep, "missing docker setup step");
  assert.ok(runStep, "missing windows run step");
  assert.equal(wslStep.uses, "Vampire/setup-wsl@v6");
  assert.equal(String(wslStep.if || "").trim(), "matrix.platform == 'windows' && matrix.snapshot_backend == 'btrfs'");
  assert.equal(dockerStep.uses, "docker/setup-docker-action@v4");
  assert.match(String(runStep.run || ""), /sqlrs_bin/);
  assert.match(String(runStep.run || ""), /engine_windows_bin/);
  assert.match(String(runStep.run || ""), /engine_linux_bin/);
  assert.match(String(runStep.run || ""), /\$isBtrfs/);
  assert.match(String(runStep.run || ""), /"--store", "dir", \$storeRoot/);
  assert.match(String(runStep.run || ""), /"--store", "image", \$storeImage/);
  assert.match(String(runStep.run || ""), /raw-stdout-run2\.log/);
  assert.match(String(runStep.run || ""), /second pass failed/);
});

run("publish RC waits for unified e2e-happy job", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["publish-rc"];
  assert.ok(job, "missing publish-rc job");
  assert.ok(Array.isArray(job.needs), "publish-rc needs must be an array");
  assert.ok(job.needs.includes("e2e-happy"), "publish-rc must depend on e2e-happy");
});

run("smoke matrix keeps darwin-only cells after windows happy-path integration", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["e2e-smoke"];
  assert.ok(job, "missing e2e-smoke job");
  const include = job.strategy?.matrix?.include || [];
  assert.equal(include.length, 1);
  assert.equal(include[0]?.os_family, "darwin");
});
