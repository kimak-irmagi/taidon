# Git-aware semantics: пассивные функции (CLI)

Статус: **дизайн на будущее**. Флаги вроде `--ref` и `--prepare` отсутствуют в
текущем MVP CLI. Сейчас MVP использует композитный вызов
`sqlrs prepare:psql ... run:psql ...`.

Цель: добавить git-aware возможности **без вмешательства в привычный процесс работы**.
Все функции в этом документе активируются **только по явной команде/флагу пользователя**
и не требуют настройки репозитория "под sqlrs".

## Принципы дизайна

- **Не угадываем намерения по репозиторию.** Контекст задает пользователь:
  `--prepare <path>` (файл/каталог миграций) и команда `sqlrs run -- <cmd>`.
- **Минимум побочных эффектов.** Где семантика команды сохраняется, предпочтительны
  чтения из Git-объектов, но совместимость важнее. В сценариях, где нужна полная
  файловая семантика, по умолчанию остаются временные `worktree`, оставляющие
  минимальные следы в `.git/worktrees`.
- **Сначала быстрый путь.** Сначала пытаемся найти готовое состояние в кэше sqlrs
  по хешам задействованных файлов. Если не нашли — строим.
- **Всё воспроизводимо.** Любое выполнение умеет сохранить манифест (provenance),
  чтобы повторить то же состояние 1:1.
- **Remote режим требует доступа к репозиторию.** Для `--ref` на удалённом раннере
  нужен server-side mirror (зеркало репозитория на стороне сервиса) или VCS-секреты;
  иначе CLI загружает исходники в `source storage` и передает `source_id` (см. [`sql-runner-api.md`](sql-runner-api.RU.md)).

---

## Сценарий P1. Repository-backed plan/prepare по git ref: `--ref`

Примечание по текущему публичному срезу: следующий принятый local CLI slice
уже по scope, чем эта общая future-design картина. Он добавляет bounded `--ref` только в
single-stage `plan` и `prepare`; `run --ref`, provenance и `cache explain`
остаются следующими шагами.

### Мотивация

Пользователь хочет вычислить prepare-oriented workflow **как в
коммите/ветке/теге**, не портя текущий рабочий каталог (грязное состояние,
открытые IDE, параллельные задачи).

### UX / CLI

Для следующего публичного local slice существующие command shapes `plan` /
`prepare` получают одно явное семейство stage-local флагов.

```bash
sqlrs plan --ref <git-ref> <prepare-alias>
sqlrs plan:psql --ref <git-ref> -- -f ./prepare.sql
sqlrs prepare --ref <git-ref> <prepare-alias>
sqlrs prepare:lb --ref <git-ref> -- update --changelog-file db/changelog.xml
```

Где `<git-ref>`: `HEAD`, `origin/main`, `abc1234`, `v1.2.3`, `refs/pull/123/head`
(если доступно локально).

Важная граница следующего публичного slice: он остаётся только локальным.
Семантика remote runner остаётся частью будущего дизайна.

Опции поведения:

- `--ref-mode worktree|blob` (по умолчанию `worktree`)
  - `worktree`: создать временный `git worktree` и удалить после завершения
    команды
  - `blob`: читать нужные файлы прямо из git-объектов (без извлечения всего репо)
- `--ref-keep-worktree` (для отладки: не удалять временный worktree)

### Алгоритм реализации (набросок)

1. Определить корень репозитория (если нет — ошибка `not a git repo`).
2. Разрешить `<git-ref>` в `commit/tree`.
3. Разрешить projected cwd вызывающего процесса внутри выбранной ревизии.
4. Привязать alias-backed или raw `plan` / `prepare` inputs в этом ref-backed
   context.
5. Собрать file-bearing inputs через shared kind-specific inputset layer.
6. Продолжить обычный flow `plan` или `prepare`.
7. Удалить временный worktree, если не задан `--ref-keep-worktree`.

---

## Сценарий P2. Zero-copy cache hit (ускорение без извлечения файлов)

### Мотивация

В больших репозиториях извлечение/checkout дорого, хотя sqlrs может уже иметь
нужное состояние по хешам миграций.

### UX / CLI

Включается автоматически при `--ref-mode blob` (или отдельным флагом):

```bash
sqlrs run --ref <ref> --ref-mode blob --prepare migrations/ -- <cmd>
```

Опционально:

- `--zero-copy=auto|off` отключить оптимизацию или оставить авто-режим

### Алгоритм реализации

1. В blob-mode получить список файлов и их blob-хеши через `git ls-tree`.
2. Посчитать ключи кэша по хешам.
3. Проверить кэш sqlrs **до** извлечения файлов.
4. Если попадание в кэш — не извлекать файлы на диск, сразу выдавать окружение.

Примечание: если нужные blob-объекты отсутствуют локально (`partial clone`/LFS),
потребуется `git fetch` или переход в `worktree` режим.

---

## Сценарий P3. Taidon-aware diff через оборачивание существующей sqlrs-команды: `diff`

### Мотивация

Обычный `git diff` показывает текст, но пользователю нужен ответ “что изменилось
**с точки зрения реальной sqlrs-команды**”, оставаясь синтаксически близким к
основному CLI.

### UX / CLI

Вместо отдельного mini-DSL вида `--prepare <path>` команда `diff` оборачивает
одну существующую content-aware команду sqlrs и вычисляет её в двух контекстах.

```bash
sqlrs diff --from-ref <refA> --to-ref <refB> plan:psql -- -f ./prepare.sql
sqlrs diff --from-ref <refA> --to-ref <refB> prepare:lb -- update --changelog-file db/changelog.xml
sqlrs diff --from-path <pathA> --to-path <pathB> prepare:psql -- -f ./prepare.sql
```

Правила:

- diff-опции идут до оборачиваемой команды;
- оборачиваемая команда сохраняет свой текущий синтаксис без изменений;
- глобальный `-v` остаётся флагом подробного вывода, а глобальный `--output` -
  переключателем text/json;
- file-bearing semantics оборачиваемой команды приходят из shared CLI-side
  компонента `inputset`, выбранного по kind;
- **Текущий CLI** (`frontend/cli-go`): ровно **один** wrapped-токен из
  `plan:psql`, `plan:lb`, `prepare:psql`, `prepare:lb`; сравниваются **closures
  файлов** (хеши), без engine. **Режим ref** по умолчанию — **`worktree`**
  ради полной семантики ФС; явный **`blob`** использует `git show` /
  `git ls-tree`.
- **Дизайн / потом**: composite `prepare ... run`, alias `prepare <ref>`; полные
  производные представления (планы, payload prepare).
- будущая standalone-поддержка `run:*` возможна только для file-backed входов,
  потому что inline-only вызовы могут не иметь payload, зависящего от ревизии.

Примеры composite-формы (**цель дизайна**; парсер CLI пока не всё поддерживает):

```bash
sqlrs diff --from-ref <refA> --to-ref <refB> prepare chinook run:psql -- -f ./queries.sql
sqlrs diff --from-path <pathA> --to-path <pathB> prepare:psql -- -f ./prepare.sql run smoke
```

Что именно сравнивает `diff`:

- **Сейчас:** один и тот же **resolved file graph + контент** для `plan:*` и
  `prepare:*` данного kind (psql vs lb).
- **Целевое:** `plan:*` → task plan; `prepare:*` → тела + граф; `run:*` → только
  файловые входы.

### Алгоритм реализации

1. Разобрать diff-scope (`from/to ref` или `from/to path`).
2. Разобрать оборачиваемую команду (сейчас: один `plan:*` или `prepare:*` токен).
3. Для каждой стороны независимо привязать оборачиваемую команду через shared
   компонент `inputset` для ее kind и собрать resulting file set в соответствии
   с ревизией или path-контекстом этой стороны.
4. **Сейчас:** построить списки файлов (closures) и сравнить Added / Modified /
   Removed.
   **Потом:** richer representations и per-phase вывод для `prepare ... run`.
5. Вывести human или JSON.

---

## Сценарий P4. Provenance (execution manifest)

### Мотивация

В реальных командах быстро возникает вопрос “что именно ты запускал?”. Нужен
артефакт, который можно приложить к багрепорту или повторить через месяц.

### UX / CLI

Автовключение по флагу или настройке:

```bash
sqlrs run --provenance write --provenance-path ./artifacts/provenance.json -- <cmd>
```

Режимы:

- `write` — записать файл
- `print` — вывести кратко в stdout
- `both`

Содержимое (минимум):

- timestamp (время запуска)
- git ref + commit (если задан `--ref`)
- `dirty/clean` (грязное/чистое состояние рабочего дерева)
- список входных файлов `--prepare` + хеши
- параметры окружения (`dbms.image`, важные флаги)
- цепочка снапшотов Taidon, использованные base/derived
- команда `sqlrs run -- <cmd>` + argv

### Алгоритм реализации

1. На старте собрать “контекст запуска”.
2. Во время `prepare` фиксировать цепочку снапшотов и ключевые решения
   (попадания/промахи кэша).
3. На выходе сериализовать JSON (и, опционально, текстовую сводку).

---

## Сценарий P5. Compare: один запрос на двух состояниях

### Мотивация

QA/разработчику нужно быстро сравнить результат/ошибку на двух версиях схемы
(например, base и PR).

### UX / CLI

```bash
sqlrs compare \
  --from-ref <refA> --from-prepare <path> \
  --to-ref <refB> --to-prepare <path> \
  -- psql -c "select * from flights limit 10"
```

Вывод:

- exit codes
- stderr/stdout (с лимитами)
- (опционально) diff результатов в табличном формате

Опции:

- `--diff text|json|table`
- `--timeout-ms 5000`
- `--max-rows 1000`

### Алгоритм реализации

1. Для `from-ref` и `to-ref` поднять окружение (с кэшем).
2. Выполнить одну и ту же команду.
3. Собрать результаты и сформировать сравнение.

---

## Сценарий P6. “Explain cache”: почему быстро/медленно

### Мотивация

Пользователь хочет понять, почему в этот раз было "долго": не было снапшота?
изменились хеши? другой движок?

### UX / CLI

```bash
sqlrs cache explain --ref <ref> --prepare <path>
```

Вывод:

- вычисленные хеши changeset-ов
- ближайшая опорная точка (если есть)
- причина промаха (нет снапшота / несовпадение движка/версии / отсутствует
  сегмент цепочки)

### Алгоритм реализации

1. Построить тот же ключ(и), что и для `migrate/run`.
2. Запросить индекс кэша.
3. Отрендерить объяснение.

---

## Минимальный MVP пассивных функций

1. bounded local `plan` / `prepare --ref` с `worktree` по умолчанию и явным
   `blob`
2. `sqlrs diff --from-ref/--to-ref <wrapped-command...>` для одной команды
   `plan:*` или `prepare:*`
3. provenance (write)
4. `cache explain` (простая версия)
