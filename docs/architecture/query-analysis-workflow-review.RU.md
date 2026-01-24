# Стандартный workflow: эксперименты с запросами (baseline + Liquibase + sqlrs)

Этот документ описывает **workflow с минимальным вмешательством** для запуска сложного `SELECT` по нескольким "версиям" базы (варианты схем/индексов, объема данных) и сбора результатов, планов и базовых метрик.

Ключевая идея: продолжать использовать привычные инструменты (`psql`, `pgbench`, скрипты). Единственное, что меняется с `sqlrs`, - **скорость и воспроизводимость получения нужного состояния БД**.

Примечание: примеры с Liquibase ниже носят ориентировочный характер; текущий локальный prepare поддерживает только `prepare:psql`.

---

## 1. Цель пользователя

- Запускать один и тот же запрос на нескольких состояниях БД, например:
  - `0-1-3`: схема v1 + небольшой seed + upgrade
  - `0-2-3`: схема v1 + большой seed + upgrade
  - плюс варианты индексов или конфигураций
- Собрать одно или несколько из:
  - результат запроса (опционально)
  - план запроса (EXPLAIN)
  - статистику выполнения (EXPLAIN ANALYZE, buffers)
  - бенчмарк-тайминги (повторные прогоны)

---

## 2. Базовый сценарий A: без Liquibase (ручные скрипты)

### 2.1 Подготовить два экземпляра БД (две "версии")

Типичная практика: запустить несколько контейнеров Postgres на разных портах.

```bash
# instance A: small seed
docker run -d --name pg_small -e POSTGRES_PASSWORD=postgres -p 5433:5432 postgres:17

# instance B: large seed
docker run -d --name pg_large -e POSTGRES_PASSWORD=postgres -p 5434:5432 postgres:17
```

Затем применить скрипты:

```bash
# schema
psql "postgresql://postgres:postgres@localhost:5433/postgres" -f schema.sql
psql "postgresql://postgres:postgres@localhost:5434/postgres" -f schema.sql

# seed variants
psql "postgresql://postgres:postgres@localhost:5433/postgres" -f seed_small.sql
psql "postgresql://postgres:postgres@localhost:5434/postgres" -f seed_large.sql

# optional upgrade
psql "postgresql://postgres:postgres@localhost:5433/postgres" -f upgrade_v2.sql
psql "postgresql://postgres:postgres@localhost:5434/postgres" -f upgrade_v2.sql

# ensure planner statistics are comparable
psql "postgresql://postgres:postgres@localhost:5433/postgres" -c "ANALYZE;"
psql "postgresql://postgres:postgres@localhost:5434/postgres" -c "ANALYZE;"
```

### 2.2 Запуск запроса и снятие планов

```bash
QFILE=query.sql

# plan only
psql "postgresql://postgres:postgres@localhost:5433/postgres" \
  -v ON_ERROR_STOP=1 -X -f - <<'SQL'
\timing on
EXPLAIN (VERBOSE, COSTS, BUFFERS, SETTINGS, FORMAT TEXT)
\i query.sql
SQL

# plan + runtime stats
psql "postgresql://postgres:postgres@localhost:5433/postgres" \
  -v ON_ERROR_STOP=1 -X -f - <<'SQL'
\timing on
EXPLAIN (ANALYZE, VERBOSE, BUFFERS, SETTINGS, FORMAT TEXT)
\i query.sql
SQL
```

### 2.3 Бенчмарк (повторные прогоны)

Два распространенных паттерна:

1. простой цикл с warmup:

   ```bash
   # warmup
   for i in 1 2 3; do psql "$DSN" -c "\i query.sql" >/dev/null; done
 
   # measure
   for i in 1 2 3 4 5; do time psql "$DSN" -c "\i query.sql" >/dev/null; done
   ```

2. кастомный скрипт pgbench:

   ```bash
   pgbench "$DSN" -f query.sql -T 30 -c 1 -j 1
   ```

### 2.4 Общие best practices (ручной режим)

- Делайте состояния воспроизводимыми через скрипты и детерминированные seeds.
- Запускайте `ANALYZE` после загрузки данных; иначе планы могут отличаться по несущественным причинам.
- Решите, сравниваете ли вы **warm-cache** или **cold-cache**.
  - warm-cache: прогреть, затем измерять
  - cold-cache: перезапускать экземпляр БД между прогонами
- Зафиксируйте важные настройки при сравнении производительности (например, `work_mem`, `jit`).

---

## 3. Базовый сценарий B: с Liquibase (стандартная практика команды)

### 3.1 Представление вариантов через contexts/labels

Типично:

- schema + upgrades: применяются всегда
- seed variants: выбираются через `context-filter` или `label-filter`

Запуск двух состояний через две отдельные БД:

```bash
# small
liquibase \
  --url="jdbc:postgresql://localhost:5433/postgres" \
  --username=postgres --password=postgres \
  --changelog-file=db.changelog.xml \
  --context-filter=small \
  update

# large
liquibase \
  --url="jdbc:postgresql://localhost:5434/postgres" \
  --username=postgres --password=postgres \
  --changelog-file=db.changelog.xml \
  --context-filter=large \
  update
```

Далее выполняются те же шаги, что и в разделе 2.2 / 2.3, через `psql` и `pgbench`.

### 3.2 Общие best practices (Liquibase)

- Держите варианты seed за contexts/labels.
- Разбивайте большие seeds на предсказуемые единицы (или генерируйте детерминированно).
- Сохраняйте согласованную версию Postgres и настройки при сравнении производительности.

---

## 4. С sqlrs (минимальное вмешательство)

### 4.1 Базовая идея

С `sqlrs` **строка подключения к БД выводится из логической спецификации состояния**, а не из заранее поднятого экземпляра.

_Спецификация состояния_ (StateSpec) объединяет:

- базовый engine и версию (например, `postgres:17`)
- рецепт подготовки (например, Liquibase changelog + фильтры)

`sqlrs` гарантирует, что логическое состояние существует (через кэш или материализацию), и затем предоставляет **стандартный DSN / connection URL** пользовательскому инструменту.

Наличие или отсутствие кэша влияет **только на время старта**, но не на семантику.

Примечание: примеры с `sqlrs run --from/--prepare` описывают **будущий UX**.
Текущий MVP использует композитный вызов
`sqlrs prepare:psql ... run:psql ...` и не поддерживает флаги `--from` или
`--prepare`.

---

### 4.2 Текущий MVP: композитный prepare + run workflow

Пример: бенчмарк с небольшим seed (MVP, только `prepare:psql`)

```bash
sqlrs prepare:psql -- -f schema.sql -f seed_small.sql \
  run:pgbench -- -f query.sql -T 30 -c 1 -j 1
```

Пример: EXPLAIN ANALYZE с большим seed

```bash
sqlrs prepare:psql -- -f schema.sql -f seed_large.sql \
  run:psql -- -v ON_ERROR_STOP=1 -X \
    -c "EXPLAIN (ANALYZE, VERBOSE, BUFFERS, SETTINGS) $(cat query.sql)"
```

Семантика:

- `prepare:psql ...` определяет и материализует состояние.
- Создается эфемерный экземпляр из этого состояния.
- `run:<kind>` выполняется против этого экземпляра и инжектирует DSN.

### 4.3 Будущее: единый workflow prepare + run

Вместо явного создания экземпляров или глобальных тегов, рекомендуемый workflow — это **одна команда**, которая:

1. описывает, как подготовить состояние БД;
2. сразу запускает пользовательский инструмент на этом состоянии.

Пример: бенчмарк с небольшим seed

```bash
sqlrs run \
  --from postgres:17 \
  --prepare liquibase \
  --changelog-file=db.changelog.xml \
  --context-filter=small \
  -- \
  pgbench -f query.sql -T 30 -c 1 -j 1
```

Пример: EXPLAIN ANALYZE с большим seed

```bash
sqlrs run \
  --from postgres:17 \
  --prepare liquibase \
  --changelog-file=db.changelog.xml \
  --context-filter=large \
  -- \
  psql -v ON_ERROR_STOP=1 -X \
    -c "EXPLAIN (ANALYZE, VERBOSE, BUFFERS, SETTINGS) $(cat query.sql)"
```

Семантика:

- `--prepare liquibase ...` определяет логическую историю БД (StateSpec).

- sqlrs разрешает этот StateSpec в конкретное неизменяемое состояние (cache hit или build).

- Создается эфемерный экземпляр из этого состояния.

- Обернутая команда получает обычное подключение к БД через env/DSN.

### 4.4 Альтернатива: явный DSN или экспорт окружения

Для инструментов, которые сложно обернуть, sqlrs может отдать детали подключения напрямую:

```bash
sqlrs url \
  --from postgres:17 \
  --prepare liquibase \
  --changelog-file=db.changelog.xml \
  --context-filter=small
```

или:

```bash
eval "$(sqlrs env \
  --from postgres:17 \
  --prepare liquibase \
  --changelog-file=db.changelog.xml \
  --context-filter=small)"

pgbench -f query.sql -T 30 -c 1 -j 1
```

Это сохраняет совместимость с существующими скриптами и интерактивным workflow.

### 4.5 Best practices (sqlrs)

- Рассматривайте параметры подготовки как **истинную идентичность** состояния БД.
- Избегайте глобальных человеко-читаемых тегов без явной необходимости; предпочитайте inline StateSpec.

- Используйте эфемерные instances для измерений, чтобы избежать "загрязнения" между прогонами.
- Держите одинаковый engine/version и релевантные настройки во всех сравнениях.
- Для больших seeds полагайтесь на cache pinning, а не на ручные теги.

## 5. Чеклист хорошего эксперимента

- [ ] Один и тот же engine + version
- [ ] Одинаковые настройки, влияющие на planner/runtime (зафиксировать их)
- [ ] `ANALYZE` после загрузки данных
- [ ] Политика прогрева определена (warm vs cold)
- [ ] Собранные артефакты сохранены (stdout, explain, timings)

---

## 6. Примечания

- Liquibase отвечает за _подготовку_ состояний БД.
- `sqlrs` отвечает за _материализацию и повторное использование_ множества состояний.
- Инструменты анализа запросов в MVP остаются теми же.
