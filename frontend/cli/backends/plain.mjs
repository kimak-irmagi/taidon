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
  },

  async snapshotVolume({ workspace, volume, runId, storage, image, stateId, prepareCmd }) {
    const stateDir = path.join(workspace, "states", stateId);
    if (fs.existsSync(stateDir)) {
      throw new Error(`State already exists: ${stateDir}`);
    }
  
    const pgdataDir = path.join(stateDir, "pgdata");
    ensureDir(pgdataDir);
    copyDir(volume.path, pgdataDir);
  
    await writeJson(path.join(stateDir, "state.json"), {
      state_id: stateId,
      run_id: runId,
      created_at: new Date().toISOString(),
      storage,
      image,
      pgdata: "pgdata",
      prepare: prepareCmd ? { cmd: prepareCmd } : null
    });
  
    return { stateId, stateDir };
  }
};

function copyDir(src, dst) {
  ensureDir(dst);
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    const s = path.join(src, entry.name);
    const d = path.join(dst, entry.name);
    if (entry.isDirectory()) copyDir(s, d);
    else if (entry.isFile()) fs.copyFileSync(s, d);
  }
}
