# Experiments workflow

## Workspace layout

We run experiments in an external workspace directory (not necessarily inside the repo).

Recommended layout:

- `<workspace>/dist/bin/sqlrs` (or `sqlrs.cmd`) — built CLI
- `<workspace>/sqlrs-work/` — run artifacts (metrics, logs)
- `<workspace>/results/` — experiment logs and aggregated outputs

## Build CLI into a workspace

```bash
pnpm build:cli -- <workspace-path>
```

Example:

```bash
pnpm build:cli -- ~/taidon-ws/exp01
```

## Run one example

```bash
pnpm run:one -- \
  --workspace ~/taidon-ws/exp01 \
  --example chinook \
  --storage plain \
  --snapshots off
```

## Run a matrix benchmark

```bash
pnpm bench -- \
  --workspace ~/taidon-ws/exp01 \
  --examples chinook,sakila,postgrespro-demo \
  --storages plain,btrfs,zfs \
  --snapshots off,stop
```

`bench` stores per-run logs under `<workspace>/results/…` and `sqlrs` stores metrics under `<workspace>/sqlrs-work/runs/<run_id>/metrics.json`.
