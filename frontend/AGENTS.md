# Agent notes for frontend

- When running Go tests under the sandbox, do NOT set `GOCACHE` or  `GOMODCACHE` to nonstandard paths. If tests fail due to the permission errors, ask users to run tests manually to warm up the caches.
