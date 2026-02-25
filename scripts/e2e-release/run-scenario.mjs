import fs from "node:fs";
import path from "node:path";
import { spawn, spawnSync } from "node:child_process";
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

function runCheckedSync({ cmd, cwd, env }) {
  const result = spawnSync(cmd[0], cmd.slice(1), {
    cwd,
    env,
    encoding: "utf8"
  });
  if (result.status !== 0) {
    const stderr = typeof result.stderr === "string" ? result.stderr.trim() : "";
    const stdout = typeof result.stdout === "string" ? result.stdout.trim() : "";
    const detail = stderr || stdout || `exit=${result.status}`;
    throw new Error(`Command failed: ${commandToString(cmd)} (${detail})`);
  }
  return result;
}

function readCommandOutput({ cmd, cwd, env }) {
  const result = spawnSync(cmd[0], cmd.slice(1), {
    cwd,
    env,
    encoding: "utf8"
  });
  if (result.status !== 0) {
    const stderr = typeof result.stderr === "string" ? result.stderr.trim() : "";
    const stdout = typeof result.stdout === "string" ? result.stdout.trim() : "";
    const detail = stderr || stdout || `exit=${result.status}`;
    throw new Error(`Command failed: ${commandToString(cmd)} (${detail})`);
  }
  return typeof result.stdout === "string" ? result.stdout.trim() : "";
}

function commandExists(name) {
  const result = spawnSync("which", [name], { encoding: "utf8" });
  return result.status === 0;
}

function detectLinuxFSType(targetPath) {
  return readCommandOutput({
    cmd: ["stat", "-f", "-c", "%T", targetPath]
  }).toLowerCase();
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

export function resolveFlowRuns(raw) {
  const value = typeof raw === "string" ? raw.trim() : "";
  if (value === "") {
    return 1;
  }
  const parsed = Number.parseInt(value, 10);
  if (!Number.isInteger(parsed) || parsed < 1) {
    throw new Error(`Invalid --flow-runs: ${value}`);
  }
  return parsed;
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

export function resolveStorePlan(snapshotBackend, outDir) {
  if (snapshotBackend === "btrfs") {
    const mountDir = path.join(outDir, "btrfs-store");
    return {
      mountType: "btrfs-loop",
      mountDir,
      imagePath: path.join(outDir, "btrfs-store.img"),
      storeDir: path.join(mountDir, "store")
    };
  }
  return {
    mountType: "plain-dir",
    storeDir: path.join(outDir, "store")
  };
}

function setupStorePlan(plan) {
  if (plan.mountType !== "btrfs-loop") {
    ensureDir(plan.storeDir);
    return () => {};
  }
  if (process.platform !== "linux") {
    throw new Error("snapshot backend btrfs requires Linux runner");
  }
  if (!commandExists("mkfs.btrfs")) {
    throw new Error("mkfs.btrfs is not available (install btrfs-progs)");
  }
  let mounted = false;
  try {
    ensureDir(plan.mountDir);
    runCheckedSync({ cmd: ["truncate", "-s", "8G", plan.imagePath] });
    runCheckedSync({ cmd: ["sudo", "mkfs.btrfs", "-f", plan.imagePath] });
    runCheckedSync({ cmd: ["sudo", "mount", "-o", "loop", plan.imagePath, plan.mountDir] });
    mounted = true;
    if (typeof process.getuid === "function" && typeof process.getgid === "function") {
      runCheckedSync({
        cmd: ["sudo", "chown", "-R", `${process.getuid()}:${process.getgid()}`, plan.mountDir]
      });
    }
    ensureDir(plan.storeDir);
    const fsType = detectLinuxFSType(plan.storeDir);
    if (fsType !== "btrfs") {
      throw new Error(`expected btrfs store filesystem, got ${fsType}`);
    }
  } catch (err) {
    if (mounted) {
      spawnSync("sudo", ["umount", "-l", plan.mountDir], { encoding: "utf8" });
    }
    throw err;
  }
  return () => {
    const unmount = spawnSync("sudo", ["umount", plan.mountDir], { encoding: "utf8" });
    if (unmount.status !== 0) {
      spawnSync("sudo", ["umount", "-l", plan.mountDir], { encoding: "utf8" });
    }
  };
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
  const flowRuns = resolveFlowRuns(args["flow-runs"]);

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
  const storePlan = resolveStorePlan(snapshotBackend, outDir);
  let cleanupStore = () => {};
  cleanupStore = setupStorePlan(storePlan);

  const queryPath = path.join(workspaceDir, ".e2e-query.sql");
  fs.copyFileSync(queryTemplatePath, queryPath);

  const initCmd = buildInitCommand({
    sqlrsPath,
    workspaceDir,
    enginePath,
    storeDir: storePlan.storeDir,
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

  try {
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

    for (let run = 1; run <= flowRuns; run += 1) {
      const suffix = run === 1 ? "" : `-run${run}`;
      const flowExit = await runCommand({
        cmd: flowCmd,
        cwd: workspaceDir,
        env: baseEnv,
        stdoutPath: path.join(outDir, `raw-stdout${suffix}.log`),
        stderrPath: path.join(outDir, `raw-stderr${suffix}.log`)
      });

      writeJSON(path.join(outDir, "result.json"), {
        scenario: scenarioId,
        stage: "prepare+run",
        flowRun: run,
        flowRuns,
        exitCode: flowExit,
        workspaceDir
      });

      if (flowExit !== 0) {
        throw new Error(`Scenario failed on flow run ${run}/${flowRuns}: ${scenarioId}`);
      }
    }
  } finally {
    cleanupStore();
  }
}

if (path.resolve(process.argv[1] || "") === __filename) {
  main().catch((err) => {
    console.error(err?.stack || String(err));
    process.exit(1);
  });
}
