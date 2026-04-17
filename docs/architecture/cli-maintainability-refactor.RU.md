# Рефакторинг maintainability CLI

Статус: Accepted (2026-04-16)

## 1. Контекст

Публичная/local CLI-поверхность функционально стабильна и хорошо покрыта
тестами, но внутренняя форма накопила технический долг из-за нескольких
feature-by-feature срезов:

- `frontend/cli-go/internal/app` теперь смешивает command dispatch,
  dependency wiring, command-context resolution, prepare/run orchestration,
  cleanup reporting и platform-specific behavior;
- `frontend/cli-go/internal/app/app.go` по-прежнему опирается на package-level
  mutable function variables для тестов и dispatch seams;
- владение alias YAML split-ится между `internal/app` и `internal/alias`;
- в `internal/discover` уже есть generic analyzer orchestrator, но общий
  `Report` / `Finding` всё ещё живёт внутри aliases analyzer implementation;
- `plan` и `prepare` уже делят множество концептов, но всё ещё выполняются
  через параллельные command pipelines с дублирующимся branching.

Этот документ определяет следующий проход рефакторинга только для CLI. Он
инкрементальный и сохраняет публичный command contract.

## 2. Цели

- Сохранить текущие CLI syntax, output shape, exit-code behavior и local
  runtime semantics.
- Уменьшить концентрацию ответственности в `internal/app`, `internal/alias` и
  `internal/discover`.
- Создать более узкие внутренние seams, чтобы следующие фичи не требовали
  синхронных правок в нескольких oversized файлах.
- Двигаться маленькими, тестируемыми и reviewable PR.

## 3. Не-цели

- Без новых top-level command-ов, flag-ов или output format-ов.
- Без изменений schema alias-файлов в этом проходе.
- Без переписывания на новый framework или большого package move одним шагом.
- Без попытки сразу вычистить все oversized файлы.

## 4. Текущий долг, который нужно закрыть

### 4.1 `internal/app` владеет слишком многими слоями

Сейчас package одновременно держит:

- CLI parse + top-level dispatch;
- command-context resolution;
- command-specific option building;
- alias-backed prepare/run orchestration;
- ref-backed prepare binding;
- prepared-instance cleanup reporting;
- WSL и host-command plumbing.

Первый заметный симптом — количество package-level mutable hooks, существующих
только ради тестов. Более глубокая проблема в том, что владение command flow
размазано по нескольким файлам без явной runner boundary.

### 4.2 Владение alias definition дублируется

Загрузка и базовая валидация prepare/run alias YAML сейчас существуют и в
`internal/app`, и в `internal/alias`.

Это означает, что schema validation, kind normalization и file-loading rules не
принадлежат одному каноническому слою, из-за чего будущие изменения alias
становятся рискованнее, чем должны быть.

### 4.3 Generic orchestration в `discover` всё ещё зависит от aliases-specific data

`internal/discover/generic.go` уже ведёт себя как shared analyzer orchestrator,
но общий `Report` / `Finding` всё ещё живёт в
`internal/discover/aliases.go` и всё ещё несёт aliases-specific fields.

Это мешает чистому росту generic analyzers и оставляет крупнейший analyzer file
владельцем общих discovery contracts.

### 4.4 `plan` и `prepare` всё ещё дублируют один stage pipeline

Текущие code path для `plan` и `prepare` дублируют:

- image resolution;
- prepare-arg parsing и validation;
- input binding;
- ref-backed binding и cleanup;
- branching по kind для `psql` и Liquibase.

Из-за этого любое cross-cutting изменение prepare-oriented flow оказывается
дороже, чем должно быть.

## 5. План рефакторинга

Работа намеренно разбивается на этапы.

### 5.1 PR1: runner boundary для `internal/app`

Ввести явную runner/dependency boundary вокруг top-level CLI dispatch.

Статус: реализовано в текущей ветке.

Основной результат:

- `app.Run(args)` становится thin facade;
- package-level mutable hooks, используемые top-level dispatch, заменяются на
  явные runner dependencies;
- command dispatch становится проще тестировать без мутации package state.

Это выбранный первый PR.

### 5.2 PR2: canonical alias domain

Перенести alias definition loading, normalization и schema validation за одного
канонического владельца в `internal/alias`, а `internal/app` заставить работать
с этой доменной моделью вместо прямого чтения YAML.

Статус: реализовано в текущей ветке.

### 5.3 PR3: cleanup generic discover model

Отделить generic discovery report types от aliases analyzer-а, а затем
разбить aliases analyzer на более узкие внутренние фазы: scan, score,
validate и suppress.

### 5.4 PR4: shared stage pipeline для plan/prepare

Свести текущие execution flow `plan` и `prepare` к одному internal stage
pipeline с mode-specific rendering и execution только там, где они реально
отличаются.

### 5.5 PR5: optional follow-up для platform-heavy кода

После очистки предыдущих границ разбить крупные platform-specific flow вроде
`init_wsl.go` на более узкие helper-ы без изменения поведения.

## 6. Дизайн PR1

### 6.1 Scope

PR1 намеренно узкий.

Входит:

- top-level command dispatch в `internal/app`;
- текущий facade `app.Run`;
- high-level dependency seams, используемые dispatch и cleanup reporting;
- создание command-context как runner collaborator.

Явно вне scope PR1:

- deep prepare-binding hooks вроде `bindPreparePsqlInputsFn` и
  `bindPrepareLiquibaseInputsFn`;
- platform-specific shell/host command hooks внутри WSL и Btrfs helper-ов;
- cleanup владения alias schema;
- cleanup discover data model;
- объединение plan/prepare pipeline.

Смысл PR1 — сначала поставить верхнюю границу, а не решить всё использование
package state одним патчем.

### 6.2 Целевая форма

`internal/app` сохраняет публичную точку входа:

```go
func Run(args []string) error
```

Внутри package появляется явный runner object:

- `runnerDeps`
  - владеет parser-ом, lookup cwd, command-context resolution и top-level
    command handlers/collaborators, нужными для dispatch;
- `runner`
  - владеет одним invocation и исполняет текущую command sequence через эти
    зависимости;
- `newDefaultRunner()`
  - один раз собирает production dependencies;
- `Run(args)`
  - делегирует в default runner.

Правила владения:

- `runner` — только orchestration слой;
- business logic пока остаётся в существующих command helper-ах;
- тесты создают runner с явными stub dependencies вместо мутации package globals.

### 6.3 Ожидаемые изменения файлов

PR1 должен в основном затронуть:

- `frontend/cli-go/internal/app/app.go`
- `frontend/cli-go/internal/app/command_cleanup.go`
- `frontend/cli-go/internal/app/discover.go`
- связанные `internal/app/*_test.go`, которые сейчас patch-ят top-level hooks

Крупного behavioral move в этом срезе не ожидается; основное изменение —
кто владеет dependency wiring.

### 6.4 Критерии успеха

PR1 считается успешным, если:

- top-level dispatch больше не зависит от mutable package globals в `app.go`;
- тесты top-level dispatch могут stub-ить зависимости через одну runner
  boundary;
- command behavior, output и текущий public CLI contract не меняются.

## 7. Тест-план для PR1

Первый implementation slice должен добавить или обновить тесты вокруг новой
runner boundary.

Ожидаемые тесты:

1. `TestRunnerUsesParserAndReturnsHelpWithoutDispatch`
2. `TestRunnerSkipsCommandContextForInitAndDiff`
3. `TestRunnerBuildsCommandContextOnceForContextualCommands`
4. `TestRunnerRejectsCompositePrepareRefBeforeRunDispatch`
5. `TestRunnerRoutesAliasAndDiscoverThroughInjectedHandlers`
6. `TestRunnerCleansPreparedInstanceThroughInjectedCleanup`
7. `TestRunUsesDefaultRunnerDependencies`

Точный split по test files не важен, но первый PR должен доказать, что
top-level dispatch тестируется без мутации package-level function state.

## 8. Правило для follow-up

PR1 не должен opportunistically втягивать работу из PR2-PR4. Если изменение
нужно только для centralize alias ownership, redesign discover payload-ов или
слияния plan/prepare pipelines, оно относится к следующему срезу.

## 9. Дизайн PR2

### 9.1 Scope

PR2 по-прежнему намеренно узкий.

Входит:

- один канонический execution-facing owner для alias definition в
  `internal/alias`;
- общая YAML loading и schema validation для prepare и run alias-ов;
- filesystem-aware loading, чтобы ref-backed prepare alias execution использовал
  тот же loader с переданной файловой системой;
- интеграция `internal/app` через alias package вместо локальных duplicate YAML
  struct-ов и loader-ов.

Явно вне scope PR2:

- изменение alias command syntax или invocation grammar;
- замена alias argument parser-ов в `internal/app`;
- слияние alias path resolution в один shared execution/inspection API там, где
  это может поменять текущие command-specific error-ы;
- redesign payload-а для alias create;
- cleanup discover model;
- объединение plan/prepare pipeline.

Смысл PR2 — сначала сделать `internal/alias` единственным владельцем alias
definition, не расползаясь на все остальные alias-related вопросы.

### 9.2 Целевая форма

`internal/alias` становится владельцем execution-facing alias definition.

Ожидаемые additions:

- `alias.Definition`
  - общие загруженные alias metadata:
    - `Class`
    - `Kind`
    - `Image`
    - `Args`
- один общий loader API, экспортируемый из `internal/alias`, например:
  - `LoadTarget(target Target) (Definition, error)`
  - `LoadTargetWithFS(target Target, fs inputset.FileSystem) (Definition, error)`

Правила владения:

- `internal/alias` владеет YAML loading, kind normalization и schema check-ами
  для execution-facing alias файлов;
- `internal/app` продолжает владеть command-shape parsing для alias invocation,
  например для `prepare`, `plan` и `run`;
- `internal/app` может оставить command-specific path-resolution wrapper-ы в
  этом PR, если они всё ещё нужны для сохранения текущих user-facing error-ов;
- `CheckTarget` в `internal/alias` должен переиспользовать тот же shared loader,
  а не держать отдельные duplicate prepare/run definition struct-ы.

После PR2:

- `internal/app` больше не должен определять duplicate execution-only alias
  types вроде отдельных YAML struct-ов `prepareAlias` / `runAlias`;
- `internal/app` больше не должен владеть duplicate функциями
  `loadPrepareAlias*` / `loadRunAlias`.

### 9.3 Почему path resolution пока остаётся split

`internal/alias` уже владеет generic target resolution для inspection и create,
но в `internal/app` всё ещё остаются command-specific wrapper-ы для execution,
потому что текущее public behavior включает command-specific wording ошибок и
ref-backed filesystem path для prepare alias-ов.

Слияние path resolution и definition loading одним шагом слишком расширит
рефакторинг и повысит риск случайных CLI-facing regressions. Поэтому PR2
централизует alias definition сначала, а полное выравнивание execution-path
resolution остаётся на потом, если оно вообще понадобится.

### 9.4 Критерии успеха

PR2 считается успешным, если:

- в `internal/alias` существует один канонический alias-definition loader;
- alias inspection и alias execution читают одни и те же prepare/run schema
  rules;
- ref-backed prepare alias execution всё ещё умеет загружать alias через
  supplied filesystem;
- `internal/app` больше не дублирует YAML execution model для alias файлов;
- public CLI syntax, output и exit-code behavior не меняются.

## 10. Тест-план для PR2

Второй implementation slice должен добавить или обновить тесты вокруг shared
alias-definition owner.

Ожидаемые тесты:

1. `TestLoadTargetPrepareDefinition`
2. `TestLoadTargetRunDefinition`
3. `TestLoadTargetWithFSSupportsPrepareAliasesInRefContexts`
4. `TestLoadTargetRejectsInvalidPrepareSchema`
5. `TestLoadTargetRejectsInvalidRunSchema`
6. `TestCheckTargetReusesSharedAliasDefinitionLoader`
7. `TestResolvePrepareAliasWithOptionalRefLoadsDefinitionsViaAliasPackage`
8. `TestRunAliasExecutionLoadsDefinitionsViaAliasPackage`

Точный split по test files не важен, но PR должен доказать, что prepare/run
execution и alias inspection больше не держат независимые YAML schema loader-ы.
