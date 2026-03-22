# Test data for `sqlrs diff` (plan:psql)

Two directory trees that simulate "left" (baseline) and "right" (new) DB migration scripts.

## Layout

**left/** — базовая версия:
- `main.sql` — подключает `schema.sql` и `data.sql`
- `schema.sql` — таблицы `users`, `orders`
- `data.sql` — два пользователя, один заказ

**right/** — новая версия:
- `main.sql` — подключает `schema.sql`, `data.sql` и `\ir features/audit_trigger.sql`
- `schema.sql` — добавлены колонка `users.role` и таблица `audit_log`
- `data.sql` — добавлен третий пользователь (admin)
- `features/audit_trigger.sql` — новый файл (триггер аудита)

## Запуск diff

Из корня репозитория:

```bash
./scripts/sqlrs-diff/test-diff-db.sh
```

С предварительным запуском движка:

```bash
./scripts/sqlrs-diff/test-diff-db.sh --with-engine
```

Ожидаемый вывод diff: **1 added** (`features/audit_trigger.sql`), **3 modified** (`main.sql`, `schema.sql`, `data.sql`), **0 removed**.
