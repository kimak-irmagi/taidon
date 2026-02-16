import fs from "node:fs";
import path from "node:path";
import { spawn } from "node:child_process";

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

function resolveStatePaths(baseDir) {
  if (process.platform === "win32") {
    return {
      appData: path.join(baseDir, "appdata"),
      localAppData: path.join(baseDir, "localappdata"),
      stateDir: path.join(baseDir, "localappdata", "sqlrs")
    };
  }
  if (process.platform === "darwin") {
    return {
      home: path.join(baseDir, "home"),
      stateDir: path.join(baseDir, "home", "Library", "Application Support", "sqlrs", "state")
    };
  }
  return {
    xdgConfigHome: path.join(baseDir, "xdg-config"),
    xdgStateHome: path.join(baseDir, "xdg-state"),
    xdgCacheHome: path.join(baseDir, "xdg-cache"),
    stateDir: path.join(baseDir, "xdg-state", "sqlrs")
  };
}

function buildEnv(baseEnv, resolved) {
  const env = { ...baseEnv };
  if (process.platform === "win32") {
    env.APPDATA = resolved.appData;
    env.LOCALAPPDATA = resolved.localAppData;
  } else if (process.platform === "darwin") {
    env.HOME = resolved.home;
  } else {
    env.XDG_CONFIG_HOME = resolved.xdgConfigHome;
    env.XDG_STATE_HOME = resolved.xdgStateHome;
    env.XDG_CACHE_HOME = resolved.xdgCacheHome;
  }
  return env;
}

function cleanupEngine(statePath) {
  if (!fs.existsSync(statePath)) {
    return;
  }
  try {
    const raw = fs.readFileSync(statePath, "utf8");
    const parsed = JSON.parse(raw);
    const pid = Number(parsed.pid || 0);
    if (pid <= 0) {
      return;
    }
    if (process.platform === "win32") {
      spawn("taskkill", ["/PID", String(pid), "/T", "/F"], { stdio: "ignore" });
      return;
    }
    process.kill(pid, "SIGTERM");
  } catch {
    // Cleanup is best-effort only.
  }
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const sqlrsPath = path.resolve(args.sqlrs || "");
  const enginePath = path.resolve(args.engine || "");
  const outDir = path.resolve(args["out-dir"] || "");

  if (!fs.existsSync(sqlrsPath)) {
    throw new Error(`sqlrs binary not found: ${sqlrsPath}`);
  }
  if (!fs.existsSync(enginePath)) {
    throw new Error(`sqlrs-engine binary not found: ${enginePath}`);
  }
  if (!outDir) {
    throw new Error("Missing --out-dir");
  }

  ensureDir(outDir);
  const stateRoots = resolveStatePaths(outDir);
  for (const value of Object.values(stateRoots)) {
    ensureDir(value);
  }

  const env = buildEnv(process.env, stateRoots);
  const workspaceDir = path.join(outDir, "workspace");
  const storeDir = path.join(outDir, "workspace-store");
  ensureDir(workspaceDir);
  ensureDir(storeDir);

  const helpCmd = [sqlrsPath, "--help"];
  const initCmd = [
    sqlrsPath,
    "--workspace",
    workspaceDir,
    "init",
    "local",
    "--engine",
    enginePath,
    "--snapshot",
    "copy",
    "--store",
    "dir",
    storeDir,
    "--no-start"
  ];
  const statusCmd = [sqlrsPath, "--workspace", workspaceDir, "status"];

  fs.writeFileSync(path.join(outDir, "command-help.txt"), `${commandToString(helpCmd)}\n`, "utf8");
  fs.writeFileSync(path.join(outDir, "command-init.txt"), `${commandToString(initCmd)}\n`, "utf8");
  fs.writeFileSync(path.join(outDir, "command-status.txt"), `${commandToString(statusCmd)}\n`, "utf8");

  const helpExit = await runCommand({
    cmd: helpCmd,
    cwd: workspaceDir,
    env,
    stdoutPath: path.join(outDir, "help-stdout.log"),
    stderrPath: path.join(outDir, "help-stderr.log")
  });
  if (helpExit !== 0) {
    throw new Error("sqlrs --help failed");
  }

  const initExit = await runCommand({
    cmd: initCmd,
    cwd: workspaceDir,
    env,
    stdoutPath: path.join(outDir, "init-stdout.log"),
    stderrPath: path.join(outDir, "init-stderr.log")
  });
  if (initExit !== 0) {
    throw new Error("sqlrs init local failed");
  }

  const statusExit = await runCommand({
    cmd: statusCmd,
    cwd: workspaceDir,
    env,
    stdoutPath: path.join(outDir, "status-stdout.log"),
    stderrPath: path.join(outDir, "status-stderr.log")
  });
  if (statusExit !== 0) {
    throw new Error("sqlrs status failed");
  }

  const statusOutput = fs.readFileSync(path.join(outDir, "status-stdout.log"), "utf8");
  if (!statusOutput.includes("status: ok")) {
    throw new Error("status output does not report healthy engine");
  }

  const engineStatePath = path.join(stateRoots.stateDir, "engine.json");
  const engineLogPath = path.join(stateRoots.stateDir, "logs", process.platform === "win32" ? "engine-wsl.log" : "engine.log");
  if (fs.existsSync(engineStatePath)) {
    fs.copyFileSync(engineStatePath, path.join(outDir, "engine.json"));
  }
  if (fs.existsSync(engineLogPath)) {
    fs.copyFileSync(engineLogPath, path.join(outDir, "engine.log"));
  }

  cleanupEngine(engineStatePath);
}

main().catch((err) => {
  console.error(err?.stack || String(err));
  process.exit(1);
});
