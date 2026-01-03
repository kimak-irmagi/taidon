import fs from "node:fs";
import { spawn } from "node:child_process";

export function run({ cmd, env, cwd, stdoutFile, stderrToStdout = false, tee = false }) {
  return new Promise((resolve, reject) => {
    const p = spawn(cmd[0], cmd.slice(1), { env, cwd, shell: false });
    let outStream = null;

    if (stdoutFile) {
      outStream = fs.createWriteStream(stdoutFile, { flags: "w" });
      p.stdout.pipe(outStream);
      if (tee) p.stdout.pipe(process.stdout);
      if (stderrToStdout) {
        p.stderr.pipe(outStream);
        if (tee) p.stderr.pipe(process.stderr);
      } else {
        p.stderr.pipe(process.stderr);
      }
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

export function runCapture({ cmd, env, cwd }) {
  return new Promise((resolve, reject) => {
    const p = spawn(cmd[0], cmd.slice(1), { env, cwd, shell: false });
    let buf = "";
    p.stdout.on("data", (d) => (buf += d.toString("utf8")));
    p.stderr.pipe(process.stderr);
    p.on("error", reject);
    p.on("exit", (code) => {
      if (code === 0) resolve(buf.trim());
      else reject(new Error(`Command failed (${code}): ${cmd.join(" ")}`));
    });
  });
}
