# Внутреннее устройство sqlrs Engine (локальный профиль)

Область: внутренняя структура процесса `sqlrs` для локального деплоймента (MVP). Фокус на том, как обрабатываются запросы от CLI, как устроены snapshot/cache, и как оркестрируются Docker и psql.

## 1. Модель компонентов

```mermaid
flowchart LR
  API[REST API]
  CTRL["Контроллер prepare"]
  DEL["Контроллер удаления"]
  PLAN["Планировщик prepare"]
  EXEC["Исполнитель prepare"]
  CACHE[Клиент кэша состояний]
  SNAP[Менеджер снапшотов]
  RUNTIME["Рантайм экземпляров (Docker)"]
  ADAPTER["Адаптер системы скриптов"]
  INST["Менеджер экземпляров"]
  CONN["Трекинг подключений"]
  STORE["Хранилище снапшотов (пути + метаданные)"]
  OBS[Телеметрия/Аудит]

  API --> CTRL
  API --> DEL
  CTRL --> PLAN
  CTRL --> EXEC
  CTRL --> CACHE
  CTRL --> SNAP
  CTRL --> RUNTIME
  CTRL --> ADAPTER
  CTRL --> INST
  CTRL --> OBS
  DEL --> INST
  DEL --> SNAP
  DEL --> STORE
  DEL --> CONN
  PLAN --> ADAPTER
  EXEC --> ADAPTER
  SNAP --> STORE
  INST --> STORE
  CACHE --> STORE
  RUNTIME --> STORE
  CONN --> RUNTIME
```

### 1.1 API-слой

- REST по loopback (HTTP/UDS); expose prepare jobs, операции с экземплярами, snapshots, cache и shutdown endpoints.
- expose endpoints удаления экземпляров и состояний с опциями recurse/force/dry-run.
- expose list endpoints для prepare jobs и tasks, а также удаление job.
- Prepare выполняется как async job; CLI следит за статусом/событиями и ждет завершения.
  `plan_only`-запросы возвращают список задач в статусе job.

### 1.2 Prepare Controller

- Координирует prepare job: plan steps, cache lookup, execute steps, snapshot states, создание экземпляра, persist metadata.
- Навязывает дедлайны и отмену; управляет дочерними процессами/контейнерами.
- Эмитит статусы и структурированные события для стрима в CLI.
- Отдает снимки jobs/tasks для list endpoints и обрабатывает удаление job.

### 1.3 Планировщик prepare

- Строит упорядоченный список шагов prepare из `psql` скриптов.
- Каждый шаг хешируется (специфично для системы скриптов) для cache key: `engine/version/base/step_hash/params`.
- На выходе цепочка шагов, а не head/tail; промежуточные состояния могут материализоваться для cache reuse.
- Вход prepare также дает стабильный fingerprint состояния:
  `state_id = hash(prepare_kind + base_image_id + normalized_args + normalized_input_hashes + engine_version)`.

### 1.4 Исполнитель prepare

- Выполняет один prepare шаг в instance через выбранный адаптер системы скриптов.
- Собирает структурированные логи/метрики для observability и cache planning.

### 1.5 Cache Client

- Общается с локальным state-cache индексом (SQLite) для lookup/store `key -> state_id`.
- Знает текущий корень state store; никогда не отдает наружу raw filesystem paths.

### 1.6 Snapshotter

- Абстрагирует CoW/copy стратегии (btrfs, VHDX+link-dest, rsync).
- Экспортирует `Clone`, `Snapshot`, `Destroy` для states и instances.
- Использует path resolver из State Store, чтобы найти корни `PGDATA`.

### 1.7 Instance Runtime

- Управляет DB-контейнерами через Docker (один контейнер на instance).
- Применяет монтирования от Snapshotter, задает лимиты ресурсов, default statement timeout.
- Возвращает connection info контроллеру.

### 1.8 Адаптер системы скриптов

- Дает общий интерфейс для систем скриптов (сейчас только `psql`).
- Каждый адаптер реализует planning, execution и правила хеширования шагов.
- Liquibase планируется как внешний CLI (host binary или Docker runner); накладные расходы измеряются и оптимизируются при необходимости.

### 1.9 Менеджер экземпляров

- Ведет mutable экземпляры, производные от immutable states.
- Создает эфемерные экземпляры и возвращает DSN.
- Управляет жизненным циклом экземпляров (ephemeral) и метаданными TTL/GC.

### 1.10 Контроллер удаления

- Строит дерево удаления для экземпляров и состояний.
- Проверяет правила безопасности (активные подключения, потомки, флаги).
- Выполняет удаление при разрешении; ответы идемпотентны.

### 1.11 Трекинг подключений

- Отслеживает активные подключения на уровне экземпляров.
- Использует introspection в БД (например, `pg_stat_activity`) по расписанию.
- Используется для продления TTL и проверок удаления.

### 1.12 State Store (Paths + Metadata)

- Разрешает корень хранилища (`~/.cache/sqlrs/state-store` или override).
- Владеет metadata DB (SQLite WAL) и layout путей (`engines/<engine>/<version>/base|states/<uuid>`).
- Пишет `engine.json` в state directory (endpoint + PID + auth token + lock) для discovery со стороны CLI.
- Хранит `parent_state_id` для поддержки иерархии состояний и рекурсивного удаления.

### 1.13 Telemetry/Audit

- Эмитит метрики: cache hit/miss, planning latency, instance bind/exec durations, snapshot size/time.
- Пишет audit events для prepare jobs и cache mutations.

## 2. Потоки (local)

### 2.1 Prepare Flow

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant API as Engine API
  participant CTRL as Prepare Controller
  participant CACHE as Cache
  participant SNAP as Snapshotter
  participant RT as Runtime (Docker)
  participant ADAPTER as Script Adapter
  participant INST as Instance Manager

  CLI->>API: start prepare job
  API-->>CLI: job_id
  CLI->>API: watch status/events
  API->>CTRL: enqueue job
  CTRL->>ADAPTER: plan steps
  CTRL->>CACHE: lookup(key)
  alt cache hit
    CACHE-->>CTRL: state_id
  else cache miss
    CACHE-->>CTRL: miss
  end
  CTRL->>SNAP: clone base/state for instance
  SNAP-->>CTRL: instance path
  CTRL->>RT: start DB container with mount
  RT-->>CTRL: endpoint ready
  loop for each step
    CTRL->>CACHE: lookup(key)
    alt cache hit
      CACHE-->>CTRL: state_id
    else cache miss
      CTRL->>ADAPTER: execute step
      ADAPTER-->>CTRL: logs/status per step
      CTRL->>SNAP: snapshot new state
      CTRL->>CACHE: store key->state_id
    end
  end
  CTRL->>INST: create instance
  INST-->>CTRL: instance_id + DSN
  CTRL-->>API: status updates / events
  CTRL->>SNAP: teardown instance (or keep warm by policy)
  API-->>CLI: terminal status
```

- Отмена: контроллер отменяет prepare job, прерывает активную работу с БД, завершает стрим со статусом `cancelled`.
- Таймауты: контроллер ограничивает wall-clock; `statement_timeout` задается на шаг.

### 2.2 Plan-only Flow

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant API as "Engine API"
  participant CTRL as "Контроллер prepare"
  participant ADAPTER as "Адаптер скриптов"
  participant CACHE as Cache

  CLI->>API: start prepare job (plan_only=true)
  API-->>CLI: job_id
  CLI->>API: watch status/events
  API->>CTRL: enqueue job
  CTRL->>ADAPTER: plan steps
  CTRL->>CACHE: lookup(key)
  CACHE-->>CTRL: hit/miss
  CTRL-->>API: tasks + cached flags
  API-->>CLI: terminal status (tasks included)
```

- Plan-only job не выполняет шаги и не создает экземпляр.
- Статус job содержит список задач, когда он доступен.

### 2.3 Delete Flow

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant API as "Engine API"
  participant DEL as "Контроллер удаления"
  participant STORE as "Metadata Store"
  participant INST as "Менеджер экземпляров"
  participant CONN as "Трекинг подключений"
  participant SNAP as Snapshotter

  CLI->>API: DELETE /v1/states/{id}?recurse&force&dry_run
  API->>DEL: build deletion tree
  DEL->>STORE: load state + descendants
  DEL->>INST: list instances per state
  DEL->>CONN: get active connection counts
  alt blocked
    DEL-->>API: DeleteResult (blocked)
  else allowed
    DEL->>INST: remove instances
    DEL->>SNAP: remove snapshots
    DEL->>STORE: delete metadata
    DEL-->>API: DeleteResult (deleted)
  end
  API-->>CLI: 200/409 + DeleteResult
```


### 2.4 List Jobs/Tasks Flow

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant API as "Engine API"
  participant CTRL as "Контроллер prepare"

  CLI->>API: GET /v1/prepare-jobs
  API->>CTRL: snapshot jobs
  CTRL-->>API: job entries
  API-->>CLI: job list

  CLI->>API: GET /v1/tasks?job=...
  API->>CTRL: snapshot tasks
  CTRL-->>API: task entries
  API-->>CLI: task list
```

### 2.5 Job Delete Flow

```mermaid
sequenceDiagram
  participant CLI as CLI
  participant API as "Engine API"
  participant CTRL as "Контроллер prepare"

  CLI->>API: DELETE /v1/prepare-jobs/{id}?force&dry_run
  API->>CTRL: delete job request
  alt blocked without force
    CTRL-->>API: DeleteResult (blocked)
  else allowed
    CTRL-->>API: DeleteResult (deleted)
  end
  API-->>CLI: 200/409 + DeleteResult
```

## 3. Конкурентность и процессная модель

- Один процесс engine; очередь job с небольшим пулом воркеров (настраиваемо).
- Один активный экземпляр на job; несколько job могут выполняться параллельно при наличии ресурсов.
- Лок: per-store lock, чтобы два экземпляра engine не писали в один store.

## 4. Персистентность и discovery

- `engine.json` в state directory: `{ pid, endpoint, socket_path|port, startedAt, lockfile, instanceId, authToken, version }`.
- Cache metadata и реестр states живут в SQLite под корнем state store.
- Другой персистентности нет; engine в остальном disposable.

## 5. Обработка ошибок

- Все долгие операции возвращают job id; ошибки фиксируются как terminal state с причиной и логами.
- Cache writes идемпотентны по `state_id`; частичные snapshots помечаются failed и не переиспользуются без явной ссылки.
- Если Docker/psql недоступны, API возвращает понятные ошибки; CLI их показывает и завершает с ненулевым кодом.

## 6. Точки эволюции

- Заменить Runtime на k8s executor без изменения формы API.
- Усилить локальный auth для multi-user/shared host (не-MVP).
- Подключить remote/shared cache client за тем же интерфейсом.
