import path from "node:path";
import fs from "node:fs";
import { runCapture } from "../lib/proc.mjs";

const BTRFS_ROOT = "/home/kislosladky/diploma/btrfs-ws"; 

async function btrfs(cmd) {
    return runCapture({ cmd: ["sudo", "btrfs", ...cmd] });
}

async function ensureDir(dir) {
    await fs.promises.mkdir(dir, { recursive: true });
}

export const btrfsBackend = {

    async createVolume({ workspace, runId }) {
        const subvolPath = path.join(BTRFS_ROOT, `run-${runId}`);

        // 1️⃣ создать subvolume
        await btrfs([
            "subvolume",
            "create",
            subvolPath
        ]);

        return {
            id: runId,
            path: subvolPath,
            meta: {
                type: "btrfs",
                subvolume: subvolPath,
                empty: true
            }
        };
    },

    async destroyVolume({ volume }) {
        if (!volume?.meta?.subvolume) return;

        const subvol = volume.meta.subvolume;

        try {
            await btrfs(["subvolume", "delete", subvol]);
        } catch (err) {
            // если snapshot read-only — нужно удалить через -c или сначала снять ro
            throw new Error(`Failed to delete Btrfs subvolume ${subvol}: ${err.message}`);
        }
    },

    async statBytes({ volume }) {
        if (!volume?.meta?.subvolume) return null;

        try {
            // Btrfs не даёт простой "used" как ZFS
            // fallback — du (реальное использование, COW-aware)
            const result = await runCapture({
                cmd: ["sudo", "du", "-sb", volume.meta.subvolume]
            });

            const bytes = result.trim().split("\t")[0];
            return Number(bytes);
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
        if (!volume?.meta?.subvolume) {
            throw new Error("Btrfs volume must contain meta.subvolume");
        }

        const source = volume.meta.subvolume;
        const snapshotPath = path.join(
            BTRFS_ROOT,
            `snap-${runId}-${stateId}`
        );

        // 1️⃣ создаём read-only snapshot (O(1), COW)
        try {
            await btrfs([
                "subvolume",
                "snapshot",
                "-r",
                source,
                snapshotPath
            ]);
        } catch (err) {
            throw new Error(`Failed to create Btrfs snapshot ${snapshotPath}: ${err.message}`);
        }

        // 2️⃣ создаём каталог состояния (метаданные)
        const stateDir = path.join(workspace, "states", stateId);
        if (fs.existsSync(stateDir)) {
            throw new Error(`State already exists: ${stateDir}`);
        }

        await ensureDir(stateDir);

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
                    snapshot: snapshotPath,
                    prepare: prepareCmd ? { cmd: prepareCmd } : null
                },
                null,
                2
            )
        );

        return {
            stateId,
            stateDir,
            snapshot: snapshotPath
        };
    }
};
