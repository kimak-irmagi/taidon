import fs from "node:fs";
import path from "node:path";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRootDefault = path.resolve(__dirname, "..", "..");
const allowedSnapshotBackends = new Set(["auto", "btrfs", "overlay", "copy"]);

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

export function resolveSnapshotBackend(raw) {
  const value = typeof raw === "string" ? raw.trim() : "";
  if (value === "") {
    return "copy";
  }
  if (!allowedSnapshotBackends.has(value)) {
    throw new Error(`Invalid --snapshot-backend: ${value}`);
  }
  return value;
}

export function buildInitCommand({ sqlrsPath, workspaceDir, enginePath, storeDir, snapshotBackend }) {
  return [
    sqlrsPath,
    "--workspace",
    workspaceDir,
    "init",
    "local",
    "--engine",
    enginePath,
    "--snapshot",
    snapshotBackend,
    "--store",
    "dir",
    storeDir,
    "--no-start"
  ];
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const repoRoot = path.resolve(args["repo-root"] || repoRootDefault);
  const scenariosPath = path.resolve(args.scenarios || path.join(repoRoot, "test", "e2e", "release", "scenarios.json"));
  const scenarioId = args.scenario;
  const sqlrsPath = path.resolve(args.sqlrs || "");
  const enginePath = path.resolve(args.engine || "");
  const outDir = path.resolve(args["out-dir"] || path.join(repoRoot, "artifacts", "e2e", scenarioId || "unknown"));
  const runTimeout = typeof args.timeout === "string" && args.timeout.trim() !== "" ? args.timeout.trim() : "15m";
  const snapshotBackend = resolveSnapshotBackend(args["snapshot-backend"]);

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
  const storeDir = path.join(outDir, "store");
  ensureDir(workspaceDir);
  ensureDir(storeDir);
  fs.cpSync(exampleDir, workspaceDir, { recursive: true });

  const queryPath = path.join(workspaceDir, ".e2e-query.sql");
  fs.copyFileSync(queryTemplatePath, queryPath);

  const initCmd = buildInitCommand({
    sqlrsPath,
    workspaceDir,
    enginePath,
    storeDir,
    snapshotBackend
  });

  const prepareArgs = Array.isArray(scenario.prepareArgs) ? scenario.prepareArgs : ["-f", "prepare.sql"];
  const runArgs = Array.isArray(scenario.runArgs) ? scenario.runArgs : ["-At", "-f", ".e2e-query.sql"];
  const image = typeof scenario.image === "string" && scenario.image.trim() !== "" ? scenario.image : "postgres:17";

  const flowCmd = [
    sqlrsPath,
    "--timeout",
    runTimeout,
    "--workspace",
    workspaceDir,
    "prepare:psql",
    "--image",
    image,
    "--",
    ...prepareArgs,
    "run:psql",
    "--",
    ...runArgs
  ];

  fs.writeFileSync(path.join(outDir, "command-init.txt"), `${commandToString(initCmd)}\n`, "utf8");
  fs.writeFileSync(path.join(outDir, "command-flow.txt"), `${commandToString(flowCmd)}\n`, "utf8");
  writeJSON(path.join(outDir, "scenario.json"), scenario);

  const baseEnv = { ...process.env };
  const initExit = await runCommand({
    cmd: initCmd,
    cwd: workspaceDir,
    env: baseEnv,
    stdoutPath: path.join(outDir, "init-stdout.log"),
    stderrPath: path.join(outDir, "init-stderr.log")
  });
  if (initExit !== 0) {
    writeJSON(path.join(outDir, "result.json"), {
      scenario: scenarioId,
      stage: "init",
      exitCode: initExit,
      workspaceDir
    });
    throw new Error(`Init command failed for ${scenarioId}`);
  }

  const flowExit = await runCommand({
    cmd: flowCmd,
    cwd: workspaceDir,
    env: baseEnv,
    stdoutPath: path.join(outDir, "raw-stdout.log"),
    stderrPath: path.join(outDir, "raw-stderr.log")
  });

  writeJSON(path.join(outDir, "result.json"), {
    scenario: scenarioId,
    stage: "prepare+run",
    exitCode: flowExit,
    workspaceDir
  });

  if (flowExit !== 0) {
    throw new Error(`Scenario failed: ${scenarioId}`);
  }
}

if (path.resolve(process.argv[1] || "") === __filename) {
  main().catch((err) => {
    console.error(err?.stack || String(err));
    process.exit(1);
  });
}
