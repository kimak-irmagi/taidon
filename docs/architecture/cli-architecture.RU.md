# Архитектура CLI (локально и удаленно)

Этот документ описывает, как CLI `sqlrs` разрешает входы и общается с SQL Runner в локальном и shared-деплойментах, включая обработку путей/URL и загрузку источников.

## 1. Цели

- Поддержать одинаковый UX CLI для локальных и удаленных целей.
- Разрешать входы как локальные файлы или публичные URL везде, где ожидается "файл".
- Избегать больших тел запросов в `POST /runs`.
- Обеспечить возобновляемые, контент-адресуемые загрузки для удаленного выполнения.

## 2. Ключевые понятия

- **Target**: endpoint движка (локальный loopback или удаленный gateway).
- **Source**: содержимое проекта (скрипты, changelog, конфиги).
- **Source ref**: локальный путь, публичный URL или серверный `source_id`.
- **Source storage**: хранилище контента на стороне сервиса, ключи по хешам и `source_id`.

## 3. Правила разрешения

Для любого флага CLI, который ожидает файл или директорию, CLI принимает:

- **Локальный путь** (файл или директория).
- **Публичный URL** (HTTP/HTTPS).
- **Серверный source ID** (ранее загруженный bundle).

Матрица решений:

| Target | Input | Действие CLI |
|---|---|---|
| Local engine | Local path | передать путь в engine |
| Local engine | Public URL | передать URL в engine |
| Remote engine | Public URL | передать URL в engine |
| Remote engine | Local path | загрузить в source storage, затем передать `source_id` |

## 4. Потоки

### 4.1 Локальная цель, локальные файлы

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant ENG as Local Engine

  CLI->>ENG: POST /runs (path:/repo/sql, entry=seed.sql)
  ENG->>ENG: read files from local FS
  ENG-->>CLI: stream status/results
```

### 4.2 Удаленная цель, локальные файлы (сначала загрузка)

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant GW as Gateway
  participant SS as Source Storage
  participant RUN as Runner

  CLI->>GW: POST /sources (create session)
  GW-->>CLI: source_id + chunk size
  loop for each chunk
    CLI->>GW: PUT /sources/{id}/chunks/{n}
  end
  CLI->>GW: POST /sources/{id}/finalize (manifest)
  GW->>SS: store bundle
  CLI->>GW: POST /runs (source_id)
  GW->>RUN: enqueue run
  RUN-->>CLI: stream status/results
```

### 4.3 Удаленная цель, публичный URL

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant GW as Gateway
  participant RUN as Runner

  CLI->>GW: POST /runs (source_url=https://...)
  GW->>RUN: enqueue run
  RUN->>RUN: fetch URL into source cache
  RUN-->>CLI: stream status/results
```

## 5. Детали загрузки (remote)

- CLI режет файлы на чанки, считает хеши и загружает только отсутствующие чанки.
- Manifest сопоставляет пути файлов и хеши чанков; это дает rsync-style delta.
- `source_id` контент-адресуемый и может переиспользоваться между запусками.
- Большие загрузки возобновляемые; сбойные чанки можно повторить без рестарта.

## 6. Наличие Liquibase

- Если Liquibase доступен, CLI может запросить Liquibase-aware планирование на runner.
- Если Liquibase недоступен, CLI строит явный step-план (упорядоченный список скриптов) и передает его в запросе запуска.
- Одни и те же правила загрузки/разрешения применяются в обоих режимах.
