# Runtime Snapshotting: дизайн (MVP)

Этот документ фиксирует **архитектурные решения по runtime и snapshotting** для MVP `sqlrs`.
Фокус на **том, как состояния БД материализуются, снимаются в снапшоты, клонируются и переиспользуются** в рантайме, с понятной дорожкой эволюции.

---

## 1. Цели

- Делать **частые и дешевые snapshot** (после каждого шага миграции).
- Полностью контролировать все **персистентное runtime-состояние** (директории данных БД).
- Поддерживать **cache rewind** и быстрое ветвление.
- Избежать архитектурных предположений про один DB engine или версию.
- Построить MVP, который прост, отлаживаем и эволюционен.

---

## 2. Не-цели (MVP)

- Снапшотинг внутри БД или манипуляции на уровне WAL.
- Восстановление или продолжение mid-transaction выполнения.
- Кросс-нодовая репликация живых экземпляров.
- Полная multi-engine поддержка за пределами семейства Postgres.

---

## 3. Выбранная runtime стратегия

### 3.1 Path A: внешнее хранение персистентного состояния

Все **персистентное состояние БД** (например, `PGDATA`):

- принадлежит `sqlrs`, а не контейнерам;
- хранится на файловой системе хоста;
- монтируется в DB контейнеры во время выполнения.

Контейнеры — **stateless executors**.

### 3.2 Стратегия host-хранилища (по платформам)

- **Linux (primary):** снапшоттер выбирается по FS `SQLRS_STATE_STORE` (btrfs/zfs → CoW, иначе copy/reflink).
- **Windows:** когда выбран btrfs, engine запускается внутри WSL2; state store — host VHDX, смонтированный в WSL и отформатированный в btrfs. Иначе engine может работать на Windows host с copy-бэкендом.

Runtime код не раскрывает конкретные пути: engine/adapter сам разрешает data dirs и передает mounts в runtime.
Для локального engine корень state store по умолчанию `<StateDir>/state-store`, если не задан `SQLRS_STATE_STORE`.
В WSL `sqlrs init local --snapshot btrfs` устанавливает systemd mount unit; движок проверяет активность маунта перед работой со store.

---

## 4. Модель контейнеров

### 4.1 Docker-based Runtime (локальный MVP)

- Docker используется как слой выполнения экземпляров.
- Каждый экземпляр - один DB контейнер.
- Контейнеры могут оставаться запущенными после prepare (warm экземпляры) до
  решения оркестрации run об остановке.

### 4.2 Образы

Docker-образы содержат:

- DBMS binaries (например, Postgres)
- entrypoint и health checks

Образы **не содержат данных**.

### 4.3 Поддерживаемые движки (MVP)

| Engine   | Versions | Notes                                |
| -------- | -------- | ------------------------------------ |
| Postgres | 15, 17   | Same engine family, different majors |

Это валидирует:

- логику выбора engine/version,
- отсутствие hardcoded предположений.

MySQL явно отложен.

---

## 5. Base States ("нулевые" состояния)

### 5.1 Определение

**Base state** — неизменяемое представление инициализированного DB кластера.

Для Postgres:

- создается через `initdb`;
- содержит стандартные БД (`postgres`, templates).

Liquibase обычно работает на существующей БД; явный `CREATE DATABASE` опционален и не требуется для MVP.

### 5.2 Хранение

Base states хранятся как **CoW-способные файловые наборы** и рассматриваются как обычные неизменяемые состояния.

---

## 6. Бэкенд snapshotting

### 6.1 Основной бэкенд (MVP)

- **OverlayFS слои** (Linux хосты) для copy-on-write снапшотов.
- **btrfs subvolume snapshots** (Linux/WSL2) для блочного CoW.

### 6.2 Fallback бэкенд

- Любой хост без CoW-бэкенда: рекурсивное копирование (MVP).

### 6.3 Плагинный интерфейс StateFS

```text
Validate(state_store_root)
Clone(base_state) -> sandbox_state
Snapshot(sandbox_state) -> new_state
RemovePath(state_or_sandbox)
Capabilities() -> { requires_db_stop, supports_writable_clone, supports_send_receive }
```

Варианты реализации:

- `OverlayFSSnapshotter`
- `CopySnapshotter`
- `BtrfsSnapshotter`
- (future) `ZfsSnapshotter`, `CsiSnapshotter`

### 6.4 Политика выбора бэкенда

- Runtime выбирает StateFS по возможностям хоста и опциональному конфиг-override
  (например, `snapshot.backend`).
- Если предпочтительный backend недоступен, происходит **fallback** на `CopySnapshotter`.
- Выбранный backend фиксируется в метаданных состояния для совместимости GC/restore.

### 6.5 Инварианты StateFS

- **Неизменяемость:** состояние неизменно после `Snapshot`.
- **Идемпотентное удаление:** `RemovePath` безопасно вызывать повторно (включая удаление subvolume).
- **Writable clone:** `Clone` всегда создаёт writable sandbox, независимый от базы.
- **Non-destructive snapshot:** `Snapshot` не должен изменять sandbox.

---

## 7. Layout состояний

```text
<StateDir>/state-store/
  engines/
    postgres/
      15/
        base/
        states/<uuid>/
      17/
        base/
        states/<uuid>/
```

- Каждое состояние неизменяемо после создания.
- Песочницы используют writable clones состояний.
- Метаданные живут в `<StateDir>/state.db`.

---

## 8. Режимы консистентности снапшотов

### 8.1 DBMS-assisted clean snapshot (по умолчанию)

- СУБД ставится на паузу через коннектор (Postgres: `pg_ctl -m fast stop`), контейнер остается запущенным.
- Менеджер снапшотов делает snapshot.
- СУБД запускается обратно.

Используется для:

- CI/CD
- строгой воспроизводимости
- Может запрашиваться политикой или флагом запуска.

### 8.2 Crash-consistent (опционально)

- Снапшот файловой системы без координации с БД.
- Полагается на crash recovery БД при следующем запуске.

### 8.3 Граница ответственности за консистентность

Режим консистентности выбирается **оркестрацией** (политика prepare/run), а не StateFS.
StateFS лишь сообщает, нужна ли остановка БД, через `Capabilities().requires_db_stop`.
Если backend требует остановки, оркестрация **обязана** приостановить СУБД перед `Snapshot`.

---

## 9. Жизненный цикл экземпляра

### 9.1 Создание

1. Выбрать `(engine, version)`.
2. Выбрать base state.
3. Клонировать base state в instance state.
4. Запустить контейнер с примонтированным instance state.

### 9.2 Выполнение

- Liquibase или пользовательские команды выполняются в экземпляре.
- Подключения к БД проксируются или открываются по необходимости.

### 9.3 Snapshotting

После успешного шага:

1. Снять snapshot instance state в неизменяемое состояние.
2. Зарегистрировать состояние в cache index.

### 9.4 Teardown и cooldown

- После prepare контейнер остается запущенным, экземпляр записывается как warm.
  Оркестрация run решает, когда останавливать warm экземпляры.
- Если контейнер warm-экземпляра отсутствует, оркестрация run пересоздает его
  на основе сохраненной runtime-директории; отсутствие runtime-данных считается
  ошибкой (без пересборки state).

---

## 10. Безопасность multi-version

Применяются правила:

- Engine и version всегда **явные параметры**.
- В runtime коде нет предположений о дефолтном engine/version.
- Все пути и образы разрешаются через engine adapters.

Это предотвращает случайную привязку к одному engine или layout.

### 10.1 Модель процесса engine

- CLI тонкий; он авто-спавнит/общается с процессом engine (локальный daemon), владеющим state store, snapshotting и жизненным циклом контейнеров.
- Engine adapter скрывает filesystem layout (`PGDATA` roots, state store paths) и предоставляет mount specs в runtime; вызывающие стороны не работают с raw paths.

---

## 11. Путь эволюции

### Phase 1 (MVP)

- Docker + OverlayFS/btrfs
- Postgres 15 & 17
- Локальный state store

### Phase 2

- Remote/shared cache
- ZFS / send-receive
- Pre-warmed pinned states
- Автоматизация Windows-установки для WSL2+btrfs

### Phase 3

- Kubernetes (CSI snapshots)
- MySQL engine adapter
- Cloud-scale eviction policies

---

## 12. Открытые вопросы

- Минимальная жизнеспособная абстракция для volume handles между реализациями StateFS?
- Насколько агрессивно делать snapshot для крупных seed-шагов?
- Политика cooldown по умолчанию для экземпляров?
