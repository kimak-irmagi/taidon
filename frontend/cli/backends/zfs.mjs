import path from "node:path";
import fs from "node:fs";
import { runCapture } from "../lib/proc.mjs";

const ZFS_POOL = "tank";
const ZFS_ROOT = `${ZFS_POOL}/workspaces`;
const ZFS_BASE = `${ZFS_ROOT}/base`;
const ZFS_BASE_SNAPSHOT = "clean"; // base@clean

async function zfs(cmd) {
    return runCapture({ cmd: ["sudo", "zfs", ...cmd] });
}

export const zfsBackend = {
    async createVolume({ workspace, runId }) {
    const dataset = `${ZFS_ROOT}/run-${runId}`;

    // 1. создать пустой dataset
    await zfs([
        "create",
        "-o", "canmount=on",
        dataset
    ]);

    // 2. получить mountpoint
    const mountpoint = (await zfs([
        "get",
        "-H",
        "-o",
        "value",
        "mountpoint",
        dataset
    ])).trim();

    return {
        id: runId,
        path: mountpoint,
        meta: {
        type: "zfs",
        dataset,
        empty: true
        }
    };
    },

    async destroyVolume({ volume }) {
        if (!volume?.meta?.dataset) return;

        // recursive destroy is safe for clones
        await zfs(["destroy", "-r", volume.meta.dataset]);
    },

    async statBytes({ volume }) {
        if (!volume?.meta?.dataset) return null;

        try {
        // "used" — сколько реально занимает (COW-aware)
        const used = await zfs([
            "get",
            "-H",
            "-o",
            "value",
            "used",
            volume.meta.dataset
        ]);

        return Number(used.trim());
        } catch {
        return null;
        }
    },

    async snapshotVolume({
        workspace,
        volume,
        runId,
        storage,
        image,
        stateId,
        prepareCmd
    }) {
        if (!volume?.meta?.dataset) {
            throw new Error("ZFS volume must contain meta.dataset");
        }

        const dataset = volume.meta.dataset;
        const snapshot = `${dataset}@${stateId}`;

        // 1️⃣ создаём ZFS snapshot (атомарно, O(1))
        try {
            await zfs(["snapshot", snapshot]);
        } catch (err) {
            throw new Error(`Failed to create ZFS snapshot ${snapshot}: ${err.message}`);
        }

        // 2️⃣ создаём каталог состояния ТОЛЬКО для метаданных
        const stateDir = path.join(ZFS_ROOT, "states", stateId);
        if (fs.existsSync(stateDir)) {
            throw new Error(`State already exists: ${stateDir}`);
        }

        await ensureDataset(stateDir);

        // 3️⃣ сохраняем описание состояния
        await fs.promises.writeFile(
            path.join(stateDir, "state.json"),
            JSON.stringify(
            {
                state_id: stateId,
                run_id: runId,
                created_at: new Date().toISOString(),
                storage,
                image,
                snapshot,              // главное отличие
                prepare: prepareCmd ? { cmd: prepareCmd } : null
            },
            null,
            2
            )
        );

        return {
            stateId,
            stateDir,
            snapshot
        };
    }
    
};

async function ensureDataset(dataset) {
    try {
        await zfs(["list", dataset]);
    } catch {
        await zfs(["create", "-p", dataset]);
    }
}
