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

function buildDiff(expected, actual) {
  const exp = expected.replace(/\r\n/g, "\n").replace(/\r/g, "\n").split("\n");
  const act = actual.replace(/\r\n/g, "\n").replace(/\r/g, "\n").split("\n");
  const max = Math.max(exp.length, act.length);
  const lines = ["--- expected", "+++ actual"];
  for (let i = 0; i < max; i += 1) {
    const left = i < exp.length ? exp[i] : null;
    const right = i < act.length ? act[i] : null;
    if (left === right) {
      if (left !== null) {
        lines.push(` ${left}`);
      }
      continue;
    }
    if (left !== null) {
      lines.push(`-${left}`);
    }
    if (right !== null) {
      lines.push(`+${right}`);
    }
  }
  return `${lines.join("\n")}\n`;
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const expectedPath = path.resolve(args.expected || "");
  const actualPath = path.resolve(args.actual || "");
  const diffPath = path.resolve(args.diff || "");

  if (!expectedPath) {
    throw new Error("Missing --expected");
  }
  if (!actualPath) {
    throw new Error("Missing --actual");
  }
  if (!diffPath) {
    throw new Error("Missing --diff");
  }

  const expected = fs.readFileSync(expectedPath, "utf8");
  const actual = fs.readFileSync(actualPath, "utf8");
  if (expected === actual) {
    fs.writeFileSync(diffPath, "match\n", "utf8");
    return;
  }

  const diff = buildDiff(expected, actual);
  fs.writeFileSync(diffPath, diff, "utf8");
  throw new Error("Normalized output does not match golden");
}

try {
  main();
} catch (err) {
  console.error(err?.stack || String(err));
  process.exit(1);
}
