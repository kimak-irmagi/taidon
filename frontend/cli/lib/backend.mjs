import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

export async function loadBackend(name, cliRootUnused) {
  // The built dist layout is dist/bin/sqlrs.js + dist/bin/sqlrs.d/...
  // We can locate sqlrs.d relative to current file at runtime.
  const here = path.dirname(fileURLToPath(import.meta.url));
  const backendsDir = path.resolve(here, "..", "backends");

  if (name === "plain") {
    const url = pathToFileURL(path.join(backendsDir, "plain.mjs"));
    const mod = await import(url.href);
    return mod.plainBackend;
  }
  // placeholders (we'll implement later)
  if (name === "btrfs" || name === "zfs") {
    throw new Error(`${name} backend not implemented yet (PoC step 2/3). Use --storage plain for now.`);
  }
  throw new Error(`Unknown backend: ${name}`);
}
