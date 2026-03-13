# sqlrs diff — структура компонентов

В этом документе описана **архитектурная проработка** команды `sqlrs diff` после
контракта CLI и user guide: какие компоненты есть, кто кого вызывает и где они
расположены. Это следующий шаг после дизайна в
[`docs/user-guides/sqlrs-diff.md`](../user-guides/sqlrs-diff.md) и
[`docs/architecture/cli-contract.md`](cli-contract.md).

## 1. Область и допущения

- **Первый срез**: diff выполняется целиком в CLI; нового API у engine нет. CLI
  разрешает две стороны (ref или path), строит списки файлов локально по тем же
  правилам замыкания, что и основной CLI, сравнивает их и выводит результат.
- **Без вызова engine** в первом срезе: построение списка файлов реализуется в
  CLI (или за счёт повторного использования логики engine через будущий API
  «только список файлов», если он появится). Это позволяет использовать diff без
  запущенного engine.
- **Единица развёртывания**: только CLI (например, `frontend/cli-go`). Изменения
  в `backend/local-engine-go` для первого среза не требуются.

## 2. Компоненты и ответственность

| Компонент | Ответственность | Кто вызывает |
|-----------|-----------------|--------------|
| **Обработчик команды diff** | Разбор области diff и оборачиваемой команды; оркестрация: разрешение области → построение списка файлов (обе стороны) → сравнение → вывод. Преобразование ошибок в коды выхода. | `internal/app` (диспетчер команд) |
| **Разрешитель области (scope)** | По `--from-ref`/`--to-ref` или `--from-path`/`--to-path` выдаёт два **контекста**. Каждый контекст — корень, от которого читаются файлы: либо дерево Git по ref (blob или временный worktree), либо локальная директория. | Обработчик команды diff |
| **Построитель списка файлов** | Для одного контекста и заданного **kind** (например, psql, lb) плюс аргументы команды строит **замыкание файловых входов**: упорядоченный список (path, содержимое или хеш). Точка входа и правило замыкания зависят от kind (см. таблицу в user guide). | Обработчик команды diff (вызывается дважды: для from- и to-контекста) |
| **Компаратор diff** | По двум спискам файлов (from, to) вычисляет Added / Modified / Removed (по пути и опционально по хешу содержимого). Опционально применяет `--limit` и `--include-content`. | Обработчик команды diff |
| **Рендер diff** | Преобразует результат diff в человеко-читаемый текст или JSON в соответствии с глобальным `--output`. | Обработчик команды diff |

## 3. Построитель списка файлов по kind

Построитель списка файлов — **ключевая абстракция**, разная для каждого kind. У
каждого kind одна точка входа и одно правило замыкания.

| Kind | Точка входа из аргументов | Правило замыкания | Реализация |
|------|---------------------------|-------------------|------------|
| **prepare:psql** / plan:psql | `-f <file>` (и `-f -`) | От каждого файла из `-f` рекурсивно добавлять все файлы, на которые ссылаются `\i`, `\ir`, `\include`, `\include_relative`. | `PsqlClosureBuilder` (или общий с engine при повторном использовании) |
| **prepare:lb** / plan:lb | `--changelog-file <path>` | От файла changelog добавлять все файлы, на которые ссылается граф changelog (include, includeAll и т.д.). Граф задаёт Liquibase. | `LbChangelogClosureBuilder` |
| **run:psql** (будущее) | `-f <file>` (только файловые входы) | Как у psql: замыкание по `\i`/`\include` от `-f`. | Повторное использование psql-построителя |

Обработчик выбирает построитель по kind оборачиваемой команды (например,
`plan:psql` → psql-построитель, `prepare:lb` → lb-построитель). Аргументы, не
зависящие от ревизии (например, `-c`, `--image`), на список файлов не влияют;
построитель использует только аргументы, задающие файловые входы.

## 4. Поток вызовов

```text
1. app (диспетчер команд)
   → обнаруживает глагол "diff"
   → разбирает глобальные флаги, затем область diff (--from-ref/--to-ref или
     --from-path/--to-path), затем оборачиваемую команду (напр. plan:psql -- -f ./x.sql)
   → вызывает cli.RunDiff(scope, wrappedCommand, globalOptions)

2. RunDiff
   → scopeResolver.Resolve(scope)  →  (fromContext, toContext)
   → fileListBuilder.Build(fromContext, kind, wrappedCommandArgs)  →  fromList
   → fileListBuilder.Build(toContext, kind, wrappedCommandArgs)    →  toList
   → comparator.Compare(fromList, toList, options)                  →  diffResult
   → renderer.Render(diffResult, outputFormat, options)             →  stdout
   → возврат кода выхода
```

Поведение разрешителя области:

- **Режим ref**: разрешить каждый ref в коммит/дерево; при `--ref-mode blob`
  читать файлы через Git blobs (например, `git show ref:path`); при `worktree`
  создать временный worktree и передать его корень как контекст; опционально
  `--ref-keep-worktree` оставляет worktree для отладки.
- **Режим path**: каждый путь — корень контекста напрямую (без Git).

Построитель списка файлов выбирается по `kind` (psql или lb). Он получает корень
контекста и разобранные аргументы оборачиваемой команды и возвращает список
(path, содержимое или хеш) в детерминированном порядке.

## 5. Предлагаемое размещение пакетов (CLI)

Всё перечисленное относится к кодовой базе CLI (например, `frontend/cli-go`).

| Пакет | Содержимое |
|-------|------------|
| `internal/app` | Добавить diff в граф команд; разбор области diff и оборачиваемой команды; вызов `cli.RunDiff`. |
| `internal/cli` | `RunDiff`, типы опций diff; оркестрация resolver → builder → comparator → renderer. Опционально: рендер human/JSON для вывода diff. |
| `internal/diff` (новый) | `ScopeResolver` (режимы ref и path). Интерфейс `FileListBuilder`; `PsqlClosureBuilder`, `LbChangelogClosureBuilder`. `Comparator` (Compare). `Renderer` (human + JSON), если не в `internal/cli`. Типы: `Context`, `FileList`, `DiffResult`. |

Вариант: оставить `internal/diff` минимальным (только resolver, comparator, типы),
а построители замыканий вынести в `internal/cli/diff` или переиспользовать логику
со стороны engine через небольшой адаптер, если engine в будущем предоставит
хелпер «список файлов для этих аргументов».

## 6. Владение данными и жизненный цикл

- **Область (from/to ref или path)**: разбирается один раз за вызов; не
  персистится.
- **Контексты**: in-memory представление двух корней (например, путь worktree
  или аксессор к blob). Временный worktree при необходимости создаётся до
  построения списков файлов и удаляется после (если не задан `--ref-keep-worktree`).
- **Списки файлов**: in-memory на время выполнения команды; кэш не используется.
  Каждый список — упорядоченное множество (path, содержимое или хеш).
- **Результат diff**: in-memory; передаётся рендеру и затем отбрасывается.
  Постоянного состояния diff не вводит.

## 7. Схема зависимостей

```mermaid
flowchart TB
  APP["internal/app <br/> (диспетчер команд)"]
  RUN_DIFF["internal/cli <br/> RunDiff"]
  RESOLVER["internal/diff <br/> ScopeResolver"]
  BUILDER["internal/diff <br/> FileListBuilder<br/>(psql, lb)"]
  COMPARE["internal/diff <br/> Comparator"]
  RENDER["internal/diff <br/> Renderer<br/>(или cli)"]

  APP --> RUN_DIFF
  RUN_DIFF --> RESOLVER
  RUN_DIFF --> BUILDER
  RUN_DIFF --> COMPARE
  RUN_DIFF --> RENDER
  RESOLVER -.->|fromContext, toContext| BUILDER
  BUILDER -.->|fromList, toList| COMPARE
  COMPARE -.->|DiffResult| RENDER
```

## 8. Ссылки

- User guide: [`docs/user-guides/sqlrs-diff.md`](../user-guides/sqlrs-diff.md)
- Контракт CLI: [`docs/architecture/cli-contract.md`](cli-contract.md) (секция 3.9)
- Git-aware passive (сценарий P3): [`docs/architecture/git-aware-passive.RU.md`](git-aware-passive.RU.md)
- Структура компонентов CLI: [`cli-component-structure.RU.md`](cli-component-structure.RU.md)
