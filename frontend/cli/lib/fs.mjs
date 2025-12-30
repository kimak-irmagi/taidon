import fs from "node:fs";
import path from "node:path";

export function ensureDir(p) {
  fs.mkdirSync(p, { recursive: true });
}

export function nowMs() {
  return Date.now();
}

export async function writeJson(p, obj) {
  await fs.promises.mkdir(path.dirname(p), { recursive: true });
  await fs.promises.writeFile(p, JSON.stringify(obj, null, 2) + "\n", "utf8");
}
