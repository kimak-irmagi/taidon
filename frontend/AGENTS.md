# Agent notes for frontend

- When running Go tests under the sandbox, set `GOCACHE` and `GOMODCACHE` to writable paths inside the module directory to avoid permission errors.
- Example for the CLI module: `GOCACHE=frontend/cli-go/.gocache GOMODCACHE=frontend/cli-go/.gomodcache go test ./...`
