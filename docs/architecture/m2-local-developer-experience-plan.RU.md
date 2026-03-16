# План M2: Local Developer Experience

Статус: принятая базовая реализационная схема (2026-03-16)

Этот документ определяет план реализации для **публичной/локальной** части
milestone M2. Он предназначен как рабочий brief для инженера, который будет
реализовывать feature slices.

План сознательно остается в публичной/open части roadmap. Он не возвращает в
документ private Team/Shared sequencing или внутренние детали rollout backend.

## 1. Результат

M2 должен уменьшить трение при локальном onboarding и улучшить инструменты
воспроизводимости для пользователей `sqlrs`, работающих из репозитория на
рабочей станции разработчика.

Ожидаемый публичный результат:

- меньше обязательных флагов в типовых локальных сценариях;
- явные repository-aware workflow, управляемые намерением пользователя;
- воспроизводимое и объяснимое поведение cache для локальных запусков;
- поэтапная поставка через slices, каждый из которых полезен сам по себе.

## 2. Ограничения

- Первые M2-срезы должны оставаться local-first и CLI-led.
- Если срез не требует engine API, предпочтение отдается CLI-only изменениям.
- Локальные workflow не должны зависеть от hosted/shared инфраструктуры.
- Нужно переиспользовать уже принятое command shape для `sqlrs diff`.
- Текущая MVP command surface остается стабильной базой.

## 3. Порядок Срезов

Утвержден следующий порядок реализации:

1. repo/workspace conventions
2. shared local input graph primitives
3. первый публичный срез `sqlrs diff`
4. baseline для выполнения по git ref
5. provenance и cache explain

Такой порядок выбран, чтобы рано отдавать пользовательскую ценность и при этом
держать Git-aware features ограниченными и тестируемыми.

## 4. Определение Срезов

### 4.1 Срез 1: Repo/Workspace Conventions

**Цель**: уменьшить конфигурационное трение для типовых локальных layout
репозитория.

**Основной результат**:

- `sqlrs` умеет находить conventional prepare inputs по repo/workspace layout;
- локальные профили вроде `dev` и `test` имеют ясную документированную форму;
- публичная документация рекомендует один-два канонических layout репозитория.

**Ожидаемая работа**:

- определить repo layout conventions для `prepare:psql` и `prepare:lb`;
- определить config fallback order для обнаруживаемых prepare inputs;
- задокументировать profile conventions и границы secret handling для local use;
- обновить user guides так, чтобы happy path не начинался с ручной прокладки путей.

**Предполагаемая CLI surface**:

- новый top-level command не требуется;
- изменения должны выражаться через предсказуемые defaults или небольшие
  флаги на существующих `plan:*` / `prepare:*` командах.

**Ожидаемые тесты**:

- тесты config/profile resolution;
- тесты repo-layout discovery;
- negative tests для ambiguous/conflicting layout;
- примеры в user guides, соответствующие shipped behavior.

**Вне scope**:

- `sqlrs diff`
- `--ref`
- provenance
- cache explain

### 4.2 Срез 2: Shared Local Input Graph Primitives

**Цель**: зафиксировать единую детерминированную модель revision-sensitive inputs.

**Основной результат**:

- CLI умеет строить детерминированный ordered input graph для поддерживаемых
  local prepare flow;
- та же модель переиспользуется для `diff`, `--ref`, provenance и cache explain.

**Ожидаемая работа**:

- определить CLI-side types для context root, input entry, ordered file list и
  hash material;
- реализовать `psql` closure builder для `-f` + include graph;
- реализовать Liquibase changelog graph builder для `--changelog-file`;
- определить стабильные правила hashing и ordering;
- сохранить первую версию модели локальной и file-oriented.

**Граница реализации**:

- по умолчанию CLI-only;
- новый engine API допустим только если явно уменьшает дублирование без
  расширения публичного протокола.

**Ожидаемые тесты**:

- тесты детерминированного порядка;
- тесты closure graph для `psql`;
- тесты обхода changelog graph для Liquibase;
- тесты стабильности hash на кейсах с path normalization.

**Вне scope**:

- конечный UX команды `diff`
- доступ к Git refs
- вывод provenance

### 4.3 Срез 3: Первый Публичный Срез `sqlrs diff`

**Цель**: поставить первый видимый Git-aware workflow без обязательного доступа
к Git objects.

**Основной результат**:

- `sqlrs diff` работает в режиме `--from-path/--to-path`;
- команда оборачивает ровно один вызов `plan:*` или `prepare:*`;
- пользователь может сравнивать локальные деревья, сохраняя привычный syntax
  уже известных sqlrs-команд.

**Принятое command shape**:

```text
sqlrs diff --from-path <pathA> --to-path <pathB> <wrapped-command> [command-args...]
```

**Ожидаемая работа**:

- добавить dispatch команды `diff` в CLI;
- отдельно парсить diff scope и wrapped command;
- переиспользовать input graph builders из среза 2;
- реализовать human и JSON output;
- задокументировать error handling и exit-code behavior.

**Ожидаемые тесты**:

- тесты парсинга аргументов;
- path-mode compare tests для `plan:psql`;
- path-mode compare tests для `prepare:lb`;
- тесты JSON shape;
- negative tests для unsupported wrapped commands и mixed scope modes.

**Вне scope**:

- `--from-ref` / `--to-ref`
- wrapped `run:*`
- nested composite `prepare ... run`

### 4.4 Срез 4: Git Ref Execution Baseline

**Цель**: позволить пользователю выполнять prepare/run из Git revision без
изменения working tree.

**Основной результат**:

- `--ref` usable в local profile;
- `blob` mode поддерживает zero-copy cache lookup до extraction;
- `worktree` mode существует как явный fallback для кейсов, где он нужен.

**Ожидаемая CLI surface**:

```text
sqlrs ... --ref <git-ref> [--ref-mode blob|worktree] [--ref-keep-worktree]
```

**Ожидаемая работа**:

- определить repo root и разрешение Git refs;
- добавить blob-mode доступ к revision-sensitive input;
- добавить режим временного worktree;
- встроить cache lookup до file extraction в blob mode;
- определить понятные user-facing errors для non-Git directory, unresolved ref
  и missing objects.

**Ожидаемые тесты**:

- тесты парсинга и разрешения refs;
- blob-mode cache-hit tests;
- worktree lifecycle tests;
- failure tests для missing repo, bad ref и unsupported inputs.

**Вне scope**:

- hosted/shared repo access
- remote source upload integration
- `sqlrs compare`

### 4.5 Срез 5: Provenance и Cache Explain

**Цель**: сделать repository-aware local runs воспроизводимыми и объяснимыми.

**Основной результат**:

- пользователь может сохранить или вывести provenance record для выполненного flow;
- пользователь может запросить объяснение, почему cache lookup был быстрым,
  медленным или завершился miss.

**Ожидаемая CLI surface**:

```text
sqlrs ... --provenance write|print|both [--provenance-path <path>]
sqlrs cache explain ...
```

**Ожидаемая работа**:

- определить стабильный provenance payload для local execution;
- записывать key run context, input hashes, cache hit/miss decision и snapshot
  chain identifiers;
- реализовать простой `cache explain` report с miss reasons;
- добавить operator/user guides по интерпретации этой диагностики.

**Ожидаемые тесты**:

- тесты provenance payload;
- тесты text/JSON output;
- cache-explain tests для hit, partial hit и miss cases;
- regression tests для missing optional metadata.

**Вне scope**:

- PR automation
- hosted/shared cache introspection
- сравнение результатов между двумя полными execution context

## 5. Сквозные Правила

- Каждый срез должен нести самостоятельную пользовательскую ценность.
- Первые публичные срезы не должны зависеть от новых private/shared service.
- Command syntax должен оставаться additive и explicit.
- Если slice в основном состоит из CLI logic, предпочтение отдается
  детерминированным локальным тестам, а не тяжелому end-to-end покрытию.
- Если срез меняет command semantics, в том же PR нужно обновлять релевантные
  user guide и architecture/contract docs.

## 6. Явные Не-Цели Этого Плана

- sequencing rollout для Team/Shared backend
- PR automation или hosted Git integrations
- поставка IDE extension
- `sqlrs compare`
- широкая переработка command surface сверх нужд M2

## 7. Definition of Done для M2

M2 можно считать завершенным для публичного/local roadmap, когда:

- local repository conventions задокументированы и реализованы;
- `sqlrs diff` существует в первом публичном path-based срезе;
- `--ref` поддерживает ограниченный, но полезный local baseline;
- provenance и cache explanation доступны для local repository-aware flow;
- итоговая документация описывает shipped public behavior без опоры на private
  deployment assumptions.

## 8. Ссылки

- [`../roadmap.RU.md`](../roadmap.RU.md)
- [`cli-contract.RU.md`](cli-contract.RU.md)
- [`git-aware-passive.RU.md`](git-aware-passive.RU.md)
- [`diff-component-structure.RU.md`](diff-component-structure.RU.md)
- [`../adr/2026-03-09-git-diff-command-shape.md`](../adr/2026-03-09-git-diff-command-shape.md)
