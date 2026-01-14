# Контракт CLI sqlrs (черновик)

Этот документ определяет **предварительный пользовательский контракт CLI** для `sqlrs`.
Он намеренно неполный и эволюционный, по духу близок к ранним версиям CLI `git` или `docker`.

Цели:

- закрепить стабильную _ментальную модель_ для пользователя,
- определить пространства команд и ответственности,
- направлять внутренние решения по API и UX.

---

## 0. Принципы дизайна

1. **Каноническое имя CLI**: `sqlrs`
2. **Интерфейс на подкомандах** (стиль git/docker)
3. **Явное состояние вместо неявной магии**
4. **Композиция команд** (plan -> apply -> run -> inspect)
5. **По умолчанию машинно-дружественный вывод** (JSON где уместно)
6. **Человеко-читаемые сводки** при интерактивном запуске

---

## 1. Высокоуровневая ментальная модель

С точки зрения пользователя `sqlrs` управляет:

- **states**: неизменяемые состояния БД, получаемые детерминированным процессом prepare
- **instances**: изменяемые копии states; все модификации БД происходят здесь
- **plans**: упорядоченные наборы изменений (например, Liquibase changesets)
- **runs**: исполнение планов, скриптов или команд

```text
state  --(materialize)-->  instance  --(run/apply)-->  new state
```

---

## 2. Группы команд (неймспейсы)

```text
sqlrs
|-- init
|-- plan
|-- migrate
|-- run
|-- state
|-- instance
|-- cache
|-- inspect
|-- tag
|-- config
`-- system
```

Не все группы требуются в MVP.

---

## 3. Основные команды (MVP)

### 3.1 `sqlrs init`

Актуальная семантика команды описана в user guide:

- [`docs/user-guides/sqlrs-init.md`](../user-guides/sqlrs-init.md)

---

### 3.2 `sqlrs plan`

Вычисляет план выполнения без применения изменений.

```bash
sqlrs plan [options]
```

Назначение:

- показать ожидаемые изменения
- посчитать хэш-горизонт
- показать потенциальные cache hits

Общие флаги:

```bash
--format=json|table
--contexts=<ctx>
--labels=<expr>
--dbms=<db>
```

Пример вывода (table):

```text
STEP  TYPE       HASH        CACHED  NOTE
0     changeset  ab12:cd     yes     snapshot found
1     changeset  ef34:56     no      requires execution
2     changeset  ?           n/a     volatile
```

---

### 3.3 `sqlrs migrate`

Применяет миграции пошагово с snapshotting и кэшированием.

```bash
sqlrs migrate [options]
```

Поведение:

- использует `plan`
- откатывается через cache там, где возможно
- материализует instances по мере необходимости
- делает снимок после каждого успешного шага

Важные флаги:

```bash
--mode=conservative|investigative
--dry-run
--max-steps=N
```

---

### 3.4 `sqlrs prepare` и `sqlrs run`

Актуальная семантика команд описана в user guides:

- [`docs/user-guides/sqlrs-prepare.md`](../user-guides/sqlrs-prepare.md)
- [`docs/user-guides/sqlrs-run.md`](../user-guides/sqlrs-run.md)

---

## 4. Управление состояниями и экземплярами

### 4.1 `sqlrs state`

Просмотр и управление неизменяемыми состояниями.

```bash
sqlrs state list
sqlrs state show <state-id>
```

---

### 4.2 `sqlrs instance`

Управление живыми экземплярами.

```bash
sqlrs instance list
sqlrs instance open <id>
sqlrs instance destroy <id>
```

---

## 5. Теги и обнаружение

### 5.1 `sqlrs tag`

Прикрепляет человеко-читаемые метаданные к состояниям.

```bash
sqlrs tag add <state-id> --name v1-seed
sqlrs tag add --nearest --name after-schema
sqlrs tag list
```

Теги:

- не меняют идентичность состояния
- влияют на eviction

---

## 6. Управление кэшем (advanced)

### 6.1 `sqlrs cache`

Просмотр и влияние на поведение кэша.

```bash
sqlrs cache stats
sqlrs cache prune
sqlrs cache pin <state-id>
```

---

## 7. Инспекция и отладка

### 7.1 `sqlrs inspect`

Инспекция планов, запусков и ошибок.

```bash
sqlrs inspect plan
sqlrs inspect run <run-id>
sqlrs inspect failure <state-id>
```

---

## 8. Конфигурация

### 8.1 `sqlrs config`

Просмотр и изменение конфигурации.

```bash
sqlrs config get
sqlrs config set key=value
```

---

## 9. Вывод и скриптинг

- Вывод по умолчанию: человеко-читаемый
- `--json`: машинно-читаемый
- Стабильные схемы для JSON-вывода

Подходит для CI/CD.

---

## 10. Источники входа (локальные пути, URL, удаленные загрузки)

Везде, где CLI ожидает файл или директорию, он принимает:

- локальный путь (файл или директория)
- публичный URL (HTTP/HTTPS)
- серверный `source_id` (предыдущая загрузка)

Поведение зависит от цели:

- локальный engine + локальный путь: передать путь напрямую
- удаленный engine + публичный URL: передать URL напрямую
- удаленный engine + локальный путь: загрузить в source storage (чанками) и передать `source_id`

Это держит `POST /runs` компактным и дает возобновляемые загрузки для больших проектов.

---

## 11. Совместимость и расширяемость

- Liquibase рассматривается как внешний планировщик/исполнитель
- CLI не раскрывает Liquibase internals напрямую
- Будущие бэкенды (Flyway, raw SQL, кастомные планировщики) вписываются в тот же контракт

---

## 12. Не-цели (для этого CLI контракта)

- Полный паритет с опциями Liquibase CLI
- Интерактивный TUI
- GUI bindings

---

## 13. Открытые вопросы

- Разрешать ли несколько prepare/run шагов на invocation? (см. user guides)
- Должен ли `plan` быть неявным в `migrate` или всегда явным?
- Сколько истории состояний показывать по умолчанию?
- Нужны ли флаги подтверждения для destructive операций?

---

## 14. Философия

`sqrls` (sic) — это не база данных.

Это **движок управления состояниями и выполнения** для баз данных.

CLI должен делать переходы состояний явными, инспектируемыми и воспроизводимыми.
