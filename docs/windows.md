# Windows (Docker Desktop + WSL2) setup & workflow

We run everything inside WSL2 for consistent Linux tooling (docker CLI, filesystem backends).

## Rules of thumb

1. Run experiments in the **WSL filesystem** (e.g. `~/taidon-ws/exp01`), not in `/mnt/c/...`.
2. Run commands in an **Ubuntu (WSL) terminal**.
3. Docker must be available inside WSL:
   - Docker Desktop installed
   - WSL integration enabled for your distro

Verify inside WSL:

```bash
docker version
docker ps
```

## Typical workflow (inside WSL)

```bash
cd ~/taidon-repo
pnpm install

mkdir -p ~/taidon-ws/exp01
pnpm bench -- --workspace ~/taidon-ws/exp01 --examples chinook --storages plain --snapshots off
```

## Running from PowerShell (optional)

You can launch the same commands via a helper that executes them in WSL:

```ps
node scripts/wsl-run.mjs --distro Ubuntu -- pnpm bench -- --workspace ~/taidon-ws/exp01 --examples chinook --storages plain,btrfs
```