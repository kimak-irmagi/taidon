import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

function rmrf(p) {
  if (!fs.existsSync(p)) return;
  fs.rmSync(p, { recursive: true, force: true });
}

function mkdirp(p) {
  fs.mkdirSync(p, { recursive: true });
}

function copyDir(src, dst) {
  mkdirp(dst);
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    const s = path.join(src, entry.name);
    const d = path.join(dst, entry.name);
    if (entry.isDirectory()) copyDir(s, d);
    else fs.copyFileSync(s, d);
  }
}

function chmodX(p) {
  try {
    fs.chmodSync(p, 0o755);
  } catch {
    // windows: ignore
  }
}

const repoRoot = path.resolve(__dirname, "..");
const workspace = process.argv[2] ? path.resolve(process.argv[2]) : repoRoot;
const srcCli = path.join(repoRoot, "frontend", "cli");

const distBin = path.resolve(workspace, "dist", "bin");
const distRoot = path.join(distBin, "sqlrs.d");

mkdirp(distBin);
mkdirp(distRoot);

// Copy main entry
fs.copyFileSync(path.join(srcCli, "sqlrs.mjs"), path.join(distBin, "sqlrs.js"));

// Copy libs/backends into sqlrs.d
rmrf(distRoot);
mkdirp(distRoot);
copyDir(path.join(srcCli, "lib"), path.join(distRoot, "lib"));
copyDir(path.join(srcCli, "backends"), path.join(distRoot, "backends"));

// Unix shim
const unixShim = `#!/usr/bin/env sh
DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
exec node "$DIR/sqlrs.js" "$@"
`;
fs.writeFileSync(path.join(distBin, "sqlrs"), unixShim, "utf8");
chmodX(path.join(distBin, "sqlrs"));

// Windows shim (.cmd)
const winShim = `@echo off
setlocal
set DIR=%~dp0
node "%DIR%sqlrs.js" %*
`;
fs.writeFileSync(path.join(distBin, "sqlrs.cmd"), winShim, "utf8");

console.log(`Built sqlrs into: ${distBin}`);
