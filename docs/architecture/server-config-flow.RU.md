# Взаимодействие с server config

Документ описывает, как `sqlrs config` взаимодействует с движком и как
изменения конфигурации применяются атомарно.

## 1. Эндпойнты

- `GET /v1/config` (опционально `path`, `effective`)
- `PATCH /v1/config` (установка `path` + `value`)
- `DELETE /v1/config` (удаление `path`)
- `GET /v1/config/schema`

## 2. Поток: get effective value

```mermaid
sequenceDiagram
  autonumber
  CLI->>API: GET /v1/config?path=...&effective=true
  API->>CFG: load effective config (defaults + overrides)
  CFG->>CFG: resolve path
  CFG-->>API: value
  API-->>CLI: 200 ConfigValue
```

## 3. Поток: set value

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

## 4. Поток: remove value

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

## 5. Ошибки

- При ошибке валидации состояние не меняется и API возвращает `400`.
- При ошибке записи на диск или rename in-memory конфиг не фиксируется.
- При старте движка конфиг загружается из JSON файла и мержится с дефолтами.
