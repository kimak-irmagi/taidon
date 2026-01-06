# SQL-примеры

Этот каталог содержит **готовые к запуску SQL-датасеты**, используемые как примеры и базовые тесты.
Сами SQL-файлы **подтягиваются из внешних open-source проектов по требованию**
и **не полностью вендорятся** в репозиторий.

## Что включено

Сейчас поддерживаются такие датасеты:

- **Chinook (PostgreSQL)**  
  Небольшая, хорошо известная учебная БД для SQL-экспериментов.

- **Sakila (порт для PostgreSQL)**  
  PostgreSQL-адаптация классической базы Sakila.

- **Flights / Airlines (PostgresPro demo DB)**  
  Реалистичный датасет бронирований авиабилетов от Postgres Professional.

Все источники с лицензиями и ссылками перечислены в  
[`scripts/external/NOTICE.RU.md`](scripts/external/NOTICE.RU.md).

---

## Структура каталога

После загрузки структура выглядит так:

```text
examples/
  chinook/
    Chinook_PostgreSql.sql
  sakila/
    0-postgres-sakila-setup.sql
    1-postgres-sakila-schema.sql
    2-postgres-sakila-insert-data.sql
    3-postgres-sakila-user.sql
  flights/
    demo-small-en-20170815.sql

Перечисленные файлы в examples/ являются сгенерированными артефактами и могут быть перезаписаны при повторной загрузке.

## Загрузка SQL-файлов

SQL-файлы скачиваются с помощью скрипта:

```bash
pnpm install
pnpm fetch:sql
```

Он:

1. Скачивает SQL-файлы из источников
2. Проверяет целостность по sha256
3. Складывает результаты прямо в `./examples/...`

### Первичная фиксация контрольных сумм

Если для источника ещё нет checksum:

```bash
pnpm fetch:sql --write-sha
```

Это вычисляет и записывает sha256 в `scripts/external/manifest.yaml`.

### Воспроизводимый / CI режим

Для CI и воспроизводимых прогонов:

```bash
pnpm fetch:sql --lock
```

---

## Запуск примеров (чистый Docker)

### Chinook

```bash
docker run --rm -it \
  -e POSTGRES_PASSWORD=postgres \
  -v "$PWD/examples/chinook:/sql" \
  postgres:17 \
  psql -U postgres -f /sql/Chinook_PostgreSql.sql
```

### Sakila

```bash
docker run --rm -it \
  -e POSTGRES_PASSWORD=postgres \
  -v "$PWD/examples/sakila:/sql" \
  postgres:17 \
  bash -c "
    psql -U postgres -f /sql/0-postgres-sakila-setup.sql &&
    psql -U postgres -f /sql/1-postgres-sakila-schema.sql &&
    psql -U postgres -f /sql/2-postgres-sakila-insert-data.sql &&
    psql -U postgres -f /sql/3-postgres-sakila-user.sql
  "
```

### Flights (PostgresPro demo DB)

```bash
docker run --rm -it \
  -e POSTGRES_PASSWORD=postgres \
  -v "$PWD/examples/flights:/sql" \
  postgres:17 \
  psql -U postgres -f /sql/demo-small-en-20170815.sql
```

## Запуск через `sqlrs` (рекомендуется)

Эти примеры рассчитаны на использование в качестве `--prepare` для `sqlrs`.

Пример:

```bash
sqlrs \
  --from postgres:17 \
  --workspace ./sqlrs-work \
  --prepare examples/flights/demo-small-en-20170815.sql \
  -run -- psql -f examples/flights/queries.sql
```

## Примечания по обновлениям и воспроизводимости

- Источники могут эволюционировать - контрольные суммы предотвращают незаметные изменения.
- `examples/` рассматривается как производные данные, а не вручную поддерживаемый код.
- Чтобы обновить датасет из источника:
  1. Обновите URL / ревизию в `manifest.yaml`
  2. Выполните `pnpm fetch:sql --write-sha`
  3. Закоммитьте обновлённый manifest

## Лицензии

Каждый датасет распространяется по своей исходной лицензии.
Подробности и атрибуцию см. в [`scripts/external/NOTICE.RU.md`](scripts/external/NOTICE.RU.md).
