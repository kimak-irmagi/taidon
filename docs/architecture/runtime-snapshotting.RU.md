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
- Кросс-нодовая репликация живых песочниц.
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

- **Linux (primary):** host-managed store на btrfs subvolumes (предпочтительно).
- **Windows / WSL2:** host-managed VHDX; использовать btrfs внутри при наличии, иначе fallback на `copy/link-dest`.

Runtime код не раскрывает конкретные пути: engine/adapter сам разрешает data dirs и передает mounts в runtime.

---

## 4. Модель контейнеров

### 4.1 Docker-based Runtime (локальный MVP)

- Docker используется как слой выполнения песочниц.
- Каждая песочница — один DB контейнер.
- Контейнеры краткоживущие и disposable.

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

- **btrfs subvolume snapshots** (Linux хосты; WSL2 при наличии btrfs внутри VHDX)

Результат:

- быстрые clone/snapshot
- дешевые ветвления
- хорошая локальная поддержка Linux

### 6.2 Fallback бэкенд

- Windows/WSL2 без btrfs: VHDX + `copy/link-dest` snapshotting
- Любой хост без CoW FS: рекурсивное копирование / rsync-based snapshotting

Используется при отсутствии CoW FS.

### 6.3 Плагинный интерфейс snapshotter

```text
Clone(base_state) -> sandbox_state
Snapshot(sandbox_state) -> new_state
Destroy(state_or_sandbox)
```

Варианты реализации:

- `BtrfsSnapshotter`
- `CopySnapshotter`
- (future) `ZfsSnapshotter`, `CsiSnapshotter`

---

## 7. Layout состояний

```text
state-store/
  engines/
    postgres/
      15/
        base/
        states/<uuid>/
      17/
        base/
        states/<uuid>/
  metadata/
    state.db
```

- Каждое состояние неизменяемо после создания.
- Песочницы используют writable clones состояний.

---

## 8. Режимы консистентности снапшотов

### 8.1 Crash-consistent (по умолчанию)

- Снапшот файловой системы без остановки БД.
- Полагается на crash recovery БД при следующем запуске.
- Самый быстрый и приемлемый для большинства сценариев.
- Дефолт для интерактивной разработки и учебных песочниц.

### 8.2 Clean snapshot (опционально)

- DB контейнер корректно останавливается.
- Снимается snapshot.
- Контейнер может быть перезапущен при необходимости.

Используется для:

- CI/CD
- строгой воспроизводимости
- Может запрашиваться политикой или флагом запуска.

---

## 9. Жизненный цикл песочницы

### 9.1 Создание

1. Выбрать `(engine, version)`.
2. Выбрать base state.
3. Клонировать base state в sandbox state.
4. Запустить контейнер с примонтированным sandbox state.

### 9.2 Выполнение

- Liquibase или пользовательские команды выполняются в песочнице.
- Подключения к БД проксируются или открываются по необходимости.

### 9.3 Snapshotting

После успешного шага:

1. Снять snapshot sandbox state в неизменяемое состояние.
2. Зарегистрировать состояние в cache index.

### 9.4 Teardown и cooldown

- После использования песочница входит в cooldown период.
- Если песочница переиспользуется, контейнер может быть оставлен warm.
- Иначе контейнер останавливается и песочница уничтожается.

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

- Docker + btrfs
- Postgres 15 & 17
- Локальный state store

### Phase 2

- Remote/shared cache
- ZFS / send-receive
- Pre-warmed pinned states

### Phase 3

- Kubernetes (CSI snapshots)
- MySQL engine adapter
- Cloud-scale eviction policies

---

## 12. Открытые вопросы

- Минимальная жизнеспособная абстракция для volume handles между snapshotter-ами?
- Насколько агрессивно делать snapshot для крупных seed-шагов?
- Политика cooldown по умолчанию для песочниц?
