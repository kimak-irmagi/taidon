import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import YAML from "yaml";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, "..", "..");
const workflowPath = path.join(repoRoot, ".github", "workflows", "e2e-windows-wsl-probe.yml");

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

run("probe workflow has windows-latest job", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["windows-wsl-happy"];
  assert.ok(job, "missing windows-wsl-happy job");
  assert.equal(job["runs-on"], "windows-latest");
});

run("probe workflow has temporary push trigger for non-main branches", () => {
  const workflow = loadWorkflow();
  const push = workflow.on?.push;
  assert.ok(push, "missing push trigger");
  assert.deepEqual(push["branches-ignore"], ["main"]);
});

run("probe workflow installs WSL via setup-wsl action", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["windows-wsl-happy"];
  const step = (job.steps || []).find((item) => item.name === "Set up WSL");
  assert.ok(step, "missing WSL setup step");
  assert.equal(step.uses, "Vampire/setup-wsl@v6");
});

run("probe workflow sets up docker on windows host", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["windows-wsl-happy"];
  const step = (job.steps || []).find((item) => item.name === "Set up Docker on host");
  assert.ok(step, "missing host docker setup step");
  assert.equal(step.uses, "docker/setup-docker-action@v4");
});

run("probe workflow fetches locked chinook sql asset", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["windows-wsl-happy"];
  const step = (job.steps || []).find((item) => item.name === "Fetch Chinook SQL asset (locked)");
  assert.ok(step, "missing chinook asset fetch step");
  assert.match(String(step.run || ""), /Chinook_PostgreSql\.sql/);
  assert.match(String(step.run || ""), /e3fde5c1a5b51a2a91429a702c9ca6e69ba56e6c7f5e112724d70c3d03db695e/);
});

run("probe workflow runs happy-path in WSL shell", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["windows-wsl-happy"];
  const step = (job.steps || []).find((item) => item.name === "Run chinook happy-path inside WSL");
  assert.ok(step, "missing WSL scenario step");
  assert.equal(step.shell, "wsl-bash {0}");
  assert.match(String(step.run || ""), /prepare:psql/);
  assert.match(String(step.run || ""), /postgres:17/);
});

run("probe workflow validates output against chinook golden", () => {
  const workflow = loadWorkflow();
  const job = workflow.jobs?.["windows-wsl-happy"];
  const step = (job.steps || []).find((item) => item.name === "Normalize and compare golden output");
  assert.ok(step, "missing normalize/compare step");
  assert.match(String(step.run || ""), /normalize-output\.mjs/);
  assert.match(String(step.run || ""), /compare-golden\.mjs/);
  assert.match(String(step.run || ""), /hp-psql-chinook\/golden\.txt/);
});
