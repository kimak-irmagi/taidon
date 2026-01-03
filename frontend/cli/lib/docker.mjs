import { run, runCapture } from "./proc.mjs";

export const docker = {
  async start(name) {
    await run({ cmd: ["docker", "start", name] });
  },
  async stop(name) {
    try { await run({ cmd: ["docker", "stop", name] }); } catch {}
  },
  async isRunning(name) {
    try {
      const out = await runCapture({ cmd: ["docker", "inspect", "-f", "{{.State.Running}}", name] });
      return out.trim() === "true";
    } catch {
      return false;
    }
  },
  async networkCreate(name) {
    await run({ cmd: ["docker", "network", "create", name] });
  },
  async networkRm(name) {
    // ignore errors
    try { await run({ cmd: ["docker", "network", "rm", name] }); } catch {}
  },
  async runDetached({ name, image, network, env, mounts, entrypoint, cmd }) {
    const args = ["docker", "run", "-d", "--name", name, "--network", network];
    for (const [k, v] of Object.entries(env || {})) args.push("-e", `${k}=${v}`);
    for (const m of mounts || []) args.push("-v", `${m.hostPath}:${m.containerPath}`);
    if (entrypoint) args.push("--entrypoint", entrypoint);
    args.push(image);
    if (cmd && cmd.length > 0) args.push(...cmd);
    await run({ cmd: args });
  },
  async stopRm(name) {
    try { await run({ cmd: ["docker", "rm", "-f", name] }); } catch {}
  },
  async execToFile(container, outfile, cmd, opts = {}) {
    const args = ["docker", "exec"];
    if (opts.user) args.push("-u", opts.user);
    if (opts.workdir) args.push("-w", opts.workdir);
    for (const [k, v] of Object.entries(opts.env || {})) args.push("-e", `${k}=${v}`);
    args.push(container, ...cmd);
    await run({
      cmd: args,
      stdoutFile: outfile,
      stderrToStdout: true,
      tee: true
    });
  },
  async cpToContainer(src, container, dst) {
    await run({ cmd: ["docker", "cp", src, `${container}:${dst}`] });
  },
  async isPgReady({ container, user }) {
    try {
      await runCapture({ cmd: ["docker", "exec", container, "pg_isready", "-U", user] });
      return true;
    } catch {
      return false;
    }
  },
  async waitPgReady({ container, user, idleMs = 60000 }) {
    // retry until ready; only timeout if container is silent for too long
    let lastActivity = Date.now();
    let lastLogCheck = Date.now();

    while (true) {
      try {
        await run({ cmd: ["docker", "exec", container, "pg_isready", "-U", user] });
        return;
      } catch {}

      try {
        const since = new Date(lastLogCheck).toISOString();
        const out = await runCapture({ cmd: ["docker", "logs", "--since", since, container] });
        if (out && out.trim().length > 0) {
          lastActivity = Date.now();
        }
      } catch {}

      lastLogCheck = Date.now();
      if (Date.now() - lastActivity > idleMs) {
        throw new Error(`Postgres did not become ready (no output for ${Math.round(idleMs / 1000)}s)`);
      }

      await new Promise((r) => setTimeout(r, 500));
    }
  },
  async runToFile({ image, network, env, mounts, workdir, cmd, outfile, tee = false }) {
    const args = ["docker", "run", "--rm", "--network", network];
    for (const [k, v] of Object.entries(env || {})) args.push("-e", `${k}=${v}`);
    for (const m of mounts || []) {
      const spec = m.readOnly ? `${m.hostPath}:${m.containerPath}:ro` : `${m.hostPath}:${m.containerPath}`;
      args.push("-v", spec);
    }
    if (workdir) args.push("-w", workdir);
    args.push(image, ...cmd);
    await run({ cmd: args, stdoutFile: outfile, stderrToStdout: true, tee });
  }
};
