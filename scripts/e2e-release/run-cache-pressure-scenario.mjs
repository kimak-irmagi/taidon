import fs from "node:fs";
import path from "node:path";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import {
  buildInitCommand,
  buildRuntimeConfigCommand,
  resolveContainerRuntime,
  resolveSnapshotBackend,
  resolveStorePlan
} from "./run-scenario.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRootDefault = path.resolve(__dirname, "..", "..");

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (!arg.startsWith("--")) {
      throw new Error(`Unknown argument: ${arg}`);
    }
    const key = arg.slice(2);
    if (i + 1 >= argv.length) {
      throw new Error(`Missing value for --${key}`);
    }
    out[key] = argv[i + 1];
    i += 1;
  }
  return out;
}

function ensureDir(p) {
  fs.mkdirSync(p, { recursive: true });
}

function rmrf(p) {
  if (!fs.existsSync(p)) {
    return;
  }
  fs.rmSync(p, { recursive: true, force: true });
}

function loadScenario(scenariosPath, scenarioId) {
  const raw = fs.readFileSync(scenariosPath, "utf8");
  const parsed = JSON.parse(raw);
  const scenarios = Array.isArray(parsed.scenarios) ? parsed.scenarios : [];
  const scenario = scenarios.find((item) => item.id === scenarioId);
  if (!scenario) {
    throw new Error(`Scenario not found: ${scenarioId}`);
  }
  return scenario;
}

function commandToString(cmd) {
  return cmd.map((part) => (part.includes(" ") ? `"${part}"` : part)).join(" ");
}

function runCommand({ cmd, cwd, env, stdoutPath, stderrPath }) {
  return new Promise((resolve, reject) => {
    const useShell = process.platform === "win32" && /\.(cmd|bat)$/i.test(cmd[0]);
    const child = spawn(cmd[0], cmd.slice(1), {
      cwd,
      env,
      shell: useShell
    });

    const stdout = fs.createWriteStream(stdoutPath, { flags: "w" });
    const stderr = fs.createWriteStream(stderrPath, { flags: "w" });
    child.stdout.pipe(stdout);
    child.stderr.pipe(stderr);

    const stdoutDone = new Promise((resolveDone, rejectDone) => {
      stdout.on("finish", resolveDone);
      stdout.on("error", rejectDone);
    });
    const stderrDone = new Promise((resolveDone, rejectDone) => {
      stderr.on("finish", resolveDone);
      stderr.on("error", rejectDone);
    });

    let settled = false;
    const settleError = (err) => {
      if (settled) {
        return;
      }
      settled = true;
      stdout.destroy();
      stderr.destroy();
      reject(err);
    };

    child.on("error", settleError);
    child.on("close", (code) => {
      Promise.all([stdoutDone, stderrDone])
        .then(() => {
          if (settled) {
            return;
          }
          settled = true;
          resolve(code ?? 1);
        })
        .catch(settleError);
    });
  });
}

function writeJSON(pathname, value) {
  fs.writeFileSync(pathname, `${JSON.stringify(value, null, 2)}\n`, "utf8");
}

function resolvePrepareCmd(scenario, scenarioId) {
  const prepareCmd = typeof scenario.prepareCmd === "string" && scenario.prepareCmd.trim() !== "" ? scenario.prepareCmd : "prepare:psql";
  const prepareArgs = Array.isArray(scenario.prepareArgs)
    ? scenario.prepareArgs
    : prepareCmd === "prepare:psql"
      ? ["-f", "prepare.sql"]
      : null;
  if (!prepareArgs) {
    throw new Error(`Scenario ${scenarioId} must define prepareArgs for ${prepareCmd}`);
  }
  return { prepareCmd, prepareArgs };
}

function buildPrepareCommand({ sqlrsPath, workspaceDir, timeout, scenario }) {
  const { prepareCmd, prepareArgs } = resolvePrepareCmd(scenario, scenario.id || "unknown");
  const image = typeof scenario.image === "string" && scenario.image.trim() !== "" ? scenario.image : "postgres:17";
  return [
    sqlrsPath,
    "--timeout",
    timeout,
    "--workspace",
    workspaceDir,
    prepareCmd,
    "--image",
    image,
    "--",
    ...prepareArgs
  ];
}

function buildStatusCommand({ sqlrsPath, workspaceDir }) {
  return [sqlrsPath, "--output", "json", "--workspace", workspaceDir, "status", "--cache"];
}

function buildListInstancesCommand({ sqlrsPath, workspaceDir }) {
  return [sqlrsPath, "--output", "json", "--workspace", workspaceDir, "ls", "--instances"];
}

function buildRmCommand({ sqlrsPath, workspaceDir, idPrefix }) {
  return [sqlrsPath, "--workspace", workspaceDir, "rm", idPrefix];
}

function parseInstanceIDs(payload) {
  if (!payload || typeof payload !== "object") {
    throw new Error("ls payload must be an object");
  }
  const instances = Array.isArray(payload.instances) ? payload.instances : null;
  if (!instances) {
    throw new Error("ls payload must include instances array");
  }
  if (instances.length === 0) {
    throw new Error("cache-pressure scenario expects at least one instance");
  }
  const ids = instances.map((entry) => (typeof entry?.instance_id === "string" ? entry.instance_id.trim() : "")).filter(Boolean);
  if (ids.length !== instances.length) {
    throw new Error("ls payload instance entries must include instance_id");
  }
  return ids;
}

function buildConfigSetCommand({ sqlrsPath, workspaceDir, pathName, value }) {
  return [sqlrsPath, "--workspace", workspaceDir, "config", "set", pathName, JSON.stringify(value)];
}

function deriveCachePressureMaxBytes(usageBytes) {
  const usage = Number(usageBytes);
  if (!Number.isFinite(usage) || usage <= 0) {
    throw new Error(`usageBytes must be positive, got ${usageBytes}`);
  }
  return Math.max(Math.ceil(usage * 1.25), Math.trunc(usage) + 1);
}

function validateCachePressureStatus(payload, expectedMaxBytes) {
  if (!payload || typeof payload !== "object") {
    throw new Error("status payload must be an object");
  }
  if (payload.ok !== true) {
    throw new Error("status payload must report ok=true");
  }
  const summary = payload.cacheSummary;
  if (!summary || typeof summary !== "object") {
    throw new Error("status payload must include cacheSummary");
  }
  if (!(Number(summary.usageBytes) > 0)) {
    throw new Error("cacheSummary.usageBytes must be positive");
  }
  if (!(Number(summary.effectiveMaxBytes) > 0)) {
    throw new Error("cacheSummary.effectiveMaxBytes must be positive");
  }
  if (expectedMaxBytes !== undefined && Number(summary.effectiveMaxBytes) !== Number(expectedMaxBytes)) {
    throw new Error(`cacheSummary.effectiveMaxBytes mismatch: expected ${expectedMaxBytes}, got ${summary.effectiveMaxBytes}`);
  }
  const details = payload.cacheDetails;
  if (!details || typeof details !== "object") {
    throw new Error("status payload must include cacheDetails");
  }
  if (Number(details.reserveBytes) !== 0) {
    throw new Error(`cacheDetails.reserveBytes must be 0, got ${details.reserveBytes}`);
  }
  const lastEviction = details.lastEviction;
  if (!lastEviction || typeof lastEviction !== "object") {
    throw new Error("status payload must include cacheDetails.lastEviction");
  }
  if (!(Number(lastEviction.evictedCount) >= 1)) {
    throw new Error("cacheDetails.lastEviction.evictedCount must be >= 1");
  }
  if (!(Number(lastEviction.freedBytes) > 0)) {
    throw new Error("cacheDetails.lastEviction.freedBytes must be positive");
  }
  if (!(Number(lastEviction.usageBytesBefore) > Number(lastEviction.usageBytesAfter))) {
    throw new Error("cacheDetails.lastEviction usage must decrease across eviction");
  }
}

function appendPrepareVariant(workspaceDir) {
  const preparePath = path.join(workspaceDir, "prepare.sql");
  const original = fs.readFileSync(preparePath, "utf8");
  const suffix = original.endsWith("\n") ? "" : "\n";
  fs.writeFileSync(preparePath, `${original}${suffix}SELECT 1;\n`, "utf8");
}

async function runChecked({ cmd, cwd, env, stdoutPath, stderrPath, stage, scenarioId, workspaceDir }) {
  const exitCode = await runCommand({ cmd, cwd, env, stdoutPath, stderrPath });
  if (exitCode !== 0) {
    writeJSON(path.join(path.dirname(stdoutPath), "result.json"), {
      scenario: scenarioId,
      stage,
      exitCode,
      workspaceDir
    });
    throw new Error(`Command failed during ${stage}: ${commandToString(cmd)}`);
  }
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const repoRoot = path.resolve(args["repo-root"] || repoRootDefault);
  const scenariosPath = path.resolve(args.scenarios || path.join(repoRoot, "test", "e2e", "release", "scenarios.json"));
  const scenarioId = args.scenario;
  const sqlrsPath = path.resolve(args.sqlrs || "");
  const enginePath = path.resolve(args.engine || "");
  const outDir = path.resolve(args["out-dir"] || path.join(repoRoot, "artifacts", "e2e", scenarioId || "unknown"));
  const runTimeout = typeof args.timeout === "string" && args.timeout.trim() !== "" ? args.timeout.trim() : "25m";
  const snapshotBackend = resolveSnapshotBackend(args["snapshot-backend"]);
  const containerRuntime = resolveContainerRuntime(args["container-runtime"]);

  if (!scenarioId) {
    throw new Error("Missing --scenario");
  }
  if (!fs.existsSync(sqlrsPath)) {
    throw new Error(`sqlrs binary not found: ${sqlrsPath}`);
  }
  if (!fs.existsSync(enginePath)) {
    throw new Error(`sqlrs-engine binary not found: ${enginePath}`);
  }

  const scenario = loadScenario(scenariosPath, scenarioId);
  const exampleDir = path.join(repoRoot, "examples", scenario.example);
  if (!fs.existsSync(exampleDir)) {
    throw new Error(`Example directory not found: ${exampleDir}`);
  }

  const queryTemplatePath = path.join(repoRoot, scenario.queryTemplate);
  if (!fs.existsSync(queryTemplatePath)) {
    throw new Error(`Scenario query template not found: ${queryTemplatePath}`);
  }

  rmrf(outDir);
  ensureDir(outDir);

  const workspaceDir = path.join(outDir, "workspace");
  ensureDir(workspaceDir);
  fs.cpSync(exampleDir, workspaceDir, { recursive: true });
  const workspaceMarkerDir = path.join(workspaceDir, ".sqlrs");
  if (fs.existsSync(workspaceMarkerDir)) {
    fs.rmSync(workspaceMarkerDir, { recursive: true, force: true });
  }

  const storePlan = resolveStorePlan(snapshotBackend, outDir);
  if (storePlan.mountType !== "plain-dir") {
    throw new Error("cache-pressure scenario currently supports plain-dir store only");
  }
  ensureDir(storePlan.storeDir);

  fs.copyFileSync(queryTemplatePath, path.join(workspaceDir, ".e2e-query.sql"));

  const initCmd = buildInitCommand({
    sqlrsPath,
    workspaceDir,
    enginePath,
    storeDir: storePlan.storeDir,
    snapshotBackend
  });
  const runtimeConfigCmd = buildRuntimeConfigCommand({
    sqlrsPath,
    workspaceDir,
    containerRuntime
  });
  const prepareCmd = buildPrepareCommand({
    sqlrsPath,
    workspaceDir,
    timeout: runTimeout,
    scenario: { ...scenario, id: scenarioId }
  });
  const statusCmd = buildStatusCommand({ sqlrsPath, workspaceDir });
  const listInstancesCmd = buildListInstancesCommand({ sqlrsPath, workspaceDir });

  fs.writeFileSync(path.join(outDir, "command-init.txt"), `${commandToString(initCmd)}\n`, "utf8");
  if (runtimeConfigCmd) {
    fs.writeFileSync(path.join(outDir, "command-config-runtime.txt"), `${commandToString(runtimeConfigCmd)}\n`, "utf8");
  }
  fs.writeFileSync(path.join(outDir, "command-prepare.txt"), `${commandToString(prepareCmd)}\n`, "utf8");
  fs.writeFileSync(path.join(outDir, "command-status.txt"), `${commandToString(statusCmd)}\n`, "utf8");
  fs.writeFileSync(path.join(outDir, "command-ls-instances.txt"), `${commandToString(listInstancesCmd)}\n`, "utf8");
  writeJSON(path.join(outDir, "scenario.json"), scenario);

  const baseEnv = { ...process.env };

  await runChecked({
    cmd: initCmd,
    cwd: workspaceDir,
    env: baseEnv,
    stdoutPath: path.join(outDir, "init-stdout.log"),
    stderrPath: path.join(outDir, "init-stderr.log"),
    stage: "init",
    scenarioId,
    workspaceDir
  });

  if (runtimeConfigCmd) {
    await runChecked({
      cmd: runtimeConfigCmd,
      cwd: workspaceDir,
      env: baseEnv,
      stdoutPath: path.join(outDir, "config-runtime-stdout.log"),
      stderrPath: path.join(outDir, "config-runtime-stderr.log"),
      stage: "config-runtime",
      scenarioId,
      workspaceDir
    });
  }

  await runChecked({
    cmd: prepareCmd,
    cwd: workspaceDir,
    env: baseEnv,
    stdoutPath: path.join(outDir, "raw-stdout.log"),
    stderrPath: path.join(outDir, "raw-stderr.log"),
    stage: "prepare-first",
    scenarioId,
    workspaceDir
  });

  await runChecked({
    cmd: listInstancesCmd,
    cwd: workspaceDir,
    env: baseEnv,
    stdoutPath: path.join(outDir, "instances-after-prepare.json"),
    stderrPath: path.join(outDir, "instances-after-prepare.stderr.log"),
    stage: "list-instances-after-prepare",
    scenarioId,
    workspaceDir
  });

  const instanceIDs = parseInstanceIDs(JSON.parse(fs.readFileSync(path.join(outDir, "instances-after-prepare.json"), "utf8")));
  fs.writeFileSync(
    path.join(outDir, "command-rm-instance.txt"),
    `${instanceIDs.map((instanceID) => commandToString(buildRmCommand({ sqlrsPath, workspaceDir, idPrefix: instanceID }))).join("\n")}\n`,
    "utf8"
  );
  for (let index = 0; index < instanceIDs.length; index += 1) {
    const instanceID = instanceIDs[index];
    const rmInstanceCmd = buildRmCommand({ sqlrsPath, workspaceDir, idPrefix: instanceID });
    await runChecked({
      cmd: rmInstanceCmd,
      cwd: workspaceDir,
      env: baseEnv,
      stdoutPath: path.join(outDir, `rm-instance-${index + 1}.stdout.log`),
      stderrPath: path.join(outDir, `rm-instance-${index + 1}.stderr.log`),
      stage: `rm-instance-after-prepare-${index + 1}`,
      scenarioId,
      workspaceDir
    });
  }

  await runChecked({
    cmd: statusCmd,
    cwd: workspaceDir,
    env: baseEnv,
    stdoutPath: path.join(outDir, "status-before.json"),
    stderrPath: path.join(outDir, "status-before.stderr.log"),
    stage: "status-before",
    scenarioId,
    workspaceDir
  });

  const beforeStatus = JSON.parse(fs.readFileSync(path.join(outDir, "status-before.json"), "utf8"));
  const usageBytes = Number(beforeStatus?.cacheSummary?.usageBytes ?? 0);
  if (!(usageBytes > 0)) {
    throw new Error(`cache-pressure scenario requires positive usageBytes after first flow, got ${usageBytes}`);
  }
  const maxBytes = deriveCachePressureMaxBytes(usageBytes);
  const configCommands = [
    buildConfigSetCommand({ sqlrsPath, workspaceDir, pathName: "cache.capacity.reserveBytes", value: 0 }),
    buildConfigSetCommand({ sqlrsPath, workspaceDir, pathName: "cache.capacity.minStateAge", value: "0s" }),
    buildConfigSetCommand({ sqlrsPath, workspaceDir, pathName: "cache.capacity.maxBytes", value: maxBytes })
  ];
  fs.writeFileSync(
    path.join(outDir, "command-config-cache-capacity.txt"),
    `${configCommands.map((cmd) => commandToString(cmd)).join("\n")}\n`,
    "utf8"
  );

  for (let index = 0; index < configCommands.length; index += 1) {
    const command = configCommands[index];
    await runChecked({
      cmd: command,
      cwd: workspaceDir,
      env: baseEnv,
      stdoutPath: path.join(outDir, `config-cache-${index + 1}.stdout.log`),
      stderrPath: path.join(outDir, `config-cache-${index + 1}.stderr.log`),
      stage: `config-cache-${index + 1}`,
      scenarioId,
      workspaceDir
    });
  }

  appendPrepareVariant(workspaceDir);
  fs.writeFileSync(path.join(outDir, "command-prepare-run2.txt"), `${commandToString(prepareCmd)}\n`, "utf8");

  await runChecked({
    cmd: prepareCmd,
    cwd: workspaceDir,
    env: baseEnv,
    stdoutPath: path.join(outDir, "raw-stdout-run2.log"),
    stderrPath: path.join(outDir, "raw-stderr-run2.log"),
    stage: "prepare-second",
    scenarioId,
    workspaceDir
  });

  await runChecked({
    cmd: statusCmd,
    cwd: workspaceDir,
    env: baseEnv,
    stdoutPath: path.join(outDir, "status-cache.json"),
    stderrPath: path.join(outDir, "status-cache.stderr.log"),
    stage: "status-after",
    scenarioId,
    workspaceDir
  });

  const finalStatus = JSON.parse(fs.readFileSync(path.join(outDir, "status-cache.json"), "utf8"));
  validateCachePressureStatus(finalStatus, maxBytes);
  writeJSON(path.join(outDir, "cache-pressure-summary.json"), {
    scenario: scenarioId,
    usageBytesBefore: usageBytes,
    configuredMaxBytes: maxBytes,
    finalStateCount: finalStatus?.cacheSummary?.stateCount ?? null,
    lastEviction: finalStatus?.cacheDetails?.lastEviction ?? null
  });
  writeJSON(path.join(outDir, "result.json"), {
    scenario: scenarioId,
    stage: "status-after",
    exitCode: 0,
    workspaceDir,
    configuredMaxBytes: maxBytes
  });
}

if (path.resolve(process.argv[1] || "") === __filename) {
  main().catch((err) => {
    console.error(err?.stack || String(err));
    process.exit(1);
  });
}

export { buildConfigSetCommand, buildListInstancesCommand, buildPrepareCommand, buildRmCommand, deriveCachePressureMaxBytes, parseInstanceIDs, validateCachePressureStatus };
