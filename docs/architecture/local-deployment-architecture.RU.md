# Архитектура локального деплоймента (sqlrs)

Этот документ описывает, как `sqlrs` разворачивается и выполняется на **рабочей станции разработчика** в MVP.

Фокус:

- тонкий CLI
- эфемерный процесс engine
- взаимодействие с Docker и Liquibase

Этот документ намеренно не повторяет Liquibase-специфику; она описана в [`liquibase-integration.RU.md`](liquibase-integration.RU.md).
Внутренности engine описаны в [`engine-internals.RU.md`](engine-internals.RU.md).
Team/Cloud вариант описан в [`shared-deployment-architecture.RU.md`](shared-deployment-architecture.RU.md).

---

## 1. Цели

- Быстрый запуск CLI
- Минимальный постоянный след на хосте
- Кросс-платформенность (Linux, macOS, Windows через WSL2)
- Четкое разделение CLI и тяжелой runtime-логики
- Простая эволюция в daemon или team/cloud deployment

---

## 2. Высокоуровневая топология (MVP)

```mermaid
flowchart LR
  U[User]
  CLI[sqlrs CLI]
  ENG[sqlrs engine process]
  DOCKER[Docker Engine]
  DB[(DB Container)]
  LB[Liquibase]
  STATE[State dir (engine.json)]
  STORE[state store]

  U --> CLI
  CLI -->|spawn / connect| ENG
  ENG --> DOCKER
  ENG --> LB
  DOCKER --> DB
  ENG --> STORE
  CLI -. discovery .-> STATE
```

---

## 3. sqlrs CLI

### 3.1 Ответственности

- Парсить команды и флаги пользователя
- Работать с локальной файловой системой (конфиг проекта, пути)
- Находить или запускать локальный процесс engine
- Общаться с engine по HTTP через loopback или Unix socket
- Завершаться сразу после выполнения команды
- Опционально: pre-flight проверки (Docker доступен, state store доступен для записи), вывод endpoint/version для диагностики

CLI намеренно **тонкий** и без состояния.

### 3.2 Не-ответственности

- Нет логики оркестрации Docker
- Нет логики snapshotting
- Нет прямого выполнения Liquibase

---

## 4. Процесс Engine (эфемерный)

### 4.1 Характеристики

- Запускается по требованию CLI
- Работает как дочерний процесс (не системный daemon)
- Слушает локальный endpoint (loopback или socket)
- Управляет runtime-состоянием пока активен

### 4.2 Ответственности

- Оркестрация Docker-контейнеров
- Snapshotting и управление состояниями
- Cache rewind и eviction
- Оркестрация Liquibase (через провайдеры)
- Connection / proxy слой (если нужен)
- IPC/API для CLI и будущих IDE-интеграций

### 4.3 Жизненный цикл

- Спавнится по требованию
- Может жить короткий TTL после последнего запроса
- Автоматически завершается в простое
- Пишет endpoint/lock и auth token в `<StateDir>/engine.json`, чтобы последующие CLI могли переиспользовать процесс

Это избегает постоянных background-сервисов в MVP.

---

## 5. IPC: CLI <-> Engine

- **Транспорт/протокол**: REST по HTTP; только loopback. Unix domain socket на Linux/macOS; TCP loopback на Windows хосте с WSL forwarding. В локальном режиме TLS нет.
- **Discovery endpoint**:
  - CLI проверяет env var `TAIDON_ENGINE_ADDR`.
  - Иначе читает `<StateDir>/engine.json` (endpoint, socket path / TCP port, PID, instanceId, auth token).
  - Если не найден или устарел, CLI запускает новый engine; engine пишет `engine.json` при готовности.
- **Security**: запрет bind на не-loopback; auth token обязателен для non-health endpoint-ов; опора на права файлов (UDS) или loopback firewall; engine отказывает в соединениях с нелокальных адресов.
- **Versioning**: CLI отправляет свою версию; engine отклоняет несовместимую major; CLI может предложить апгрейд.

Ключевые endpoint-ы engine (логически):

- `POST /runs` - старт run (migrate/apply/run scripts)
- `POST /snapshots` - ручной snapshot
- `GET /cache/{key}` - cache lookup
- `POST /engine/shutdown` - опциональная мягкая остановка

### 5.1 Долгие операции: sync vs async режимы CLI

- **Async (fire-and-forget)**: CLI отправляет запрос, получает `run_id` и URL статуса, печатает его и выходит. Пользователь может позже опрашивать или стримить.
- **Sync (watch)**: CLI отправляет запрос, затем опрашивает/стримит статусы до терминального состояния; выводит прогресс/логи; завершаетcя кодом engine.
- Со стороны engine: все долгие операции асинхронны; даже sync-режим — это просто watch на стороне CLI поверх тех же REST endpoint-ов (`GET /runs/{id}`, `GET /runs/{id}/stream`).
- Флаги: например, `--watch/--no-watch` для переключения; default может быть `--watch` для интерактива и `--no-watch` для скриптов/CI.

---

## 6. Взаимодействие с Liquibase

Engine делегирует выполнение Liquibase _Liquibase provider_:

- system-installed Liquibase
- Docker-based Liquibase runner

Выбор провайдера и compatibility checks описаны в [`liquibase-integration.RU.md`](liquibase-integration.RU.md).

Engine потребляет **структурированные логи** Liquibase для наблюдаемости и контроля.

---

## 6. Взаимодействие с Docker

- Docker обязателен в MVP
- Engine управляет DB-контейнерами и Liquibase-контейнерами
- Все persistent data directories монтируются из host-managed хранилища
- Engine проверяет доступность Docker на старте; CLI выводит понятные ошибки, если Docker недоступен

На Windows:

- Docker работает внутри WSL2
- State store живет внутри Linux файловой системы

---

## 7. Особенности Windows / WSL2

- Engine и snapshotter работают внутри WSL2
- CLI может работать на Windows host или внутри WSL2
- Коммуникация через localhost forwarding
- Engine пишет `engine.json` внутри WSL state directory; Windows CLI читает его через `wslpath`/interop, чтобы подключиться через проброшенный TCP порт
- Snapshot backend может откатываться на copy-based стратегию

---

## 8. Путь эволюции

### Phase 1 (MVP)

- Эфемерный engine процесс
- Тонкий CLI
- Только локальный деплоймент

### Phase 2

- Опциональный постоянный локальный daemon (`sqlrsd`)
- Переиспользование warm sandbox
- IDE интеграции

### Phase 3

- Team-shared engine
- Remote cache
- Cloud-hosted control plane

---

## 9. Не-цели

- System-wide background service по умолчанию
- OS-specific installers или service managers
- Глубокий embedding Liquibase

---

## 10. Открытые вопросы

- Unix socket vs TCP loopback как дефолт IPC?
- Дефолтный TTL engine после последней команды?
- Нужно ли CLI авто-обновлять engine binary?
