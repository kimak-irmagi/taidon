# sqlrs-engine (local, Go)

Minimal local engine used by `sqlrs` for MVP validation.
Right now it exposes only `/v1/health` and auto-shuts down after an idle timeout.

## Build

From repository root:

```bash
go build -o dist/bin/sqlrs-engine ./backend/local-engine-go/cmd/sqlrs-engine
```

From this module directory:

```bash
go build -o ../../dist/bin/sqlrs-engine ./cmd/sqlrs-engine
```

## Run

```bash
./dist/bin/sqlrs-engine \
  --listen 127.0.0.1:0 \
  --run-dir /tmp/sqlrs-run \
  --write-engine-json /tmp/sqlrs-state/engine.json \
  --idle-timeout 30s
```

## Flags

- `--listen` (required): host:port to bind (use `127.0.0.1:0` for random port).
- `--run-dir`: runtime directory (created if missing).
- `--write-engine-json` (required): path to `engine.json` written on startup.
- `--idle-timeout`: shutdown after idle period (default `30s`).
- `--version`: engine version string (default `dev`).

## Health check

```
GET /v1/health
```

Response:

```json
{"ok":true,"version":"dev","instanceId":"...","pid":1234}
```
