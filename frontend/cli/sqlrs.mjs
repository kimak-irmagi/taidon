#!/usr/bin/env node
import path from "node:path";
import fs from "node:fs";
import crypto from "node:crypto";
import { spawn } from "node:child_process";

import { ensureDir, writeJson, nowMs } from "./sqlrs.d/lib/fs.mjs";
import { run, runCapture } from "./sqlrs.d/lib/proc.mjs";
import { docker } from "./sqlrs.d/lib/docker.mjs";
import { loadBackend } from "./sqlrs.d/lib/backend.mjs";

function randomId() {
  return `${Date.now()}-${crypto.randomBytes(4).toString("hex")}`;
}

function spawnDetached(cmd, args) {
  try {
    const p = spawn(cmd, args, { detached: true, stdio: "ignore" });
    p.unref();
    return true;
  } catch {
    return false;
  }
}

async function isDockerReady() {
  try {
    await runCapture({ cmd: ["docker", "info"] });
    return true;
  } catch {
    return false;
  }
}

function findDockerDesktopExe() {
  const candidates = [];
  const programFiles = process.env.ProgramFiles;
  if (programFiles) {
    candidates.push(path.join(programFiles, "Docker", "Docker", "Docker Desktop.exe"));
  }
  const programFilesX86 = process.env["ProgramFiles(x86)"];
  if (programFilesX86) {
    candidates.push(path.join(programFilesX86, "Docker", "Docker", "Docker Desktop.exe"));
  }
  const localAppData = process.env.LOCALAPPDATA;
  if (localAppData) {
    candidates.push(path.join(localAppData, "Docker", "Docker", "Docker Desktop.exe"));
  }
  return candidates.find((p) => fs.existsSync(p)) || null;
}

function startDockerDesktopWindows() {
  const exe = findDockerDesktopExe();
  if (!exe) return false;
  return spawnDetached(exe, []);
}

function startDockerDesktopMac() {
  return spawnDetached("open", ["-a", "Docker"]);
}

async function startDockerLinux() {
  try {
    await run({ cmd: ["systemctl", "start", "docker"] });
    return true;
  } catch {}
  try {
    await run({ cmd: ["service", "docker", "start"] });
    return true;
  } catch {}
  return false;
}

async function tryStartDocker() {
  if (process.platform === "win32") return startDockerDesktopWindows();
  if (process.platform === "darwin") return startDockerDesktopMac();
  if (process.platform === "linux") return await startDockerLinux();
  return false;
}

async function waitForDockerReady(timeoutMs) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (await isDockerReady()) return true;
    await new Promise((r) => setTimeout(r, 1000));
  }
  return false;
}

async function ensureDockerReady() {
  if (await isDockerReady()) return;
  const started = await tryStartDocker();
  if (started) {
    const ok = await waitForDockerReady(60000);
    if (ok) return;
  }
  throw new Error("Docker is not running. Start Docker Desktop and retry.");
}

function copyDir(src, dst) {
  ensureDir(dst);
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    const s = path.join(src, entry.name);
    const d = path.join(dst, entry.name);
    if (entry.isDirectory()) copyDir(s, d);
    else if (entry.isFile()) fs.copyFileSync(s, d);
  }
}

function looksLikeConnArg(s) {
  return typeof s === "string" && (s.startsWith("postgres://") || s.startsWith("postgresql://"));
}

function hasAnyFlag(args, flags) {
  return args.some((a) => flags.includes(a));
}

function hasConnSpec(args) {
  const connFlags = ["-h", "--host", "-p", "--port", "-U", "--username", "-d", "--dbname"];
  return args.some((a) => connFlags.includes(a)) || args.some(looksLikeConnArg);
}

function cmdName(cmd) {
  if (!cmd || cmd.length === 0) return "";
  const base = path.basename(cmd[0]).toLowerCase();
  return base.endsWith(".exe") ? base.slice(0, -4) : base;
}

function isPsqlCmd(cmd) {
  return cmdName(cmd) === "psql";
}

const knownCommandsWithFileArgs = new Set(["psql", "pgbench"]);

function hasFileArg(cmd) {
  return knownCommandsWithFileArgs.has(cmdName(cmd));
}

function assertStrictPsql(cmd) {
  if (!cmd || cmd.length === 0) return;
  if (!isPsqlCmd(cmd)) return;

  const rest = cmd.slice(1);

  // Catch Cyrillic "-с" (U+0441) which looks like "-c"
  const hasCyrillic = rest.includes("-с"); // note: кириллическая "с"
  if (hasCyrillic) {
    throw new Error(
      'psql: найден флаг "-с" (кириллица). Нужен "-c" (латиница). Пример: psql -c "select 1"'
    );
  }

  const hasC = hasAnyFlag(rest, ["-c"]);
  const hasF = hasAnyFlag(rest, ["-f"]);

  if (!hasC && !hasF) {
    throw new Error('psql: требуется явно указать "-c" или "-f". Пример: psql -c "select 1"');
  }
}

function injectDbUrlIntoPsql(cmd, databaseUrl) {
  if (!cmd || cmd.length === 0) return cmd;
  if (!isPsqlCmd(cmd)) return cmd;

  const rest = cmd.slice(1);
  if (hasConnSpec(rest)) return cmd; // user provided connection

  return ["psql", databaseUrl, ...rest];
}

function toPosix(p) {
  return p.split(path.sep).join(path.posix.sep);
}

function extractFileArgs(cmd) {
  const files = [];
  for (let i = 0; i < cmd.length; i++) {
    const arg = cmd[i];
    if (arg === "-f" || arg === "--file") {
      const val = cmd[i + 1];
      if (!val) throw new Error("Missing file path after -f/--file");
      files.push(val);
      i++;
      continue;
    }
    if (arg.startsWith("-f") && arg.length > 2) {
      files.push(arg.slice(2));
      continue;
    }
    if (arg.startsWith("--file=")) {
      files.push(arg.slice("--file=".length));
    }
  }
  return files;
}

function parseIncludePath(raw) {
  if (!raw) return null;
  let s = raw.trim();
  if (s.endsWith(";")) s = s.slice(0, -1).trim();
  if ((s.startsWith("'") && s.endsWith("'")) || (s.startsWith("\"") && s.endsWith("\""))) {
    s = s.slice(1, -1);
  }
  if (s.startsWith(":")) return null;
  return s;
}

function collectSqlFiles(entryPath, seen, out) {
  const full = path.resolve(entryPath);
  if (seen.has(full)) return;
  if (!fs.existsSync(full)) throw new Error(`Prepare file not found: ${full}`);
  seen.add(full);
  out.push(full);

  const text = fs.readFileSync(full, "utf8");
  const dir = path.dirname(full);
  const lines = text.split(/\r?\n/);
  for (const line of lines) {
    const m = line.match(/^\s*\\(i|ir|include)\s+(.+?)\s*$/i);
    if (!m) continue;
    const ref = parseIncludePath(m[2]);
    if (!ref) continue;
    const next = path.isAbsolute(ref) ? ref : path.join(dir, ref);
    collectSqlFiles(next, seen, out);
  }
}

function computePrepareState(cmd) {
  const hash = crypto.createHash("md5");
  hash.update(JSON.stringify(cmd));

  let files = [];
  if (hasFileArg(cmd)) {
    const fileArgs = extractFileArgs(cmd);
    for (const f of fileArgs) {
      if (f === "-") continue;
      if (isPsqlCmd(cmd)) {
        const seen = new Set();
        const out = [];
        collectSqlFiles(f, seen, out);
        files = files.concat(out);
      } else {
        const full = path.resolve(f);
        if (!fs.existsSync(full)) throw new Error(`File not found: ${full}`);
        files.push(full);
      }
    }
  }

  const uniq = Array.from(new Set(files)).sort();
  for (const f of uniq) {
    hash.update("\0");
    hash.update(f);
    hash.update("\0");
    hash.update(fs.readFileSync(f));
  }

  return { stateId: hash.digest("hex"), files: uniq };
}

function commonAncestor(paths) {
  if (!paths || paths.length === 0) return null;
  const parts = paths.map((p) => path.resolve(p).split(path.sep));
  const first = parts[0];
  let end = first.length;
  for (let i = 0; i < end; i++) {
    const seg = first[i];
    for (let j = 1; j < parts.length; j++) {
      if (parts[j][i] !== seg) {
        end = i;
        i = end;
        break;
      }
    }
  }
  if (end === 0) return null;
  return first.slice(0, end).join(path.sep);
}

function collectFilesFromCmd(cmd) {
  if (!cmd || cmd.length === 0) return [];
  if (!hasFileArg(cmd)) return [];

  const files = [];
  const fileArgs = extractFileArgs(cmd);
  for (const f of fileArgs) {
    if (f === "-") continue;
    if (isPsqlCmd(cmd)) {
      const seen = new Set();
      const out = [];
      collectSqlFiles(f, seen, out);
      files.push(...out);
    } else {
      const full = path.resolve(f);
      if (!fs.existsSync(full)) throw new Error(`File not found: ${full}`);
      files.push(full);
    }
  }
  return Array.from(new Set(files)).sort();
}

async function stagePsqlFilesToContainer(container, files) {
  if (!files || files.length === 0) return { root: null, containerRoot: null };
  const root = commonAncestor(files) || path.dirname(files[0]);
  const containerRoot = `/tmp/sqlrs_mount_${crypto.randomBytes(4).toString("hex")}`;
  const created = new Set();

  for (const file of files) {
    const rel = toPosix(path.relative(root, file));
    const containerPath = path.posix.join(containerRoot, rel);
    const containerDir = path.posix.dirname(containerPath);
    if (!created.has(containerDir)) {
      await run({ cmd: ["docker", "exec", container, "mkdir", "-p", containerDir] });
      created.add(containerDir);
    }
    await docker.cpToContainer(file, container, containerPath);
  }

  return { root, containerRoot };
}

function rewritePsqlFileArgsForContainer(cmd, root, containerRoot) {
  if (!root || !containerRoot) return { cmd, workdir: null };
  const nextCmd = cmd.slice();
  let workdir = null;

  for (let i = 0; i < nextCmd.length; i++) {
    const flag = nextCmd[i];
    if (flag === "-f" || flag === "--file") {
      const rawPath = nextCmd[i + 1];
      if (!rawPath) throw new Error("Missing file path after -f/--file");
      const hostPath = path.resolve(rawPath);
      const rel = toPosix(path.relative(root, hostPath));
      const containerPath = path.posix.join(containerRoot, rel);
      nextCmd[i + 1] = containerPath;
      if (!workdir) workdir = path.posix.dirname(containerPath);
      i++;
      continue;
    }
    if (flag.startsWith("-f") && flag.length > 2) {
      const rawPath = flag.slice(2);
      const hostPath = path.resolve(rawPath);
      const rel = toPosix(path.relative(root, hostPath));
      const containerPath = path.posix.join(containerRoot, rel);
      nextCmd[i] = `-f${containerPath}`;
      if (!workdir) workdir = path.posix.dirname(containerPath);
      continue;
    }
    if (flag.startsWith("--file=")) {
      const rawPath = flag.slice("--file=".length);
      const hostPath = path.resolve(rawPath);
      const rel = toPosix(path.relative(root, hostPath));
      const containerPath = path.posix.join(containerRoot, rel);
      nextCmd[i] = `--file=${containerPath}`;
      if (!workdir) workdir = path.posix.dirname(containerPath);
    }
  }

  return { cmd: nextCmd, workdir };
}

function parseFrom(s) {
  // expects "postgres:17" for now
  if (!s) throw new Error("Missing --from");
  return { image: s };
}

function printHelp() {
  console.log(`Usage:
  sqlrs --from <image> [options] --prepare -- <cmd...> [--run -- <cmd...>]
  sqlrs --from <image> [options] --run -- <cmd...>

Options:
  --from, -f       Container image (e.g. postgres:17) [required]
  --workspace      Workspace root (default: ./sqlrs-work)
  --storage        plain|btrfs|zfs (default: plain)
  --client         container|host (default: container)
  --pgUser         Postgres user (default: postgres)
  --pgPassword     Postgres password (default: postgres)
  --pgDb           Postgres database (default: postgres)
  --help           Show this help
`);
}

function parseGlobalArgs(argv) {
  const opts = {
    from: null,
    workspace: path.resolve(process.cwd(), "sqlrs-work"),
    storage: "plain",
    client: "container",
    pgUser: "postgres",
    pgPassword: "postgres",
    pgDb: "postgres"
  };

  const withValue = new Map([
    ["--from", "from"],
    ["-f", "from"],
    ["--workspace", "workspace"],
    ["--storage", "storage"],
    ["--client", "client"],
    ["--pgUser", "pgUser"],
    ["--pgPassword", "pgPassword"],
    ["--pgDb", "pgDb"]
  ]);

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === "--help") {
      printHelp();
      process.exit(0);
    }

    if (arg.startsWith("--") && arg.includes("=")) {
      const idx = arg.indexOf("=");
      const flag = arg.slice(0, idx);
      const val = arg.slice(idx + 1);
      const key = withValue.get(flag);
      if (!key) throw new Error(`Unknown option: ${flag}`);
      opts[key] = val;
      continue;
    }

    const key = withValue.get(arg);
    if (key) {
      if (i + 1 >= argv.length) throw new Error(`Missing value for ${arg}`);
      opts[key] = argv[i + 1];
      i++;
      continue;
    }

    throw new Error(`Unknown option: ${arg}`);
  }

  if (!opts.from) throw new Error("Missing --from");
  if (!["plain", "btrfs", "zfs"].includes(opts.storage)) {
    throw new Error(`Invalid --storage: ${opts.storage}`);
  }
  if (!["container", "host"].includes(opts.client)) {
    throw new Error(`Invalid --client: ${opts.client}`);
  }

  return opts;
}

function parseStepArgs(argv) {
  const markers = new Set(["--prepare", "--run"]);
  const firstIdx = argv.findIndex((a) => markers.has(a));
  if (firstIdx === -1) {
    throw new Error('Missing "--prepare" or "--run". Example: --run -- psql -c "select 1"');
  }

  const globalArgs = argv.slice(0, firstIdx);
  let prepareCmd = null;
  let runCmd = null;
  let i = firstIdx;

  while (i < argv.length) {
    const marker = argv[i];
    if (!markers.has(marker)) {
      throw new Error(`Unexpected token: ${argv[i]}`);
    }
    if (marker === "--prepare") {
      if (prepareCmd) throw new Error('Only one "--prepare" is allowed.');
      if (runCmd) throw new Error('"--prepare" must come before "--run".');
    } else {
      if (runCmd) throw new Error('Only one "--run" is allowed.');
    }
    if (argv[i + 1] !== "--") {
      throw new Error(`Expected "--" after ${marker}.`);
    }

    let j = i + 2;
    while (j < argv.length && !markers.has(argv[j])) j++;
    const cmd = argv.slice(i + 2, j);
    if (cmd.length === 0) {
      throw new Error(`Missing command after "${marker} --".`);
    }

    if (marker === "--prepare") prepareCmd = cmd;
    else runCmd = cmd;

    i = j;
  }

  return { globalArgs, prepareCmd, runCmd };
}

async function snapshotVolume({ workspace, volume, runId, storage, image, stateId, prepareCmd }) {
  const stateDir = path.join(workspace, "states", stateId);
  if (fs.existsSync(stateDir)) {
    throw new Error(`State already exists: ${stateDir}`);
  }

  const pgdataDir = path.join(stateDir, "pgdata");
  ensureDir(pgdataDir);
  copyDir(volume.path, pgdataDir);

  await writeJson(path.join(stateDir, "state.json"), {
    state_id: stateId,
    run_id: runId,
    created_at: new Date().toISOString(),
    storage,
    image,
    pgdata: "pgdata",
    prepare: prepareCmd ? { cmd: prepareCmd } : null
  });

  return { stateId, stateDir };
}

async function main() {
  const { globalArgs, prepareCmd, runCmd } = parseStepArgs(process.argv.slice(2));
  const args = parseGlobalArgs(globalArgs);

  if (!prepareCmd && !runCmd) {
    throw new Error('Missing "--prepare" or "--run".');
  }

  await ensureDockerReady();

  const from = parseFrom(args.from);
  const runId = randomId();

  const ws = path.resolve(args.workspace);
  const runDir = path.join(ws, "runs", runId);
  const logsDir = path.join(runDir, "logs");
  ensureDir(logsDir);

  const prepareState = prepareCmd ? computePrepareState(prepareCmd) : null;
  const stateDir = prepareState ? path.join(ws, "states", prepareState.stateId) : null;

  const metrics = {
    run_id: runId,
    storage: args.storage,
    image: from.image,
    times_ms: {},
    sizes: {}
  };

  const t0 = nowMs();

  // Load backend and allocate PGDATA
  const backend = await loadBackend(args.storage, path.resolve(path.dirname(new URL(import.meta.url).pathname)));
  const vol = await backend.createVolume({ workspace: ws, runId });
  const pgdataHostPath = vol.path;
  let reusedState = false;
  if (prepareState && stateDir && fs.existsSync(path.join(stateDir, "pgdata"))) {
    console.log(`[state] reuse ${prepareState.stateId}`);
    copyDir(path.join(stateDir, "pgdata"), pgdataHostPath);
    reusedState = true;
    metrics.state = { state_id: prepareState.stateId, path: stateDir, reused: true };
  }

  // Start postgres container
  const pgName = `sqlrs-pg-${runId}`;
  const netName = `sqlrs-net-${runId}`;
  metrics.docker = { container: pgName, network: netName };

  const tStart = nowMs();
  await docker.networkCreate(netName);

  // Expose port on random free host port for convenience (optional)
  // We'll mostly use docker exec / docker run --network anyway.
  await docker.runDetached({
    name: pgName,
    image: from.image,
    network: netName,
    env: {
      POSTGRES_USER: args.pgUser,
      POSTGRES_PASSWORD: args.pgPassword,
      POSTGRES_DB: args.pgDb
    },
    mounts: [{ hostPath: pgdataHostPath, containerPath: "/var/lib/postgresql/data" }],
    entrypoint: "/bin/bash",
    cmd: ["-c", "/usr/local/bin/docker-entrypoint.sh postgres & tail -f /dev/null"]
  });

  metrics.times_ms.container_start = nowMs() - tStart;

  // Wait ready
  const tReady = nowMs();
  await docker.waitPgReady({ container: pgName, user: args.pgUser });
  metrics.times_ms.pg_ready = nowMs() - tReady;

  const envUrl = `postgresql://${encodeURIComponent(args.pgUser)}:${encodeURIComponent(
    args.pgPassword
  )}@${pgName}:5432/${encodeURIComponent(args.pgDb)}`;
  const envUrlLocal = `postgresql://${encodeURIComponent(args.pgUser)}:${encodeURIComponent(
    args.pgPassword
  )}@localhost:5432/${encodeURIComponent(args.pgDb)}`;

  const clientEnv = {
    DATABASE_URL: envUrl,
    PGUSER: args.pgUser,
    PGPASSWORD: args.pgPassword,
    PGDATABASE: args.pgDb
  };
  const clientEnvLocal = {
    DATABASE_URL: envUrlLocal,
    PGUSER: args.pgUser,
    PGPASSWORD: args.pgPassword,
    PGDATABASE: args.pgDb
  };

  const runStep = async (label, cmd, logName, filePaths = null) => {
    assertStrictPsql(cmd);
    const out = path.join(logsDir, logName);
    const tCmd = nowMs();
    const isPsql = isPsqlCmd(cmd);
    const urlForPsql = args.client === "container" && isPsql ? envUrlLocal : envUrl;
    const finalCmd = injectDbUrlIntoPsql(cmd, urlForPsql);

    if (args.client === "container") {
      try {
        if (isPsql) {
          const files = filePaths && filePaths.length > 0 ? filePaths : collectFilesFromCmd(cmd);
          const staged = await stagePsqlFilesToContainer(pgName, files);
          const rewritten = rewritePsqlFileArgsForContainer(finalCmd, staged.root, staged.containerRoot);
          await docker.execToFile(pgName, out, rewritten.cmd, {
            user: "postgres",
            workdir: rewritten.workdir,
            env: clientEnvLocal
          });
        } else {
          await docker.execToFile(pgName, out, finalCmd, { env: clientEnvLocal });
        }
      } catch (err) {
        fs.appendFileSync(out, `\n-- Error during ${label} execution: ${err?.stack || String(err)}\n`);
        throw err;
      }
    } else {
      try {
        await run({
          cmd: finalCmd,
          env: { ...process.env, ...clientEnv },
          stdoutFile: out,
          stderrToStdout: true,
          tee: true
        });
      } catch (err) {
        fs.appendFileSync(out, `\n-- Error during ${label} execution: ${err?.stack || String(err)}\n`);
        throw err;
      }
    }

    metrics.times_ms[label] = nowMs() - tCmd;
    return { cmd: finalCmd, log: out };
  };

  if (prepareCmd && !reusedState) {
    await runStep("prepare", prepareCmd, "prepare.log", prepareState?.files || null);

    const tStop = nowMs();
    const stopLog = path.join(logsDir, "pg_stop.log");
    let stopError = null;
    try {
      await docker.execToFile(
        pgName,
        stopLog,
        ["pg_ctl", "-D", "/var/lib/postgresql/data", "-m", "fast", "-w", "stop"],
        { user: "postgres" }
      );
    } catch (err) {
      stopError = err;
      fs.appendFileSync(stopLog, `\n-- Error during pg_ctl stop: ${err?.stack || String(err)}\n`);
    }

    const stillReady = await docker.isPgReady({ container: pgName, user: args.pgUser });
    if (stillReady) {
      throw stopError || new Error("Postgres did not stop");
    }
    if (stopError) {
      console.warn("[warn] pg_ctl stop returned error, but server is down.");
    }
    metrics.times_ms.pg_stop = nowMs() - tStop;

    const tSnap = nowMs();
    const snap = await snapshotVolume({
      workspace: ws,
      volume: vol,
      runId,
      storage: args.storage,
      image: from.image,
      stateId: prepareState ? prepareState.stateId : `${runId}-prepare`,
      prepareCmd
    });
    metrics.times_ms.snapshot = nowMs() - tSnap;
    metrics.snapshot = { state_id: snap.stateId, path: snap.stateDir };

    if (runCmd) {
      const tStartPg = nowMs();
      const startLog = path.join(logsDir, "pg_start.log");
      try {
        const running = await docker.isRunning(pgName);
        if (!running) await docker.start(pgName);
        await docker.execToFile(
          pgName,
          startLog,
          ["pg_ctl", "-D", "/var/lib/postgresql/data", "-w", "start"],
          { user: "postgres" }
        );
        await docker.waitPgReady({ container: pgName, user: args.pgUser });
        metrics.times_ms.pg_restart = nowMs() - tStartPg;
      } catch(err) {
        fs.appendFileSync(startLog, `\n-- Error during pg_ctl start: ${err?.stack || String(err)}\n`);
        throw err;
      }
    }
  }

  // sizes (best-effort, plain du; CoW backends can override later)
  metrics.sizes.pgdata_du_bytes = await backend.statBytes({ volume: vol });

  if (runCmd) {
    await runStep("run", runCmd, "run.log");
  }

  metrics.sizes.pgdata_du_bytes_after = await backend.statBytes({ volume: vol });
  metrics.times_ms.total = nowMs() - t0;

  await writeJson(path.join(runDir, "metrics.json"), metrics);

  // Cleanup (PoC: always clean docker + volume; later add --keep)
  await docker.stopRm(pgName);
  await docker.networkRm(netName);
  await backend.destroyVolume({ volume: vol });

  console.log(`Run complete: ${runId}`);
  console.log(`Metrics: ${path.join(runDir, "metrics.json")}`);
}

main().catch((err) => {
  console.error(err?.stack || String(err));
  process.exit(1);
});
