// scripts/wsl-run.mjs
import { spawn } from "node:child_process";

function run(cmd, args) {
  return new Promise((resolve, reject) => {
    const p = spawn(cmd, args, { stdio: "inherit", shell: false });
    p.on("error", reject);
    p.on("exit", (code) => (code === 0 ? resolve() : reject(new Error(`Exit ${code}`))));
  });
}

function escapeForBash(words) {
  // minimal escaping for bash -lc
  return words.map((w) => w.replace(/(["\\$`])/g, "\\$1")).join(" ");
}

async function main() {
  const argv = process.argv.slice(2);
  const sep = argv.indexOf("--");
  if (sep === -1) {
    console.error("Usage: node scripts/wsl-run.mjs [--distro <Ubuntu>] -- <command...>");
    process.exit(2);
  }

  const pre = argv.slice(0, sep);
  const cmd = argv.slice(sep + 1);

  const distroIdx = pre.indexOf("--distro");
  const distro = distroIdx !== -1 ? pre[distroIdx + 1] : undefined;

  const bashCmd = escapeForBash(cmd);
  const wslArgs = [];
  if (distro) wslArgs.push("-d", distro);
  wslArgs.push("--", "bash", "-lc", bashCmd);

  await run("wsl.exe", wslArgs);
}

main().catch((e) => {
  console.error(String(e));
  process.exit(1);
});
