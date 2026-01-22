# Server config interaction flow

This document describes how `sqlrs config` interacts with the engine and how
configuration updates are applied atomically.

## 1. Endpoints

- `GET /v1/config` (optional `path`, `effective`)
- `PATCH /v1/config` (set `path` + `value`)
- `DELETE /v1/config` (remove `path`)
- `GET /v1/config/schema`

## 2. Flow: get effective value

```mermaid
sequenceDiagram
  autonumber
  CLI->>API: GET /v1/config?path=...&effective=true
  API->>CFG: load effective config (defaults + overrides)
  CFG->>CFG: resolve path
  CFG-->>API: value
  API-->>CLI: 200 ConfigValue
```

## 3. Flow: set value

```mermaid
sequenceDiagram
  autonumber
  CLI->>API: PATCH /v1/config {path, value}
  API->>CFG: parse and validate
  CFG->>CFG: apply in-memory update (staged)
  CFG->>CFG: write JSON to temp file
  CFG->>CFG: fsync and atomic rename
  CFG->>CFG: commit in-memory update
  CFG-->>API: updated value
  API-->>CLI: 200 ConfigValue
```

## 4. Flow: remove value

```mermaid
sequenceDiagram
  autonumber
  CLI->>API: DELETE /v1/config?path=...
  API->>CFG: parse and validate
  CFG->>CFG: remove key (staged)
  CFG->>CFG: write JSON to temp file
  CFG->>CFG: fsync and atomic rename
  CFG->>CFG: commit in-memory update
  CFG-->>API: updated value
  API-->>CLI: 200 ConfigValue
```

## 5. Failure handling

- If validation fails, no state is mutated and the API returns `400`.
- If disk write or rename fails, the in-memory config is not committed.
- On engine start, config is loaded from the JSON file and merged with defaults.
