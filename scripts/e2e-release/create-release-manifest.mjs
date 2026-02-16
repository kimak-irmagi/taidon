import fs from "node:fs";
import path from "node:path";

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

function walkFiles(dir, out = []) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      walkFiles(fullPath, out);
    } else {
      out.push(fullPath);
    }
  }
  return out;
}

function escapeRegex(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function parseChecksum(pathname) {
  const raw = fs.readFileSync(pathname, "utf8").trim();
  const parts = raw.split(/\s+/);
  if (parts.length < 2) {
    throw new Error(`Invalid checksum file format: ${pathname}`);
  }
  return parts[0];
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const version = args.version;
  const inputDir = path.resolve(args["input-dir"] || "");
  const outputPath = path.resolve(args.output || "");
  const sourceSHA = args["source-sha"] || "";
  const workflowRunID = args["workflow-run-id"] || "";

  if (!version) {
    throw new Error("Missing --version");
  }
  if (!inputDir || !fs.existsSync(inputDir)) {
    throw new Error(`Input directory not found: ${inputDir}`);
  }
  if (!outputPath) {
    throw new Error("Missing --output");
  }

  const archiveRe = new RegExp(`^sqlrs_${escapeRegex(version)}_([a-z0-9]+)_([a-z0-9]+)\\.(tar\\.gz|zip)$`);
  const files = walkFiles(inputDir);
  const targets = [];

  for (const filePath of files) {
    const base = path.basename(filePath);
    const match = base.match(archiveRe);
    if (!match) {
      continue;
    }
    const checksumPath = `${filePath}.sha256`;
    if (!fs.existsSync(checksumPath)) {
      throw new Error(`Missing checksum for archive: ${filePath}`);
    }
    targets.push({
      os: match[1],
      arch: match[2],
      archive: base,
      checksum_file: path.basename(checksumPath),
      sha256: parseChecksum(checksumPath)
    });
  }

  if (targets.length === 0) {
    throw new Error(`No release archives found for version ${version} in ${inputDir}`);
  }

  targets.sort((a, b) => {
    if (a.os === b.os) {
      return a.arch.localeCompare(b.arch);
    }
    return a.os.localeCompare(b.os);
  });

  ensureDir(path.dirname(outputPath));
  fs.writeFileSync(
    outputPath,
    `${JSON.stringify(
      {
        version,
        source_sha: sourceSHA,
        workflow_run_id: workflowRunID,
        generated_at: new Date().toISOString(),
        targets
      },
      null,
      2
    )}\n`,
    "utf8"
  );
}

try {
  main();
} catch (err) {
  console.error(err?.stack || String(err));
  process.exit(1);
}
