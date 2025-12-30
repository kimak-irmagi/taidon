import path from "node:path";
import fs from "node:fs";
import { ensureDir } from "../lib/fs.mjs";
import { runCapture } from "../lib/proc.mjs";

async function dirSizeBytes(p) {
  let total = 0;
  const entries = await fs.promises.readdir(p, { withFileTypes: true });
  for (const entry of entries) {
    const full = path.join(p, entry.name);
    if (entry.isDirectory()) {
      total += await dirSizeBytes(full);
    } else if (entry.isFile()) {
      const st = await fs.promises.stat(full);
      total += st.size;
    }
  }
  return total;
}

export const plainBackend = {
  async createVolume({ workspace, runId }) {
    const volPath = path.join(workspace, "volumes", "plain", runId);
    ensureDir(volPath);
    return { id: runId, path: volPath, meta: { type: "plain" } };
  },

  async destroyVolume({ volume }) {
    if (!volume?.path) return;
    await fs.promises.rm(volume.path, { recursive: true, force: true });
  },

  async statBytes({ volume }) {
    if (!volume?.path) return null;
    if (process.platform === "win32") {
      try {
        return await dirSizeBytes(volume.path);
      } catch {
        return null;
      }
    }
    // best-effort: du -sb (works on Linux/macOS; Windows with WSL)
    // If du isn't available, return null.
    try {
      const out = await runCapture({ cmd: ["du", "-sb", volume.path] });
      const first = out.split(/\s+/)[0];
      const n = Number(first);
      return Number.isFinite(n) ? n : null;
    } catch {
      try {
        return await dirSizeBytes(volume.path);
      } catch {
        return null;
      }
    }
  }
};
