import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRootDefault = path.resolve(__dirname, "..", "..");

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

function loadScenario(scenariosPath, scenarioId) {
  const raw = fs.readFileSync(scenariosPath, "utf8");
  const parsed = JSON.parse(raw);
  const scenarios = Array.isArray(parsed.scenarios) ? parsed.scenarios : [];
  const scenario = scenarios.find((item) => item.id === scenarioId);
  if (!scenario) {
    throw new Error(`Scenario not found: ${scenarioId}`);
  }
  return scenario;
}

function normalizeLines(text, dropPatterns) {
  const regexes = dropPatterns.map((pattern) => new RegExp(pattern));
  const lines = text
    .replace(/\r\n/g, "\n")
    .replace(/\r/g, "\n")
    .split("\n")
    .map((line) => line.replace(/\u001b\[[0-9;]*m/g, "").replace(/[ \t]+$/g, ""));

  const kept = lines.filter((line) => !regexes.some((re) => re.test(line)));
  while (kept.length > 0 && kept[kept.length - 1] === "") {
    kept.pop();
  }
  return `${kept.join("\n")}\n`;
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const repoRoot = path.resolve(args["repo-root"] || repoRootDefault);
  const scenariosPath = path.resolve(args.scenarios || path.join(repoRoot, "test", "e2e", "release", "scenarios.json"));
  const scenarioId = args.scenario;
  const inputPath = path.resolve(args.input || "");
  const outputPath = path.resolve(args.output || "");

  if (!scenarioId) {
    throw new Error("Missing --scenario");
  }
  if (!inputPath) {
    throw new Error("Missing --input");
  }
  if (!outputPath) {
    throw new Error("Missing --output");
  }

  const scenario = loadScenario(scenariosPath, scenarioId);
  const scenarioDropPatterns = Array.isArray(scenario?.normalize?.dropLinePatterns) ? scenario.normalize.dropLinePatterns : [];
  const dropPatterns = ["^Deleting instance\\b.*$", ...scenarioDropPatterns];
  const input = fs.readFileSync(inputPath, "utf8");
  const normalized = normalizeLines(input, dropPatterns);
  fs.writeFileSync(outputPath, normalized, "utf8");
}

try {
  main();
} catch (err) {
  console.error(err?.stack || String(err));
  process.exit(1);
}
