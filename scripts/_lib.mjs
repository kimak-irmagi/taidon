// scripts/_lib.mjs
import fs from "node:fs";
import path from "node:path";
import { spawn } from "node:child_process";

export function ensureDir(p) {
  fs.mkdirSync(p, { recursive: true });
}

export function isWindows() {
  return process.platform === "win32";
}

export function sqlrsPath(workspace) {
  const bin = path.resolve(workspace, "dist", "bin");
  return isWindows() ? path.join(bin, "sqlrs.cmd") : path.join(bin, "sqlrs");
}

export function run({ cmd, cwd, env, stdoutFile, tee = false }) {
  return new Promise((resolve, reject) => {
    const useShell = isWindows() && /\.(cmd|bat)$/i.test(cmd[0]);
    const p = spawn(cmd[0], cmd.slice(1), {
      cwd,
      env: env ?? process.env,
      shell: useShell
    });

    let outStream = null;
    if (stdoutFile) {
      ensureDir(path.dirname(stdoutFile));
      outStream = fs.createWriteStream(stdoutFile, { flags: "w" });
      p.stdout.pipe(outStream);
      if (tee) p.stdout.pipe(process.stdout);
      p.stderr.pipe(outStream);
      if (tee) p.stderr.pipe(process.stderr);
    } else {
      p.stdout.pipe(process.stdout);
      p.stderr.pipe(process.stderr);
    }

    p.on("error", reject);
    p.on("exit", (code) => {
      if (outStream) outStream.end();
      if (code === 0) resolve();
      else reject(new Error(`Command failed (${code}): ${cmd.join(" ")}`));
    });
  });
}

export function nowIsoSafe() {
  return new Date().toISOString().replace(/[:.]/g, "-");
}
