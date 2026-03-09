# sqlrs CLI (Go)

Primary Go implementation of the `sqlrs` CLI for local and remote profiles.

Current command groups include:

- `init`
- `status`
- `ls`
- `rm`
- `config`
- `watch`
- `plan:psql`, `plan:lb`
- `prepare:psql`, `prepare:lb`
- `run:psql`, `run:pgbench`

The CLI loads workspace/global config, autostarts the local engine when needed,
and supports Windows + WSL local mode.

See also:

- `SPEC.md`
- `../../docs/architecture/cli-contract.md`
- `../../docs/architecture/cli-component-structure.md`
- `../../docs/architecture/local-engine-cli-maintainability-refactor.md`

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

The local release bundles `sqlrs` and `sqlrs-engine` into archives under
`dist/release/`.

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
