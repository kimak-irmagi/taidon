import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { ensureDir, run } from "./_lib.mjs";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const repoRoot = path.resolve(__dirname, "..");

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

function rmrf(p) {
  if (!fs.existsSync(p)) return;
  fs.rmSync(p, { recursive: true, force: true });
}

function sha256File(pathname) {
  const data = fs.readFileSync(pathname);
  return crypto.createHash("sha256").update(data).digest("hex");
}

function isWindowsTarget(goos) {
  return goos === "windows";
}

const args = parseArgs(process.argv.slice(2));
const version = args.version || process.env.SQLRS_VERSION;
const goos = args.os || process.env.GOOS;
const goarch = args.arch || process.env.GOARCH;
const workspace = args.workspace ? path.resolve(args.workspace) : repoRoot;
const engineBin = args["engine-bin"] || process.env.SQLRS_ENGINE_BIN;

if (!version) {
  throw new Error("Missing version. Provide --version or SQLRS_VERSION.");
}
if (!goos || !goarch) {
  throw new Error("Missing target platform. Provide --os and --arch (or GOOS/GOARCH).");
}
if (!engineBin) {
  throw new Error("Missing engine binary. Provide --engine-bin or SQLRS_ENGINE_BIN.");
}

const exeSuffix = isWindowsTarget(goos) ? ".exe" : "";

const cliRoot = path.join(repoRoot, "frontend", "cli-go");
const distBin = path.resolve(workspace, "dist", "bin", `${goos}_${goarch}`);
const distRelease = path.resolve(workspace, "dist", "release");
const stagingRoot = path.join(distRelease, "staging");
const stagingDir = path.join(stagingRoot, `sqlrs_${version}_${goos}_${goarch}`);

ensureDir(distBin);
ensureDir(distRelease);
ensureDir(stagingRoot);
rmrf(stagingDir);
ensureDir(stagingDir);

const cliPath = path.join(distBin, `sqlrs${exeSuffix}`);
const engineOut = path.join(distBin, `sqlrs-engine${exeSuffix}`);
const enginePath = path.resolve(engineBin);

await run({
  cmd: ["go", "build", "-o", cliPath, "./cmd/sqlrs"],
  cwd: cliRoot,
  env: { ...process.env, GOOS: goos, GOARCH: goarch, CGO_ENABLED: "0" }
});

if (!fs.existsSync(enginePath)) {
  throw new Error(`Engine binary not found: ${enginePath}`);
}
fs.copyFileSync(enginePath, engineOut);

fs.copyFileSync(cliPath, path.join(stagingDir, `sqlrs${exeSuffix}`));
fs.copyFileSync(engineOut, path.join(stagingDir, `sqlrs-engine${exeSuffix}`));
fs.copyFileSync(path.join(repoRoot, "LICENSE"), path.join(stagingDir, "LICENSE"));
fs.copyFileSync(path.join(repoRoot, "README.md"), path.join(stagingDir, "README.md"));

let archivePath = "";
if (isWindowsTarget(goos)) {
  if (process.platform !== "win32") {
    throw new Error("Windows target requires running on Windows for zip packaging.");
  }
  archivePath = path.join(distRelease, `sqlrs_${version}_${goos}_${goarch}.zip`);
  await run({
    cmd: [
      "powershell",
      "-NoProfile",
      "-Command",
      `Compress-Archive -Path "${stagingDir}\\*" -DestinationPath "${archivePath}" -Force`
    ],
    cwd: workspace
  });
} else {
  archivePath = path.join(distRelease, `sqlrs_${version}_${goos}_${goarch}.tar.gz`);
  await run({
    cmd: ["tar", "-czf", archivePath, "-C", stagingRoot, path.basename(stagingDir)],
    cwd: workspace
  });
}

const checksum = sha256File(archivePath);
const checksumPath = `${archivePath}.sha256`;
fs.writeFileSync(checksumPath, `${checksum}  ${path.basename(archivePath)}\n`, "utf8");

console.log(`Built sqlrs local release: ${archivePath}`);
