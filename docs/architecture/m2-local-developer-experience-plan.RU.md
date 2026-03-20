# План M2: Local Developer Experience

Статус: принятая базовая реализационная схема (2026-03-16)

Этот документ определяет план реализации для **публичной/локальной** части
roadmap milestone M2 после отказа от implicit repo-layout guessing в пользу
**явных versioned alias files** и advisory workspace discovery.

План остается в публичной/open части roadmap. Он не возвращает private
Team/Shared sequencing или внутренние детали rollout backend.

## 1. Результат

M2 должен уменьшить трение при локальном onboarding и улучшить инструменты
воспроизводимости для пользователей `sqlrs`, работающих из репозитория на
рабочей станции разработчика.

Ожидаемый публичный результат:

- repo-tracked workflow recipes для типовых `plan`, `prepare` и `run` flow;
- явное разделение между versioned workflow definitions и local-only
  workspace configuration;
- advisory discovery tooling, предлагающий улучшения, но не участвующий в
  execution semantics;
- детерминированные локальные building blocks для последующих `diff`, Git-ref,
  provenance и cache explanation возможностей.

## 2. Ограничения

- Ранние M2-срезы должны оставаться local-first и CLI-led.
- Нельзя полагаться на execution-time guesswork.
- Versioned workflow definitions должны жить отдельно от `.sqlrs/config.yaml`.
- Aliases и runtime names должны оставаться разными сущностями.
- Пока engine API явно не нужен, предпочтение отдается CLI-only изменениям.
- Command syntax должен оставаться additive и explicit.

## 3. Утвержденный Порядок Срезов

Утвержден следующий порядок реализации:

1. file-based prepare aliases
2. run aliases, alias inspection и mixed `prepare ... run` composition
3. `discover --aliases`
4. generic discover analyzers
5. shared local input graph primitives
6. `sqlrs diff` в path mode
7. Git ref execution baseline
8. provenance и cache explain

Такой порядок дает раннюю публичную ценность и при этом держит будущую
Git-aware часть ограниченной и тестируемой.

## 4. PR-срезы

### 4.1 PR1: Базовый Срез File-Based Prepare Aliases

**Цель**: сделать repo-tracked prepare recipes исполняемыми без смешивания их с
local workspace config.

**Основной результат**:

- `sqlrs plan <prepare-ref>` резолвит `<prepare-ref>.prep.s9s.yaml` от
  текущего рабочего каталога
- `sqlrs prepare <prepare-ref>` резолвит тот же класс alias files
- поддерживается exact-file escape через trailing `.`
- runtime names остаются отдельными от alias refs

**Ожидаемая работа**:

- определить формат prepare alias files
- реализовать alias-ref resolution от текущего рабочего каталога
- резолвить file-bearing alias arguments относительно директории alias file
- добавить alias-mode dispatch для `plan` и `prepare`
- задокументировать взаимодействие с `--name`

**Ожидаемые тесты**:

- тесты alias-ref resolution
- тесты exact-file escape
- тесты валидации prepare aliases
- negative tests для missing files и kind/schema errors

**Вне scope**:

- run aliases
- discover
- diff
- Git refs

### 4.2 PR2: Run Aliases, Alias Inspection и Mixed Composition

**Цель**: завершить явную alias execution surface и заставить обычные
`prepare ... run` pipeline работать через raw и alias modes.

**Основной результат**:

- standalone `sqlrs run <run-ref> --instance <id|name>` резолвит
  `<run-ref>.run.s9s.yaml` от текущего рабочего каталога
- `prepare ... run ...` принимает смешанные raw/alias комбинации
- `sqlrs alias ls`
- `sqlrs alias show <ref>`
- `sqlrs alias validate [<ref>]`

**Ожидаемая работа**:

- определить формат run alias files
- добавить run alias-mode dispatch
- разрешить смешивание raw/alias стадий в composite `prepare ... run`
- запретить `--instance`, если target instance уже выбран предшествующим
  `prepare`
- реализовать alias listing/show/validation commands
- сохранить raw `run:<kind>` mode рядом с alias mode

**Ожидаемые тесты**:

- тесты run alias resolution
- тесты composite grammar для смешанных raw/alias стадий
- тесты alias inspection commands
- тесты валидации kind-specific constraints
- negative tests для missing `--instance`, conflicting composite `--instance`
  и wrong alias type

**Вне scope**:

- discover analyzers
- diff
- Git refs

### 4.3 PR3: `discover --aliases`

**Цель**: помочь авторам репозитория bootstrap-ить explicit alias files без
зависимости execution path от эвристик.

**Основной результат**:

- `sqlrs discover --aliases` анализирует workspace и сообщает candidate alias files
- команда advisory и read-only по умолчанию

**Ожидаемая работа**:

- добавить семейство команд `discover`
- реализовать analyzer `--aliases`
- определить стабильный human и JSON output для findings
- сохранить жесткое разделение между discovery и execution semantics

**Ожидаемые тесты**:

- тесты выбора analyzers
- тесты candidate detection
- тесты JSON finding shape
- regression tests, подтверждающие, что `plan/prepare/run` не fallback-ятся к discovery

**Вне scope**:

- generic discover analyzers
- write mode для unrelated workspace files
- diff

### 4.4 PR4: Generic Discover Analyzers

**Цель**: превратить `discover` в общий advisory workflow для local repository
  hygiene и cache-friendly shaping.

**Основной результат**:

- базовый analyzer framework для нескольких selectors
- стартовые non-alias analyzers, например:
  - `--gitignore`
  - `--vscode`
  - `--prepare-shaping`

**Ожидаемая работа**:

- добавить analyzer registration и selection rules
- где это уместно, определить shared finding structure
- сохранить analyzer-specific semantics явными

**Ожидаемые тесты**:

- тесты multi-analyzer selection
- per-analyzer finding tests
- negative tests для incompatible write/update modes, если они появятся

**Вне scope**:

- Git-ref workflow
- provenance

### 4.5 PR5: Shared Local Input Graph Primitives

**Цель**: зафиксировать единую детерминированную модель revision-sensitive
local inputs.

**Основной результат**:

- CLI умеет строить детерминированные ordered input graphs для поддерживаемых
  prepare flow
- та же модель переиспользуется для discover analyzers, `diff`, Git-ref mode,
  provenance и cache explanation

**Ожидаемая работа**:

- определить CLI-side types для context roots и ordered input entries
- реализовать `psql` include-graph closure building
- реализовать Liquibase changelog-graph closure building
- определить стабильные правила hashing и ordering

**Ожидаемые тесты**:

- тесты детерминированного порядка
- тесты обхода closure для `psql`
- тесты обхода changelog graph для Liquibase
- тесты стабильности hash на normalization cases

### 4.6 PR6: `sqlrs diff` в Path Mode

**Цель**: поставить первый user-visible Git-aware workflow без обязательного
доступа к Git objects.

**Основной результат**:

- `sqlrs diff --from-path/--to-path ...` работает для одного wrapped `plan:*`
  или `prepare:*` command, а также для обычной двухстадийной grammar
  `prepare ... run`

**Ожидаемая работа**:

- добавить dispatch команды `diff`
- отдельно парсить diff scope и wrapped command
- переиспользовать input-graph builders из PR5
- отдельно вычислять wrapped `prepare` и `run` фазы, если используется
  composite
- реализовать human и JSON rendering

**Ожидаемые тесты**:

- тесты парсинга аргументов
- path-mode compare tests для `plan:psql`
- path-mode compare tests для `prepare:lb`
- path-mode compare tests для mixed raw/alias `prepare ... run`
- тесты JSON shape

### 4.7 PR7: Git Ref Execution Baseline

**Цель**: позволить пользователю выполнять repository-aware workflow из Git
revision без изменения working tree.

**Основной результат**:

- bounded local `--ref` support
- `blob` mode с zero-copy cache lookup до extraction
- явный `worktree` fallback mode

**Ожидаемая работа**:

- Git ref resolution
- blob-mode input access
- worktree lifecycle handling
- понятные user-facing errors для non-Git и missing-object случаев

**Ожидаемые тесты**:

- тесты парсинга и разрешения refs
- blob-mode cache-hit tests
- worktree lifecycle tests
- failure tests для bad refs и missing objects

### 4.8 PR8: Provenance и Cache Explain

**Цель**: сделать repository-aware local workflows воспроизводимыми и объяснимыми.

**Основной результат**:

- provenance output для local alias/Git-ref workflows
- `sqlrs cache explain ...` для user-facing hit/miss diagnostics

**Ожидаемая работа**:

- определить provenance payload
- записывать input hashes и cache decisions
- реализовать human и JSON output для cache explain

**Ожидаемые тесты**:

- тесты provenance payload
- cache-explain tests для hit/miss
- тесты text/JSON rendering

## 5. Сквозные Правила

- Каждый slice должен нести самостоятельную пользовательскую ценность.
- Discovery остается advisory, если только явно не выбран определенный write mode.
- Execution commands никогда не должны зависеть от предыдущего `discover`.
- Alias refs должны оставаться детерминированными и зависеть от текущего
  рабочего каталога.
- File-bearing paths внутри alias files должны резолвиться от директории самого
  alias file.
- Names остаются runtime handles и не заменяют aliases.
- Если slice меняет command semantics, в том же PR нужно обновлять релевантные
  user guide и architecture/contract docs.

## 6. Явные Не-Цели Этого Плана

- sequencing rollout для Team/Shared backend
- PR automation или hosted Git integrations
- сама поставка IDE extension
- `sqlrs compare`
- широкая переработка command surface за пределами alias/discover/Git-aware roadmap

## 7. Definition of Done для M2

M2 можно считать завершенным для публичного/local roadmap, когда:

- repo-tracked prepare/run aliases существуют и исполнимы;
- `discover` дает полезный advisory analysis для local repositories;
- `sqlrs diff` существует в path mode;
- Git-ref execution поддерживается в ограниченном local baseline;
- provenance и cache explanation доступны для repository-aware flow;
- итоговая документация описывает shipped public behavior без опоры на private
  deployment assumptions.

## 8. Ссылки

- [`../roadmap.RU.md`](../roadmap.RU.md)
- [`../user-guides/sqlrs-aliases.md`](../user-guides/sqlrs-aliases.md)
- [`cli-contract.RU.md`](cli-contract.RU.md)
- [`git-aware-passive.RU.md`](git-aware-passive.RU.md)
- [`diff-component-structure.RU.md`](diff-component-structure.RU.md)
