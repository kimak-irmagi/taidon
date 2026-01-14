# sqlrs CLI (Go)

Status: MVP scaffold with `status` command, local connect-or-start, and config loading.

This CLI lives under `frontend/cli-go`. See `SPEC.md` for the full specification.

## Build

```bash
cd frontend/cli-go
go build ./...
```

Binary output (repo root):

```bash
go build -o ../../dist/bin/sqlrs ./cmd/sqlrs
```

Windows:

```bash
go build -o ../../dist/bin/sqlrs.exe ./cmd/sqlrs
```

Or from repo root:

```bash
node scripts/build-cli-go.mjs
```

## Release packaging (local variant)

The local release bundles `sqlrs` + `sqlrs-engine` into archives under `dist/release/`.

Provide a platform-specific engine binary per target, for example:

```bash
dist/engine/<os>_<arch>/sqlrs-engine[.exe]
```

Build an archive:

```bash
node scripts/release-local.mjs --version v0.1.0 --os linux --arch amd64 --engine-bin dist/engine/linux_amd64/sqlrs-engine
```

## Test

```bash
go test ./...
```
