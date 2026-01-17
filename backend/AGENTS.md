# Agent notes for backend

- When running Go tests under the sandbox, set `GOCACHE` and `GOMODCACHE` to writable paths inside the module directory to avoid permission errors.
- Example for the engine module: `GOCACHE=backend/local-engine-go/.gocache GOMODCACHE=backend/local-engine-go/.gomodcache go test ./...`
