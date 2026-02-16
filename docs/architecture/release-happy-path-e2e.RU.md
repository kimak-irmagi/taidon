# Release-gated happy-path E2E

## Scope

Этот документ задает MVP-валидацию релизов для локального профиля:

- контракт релизных бандлов;
- happy-path E2E gating в GitHub Actions;
- flow взаимодействия компонентов от RC до GA;
- внутреннюю структуру компонентов для CLI, engine и CI services.

Цель дизайна: пользователи должны получать те же бинарники, которые прошли E2E.

---

## Ограничения и принципы

- Валидируем артефакты, а не состояние исходников.
- Сравнение результатов должно быть детерминированным, с нормализацией
  нестабильных полей.
- Happy-path сценарии должны быть короткими и воспроизводимыми.
- Кросс-платформенные релизные бандлы должны сохраняться.
- Предпочитаем `build once -> test -> promote` вместо пересборки при публикации.

---

## Контракт релизных бандлов

Текущие релизные бандлы:

- `linux/amd64` (`.tar.gz`)
- `linux/arm64` (`.tar.gz`)
- `windows/amd64` (`.zip`)
- `darwin/amd64` (`.tar.gz`)
- `darwin/arm64` (`.tar.gz`)

Для каждого таргета релиз должен содержать:

- архив: `sqlrs_<version>_<os>_<arch>.<ext>`
- checksum: `sqlrs_<version>_<os>_<arch>.<ext>.sha256`

Также нужен дополнительный manifest-артефакт:

- `release-manifest.json` со списком таргетов, checksums, `workflow run id` и
  `commit SHA`.

---

## Каталог happy-path сценариев

Happy-path сценарии берутся из `examples/`.

Release-blocking сценарии для MVP:

- `hp-psql-chinook`: prepare из `examples/chinook/prepare.sql`, затем run `examples/chinook/queries.sql`.
- `hp-psql-sakila`: prepare из `examples/sakila/prepare.sql`, затем run `examples/sakila/queries.sql`.

Расширенные non-blocking сценарии:

- `hp-psql-flights-smoke`: минимальный prepare+query для flights.
- `hp-lb-jhipster`: Liquibase flow из `examples/liquibase/jhipster-sample-app`
  (зависит от runner/tooling).

Метаданные сценариев хранятся в manifest:

- `test/e2e/release/scenarios.json`

---

## Flow взаимодействия компонентов

1. Maintainer пушит RC tag `vX.Y.Z-rc.N`.
2. `build_rc` компилирует `sqlrs` и `sqlrs-engine` для всех таргетов и
   упаковывает бандлы.
3. `build_rc` публикует архивы, checksums и `release-manifest.json` как workflow
   artifacts.
4. `e2e_happy_path` скачивает RC-артефакты и выполняет матрицу сценариев в чистых
   runner-ах.
5. Каждый прогон нормализует вывод и сравнивает с golden snapshots.
6. `publish_rc` создает/обновляет pre-release и прикладывает валидированные
   артефакты, если обязательные E2E прошли.
7. Maintainer пушит GA tag `vX.Y.Z`.
8. `promote_ga` забирает RC-артефакты, валидирует provenance manifest
   (`release-manifest.json` `source_sha` должен совпадать с commit SHA GA-тега),
   проверяет checksums и публикует финальный релиз без пересборки.

Любой сбой на этапе валидации должен блокировать promotion.

---

## Внутренняя структура компонентов

### Deployment unit CLI (`frontend/cli-go`)

- `cmd/sqlrs`: entrypoint процесса.
- `internal/cli`: parsing/dispatch команд `init`, `prepare`, `run`, `plan`,
  `rm`, `status`.
- `internal/app`: orchestration, lifecycle локального engine, wiring config/IO.
- `internal/client`: HTTP transport к локальному engine.

Ответственности в E2E:

- выполнять сценарии так же, как их запускают пользователи;
- формировать стабильные text/JSON outputs для golden-сравнения.

Data ownership:

- in-memory: состояние выполнения команд;
- persistent: state-файлы в workspace и результаты в E2E temp dirs.

### Deployment unit engine (`backend/local-engine-go`)

- `cmd/sqlrs-engine`: bootstrap процесса и wiring зависимостей.
- `internal/httpapi`: API слой для CLI.
- `internal/prepare`, `internal/run`, `internal/deletion`: workflow services.
- `internal/store/sqlite`: persistent metadata store.
- `internal/runtime`, `internal/snapshot`, `internal/statefs`, `internal/dbms`:
  runtime/state логика.

Ответственности в E2E:

- выполнять реальный prepare/run lifecycle для happy-path сценариев;
- сохранять и отдавать логи/переходы состояний для assertions и диагностики.

Data ownership:

- in-memory: координация jobs/tasks и runtime state;
- persistent: sqlite metadata, state directories, snapshot/state files в workspace
  runner-а.

### Deployment unit CI service (GitHub Actions + scripts)

- `.github/workflows/release-local.yml`: pipeline сборки и упаковки релизов.
- planned split/extension workflow:
  - `build_rc` и публикация артефактов;
  - `e2e_happy_path` выполнение матрицы;
  - `publish_rc` публикация pre-release;
  - `promote_ga` публикация финального релиза.
- harness scripts:
  - `scripts/e2e-release/run-scenario.mjs`
  - `scripts/e2e-release/normalize-output.mjs`
  - `scripts/e2e-release/compare-golden.mjs`
  - `scripts/e2e-release/smoke-bundle.mjs`
  - `scripts/e2e-release/create-release-manifest.mjs`

Ответственности в E2E:

- изолировать прогон сценариев в чистом workspace runner-а;
- собирать логи, нормализованные outputs и diffs в artifacts;
- enforce policy блокировки публикации релиза.

Data ownership:

- in-memory: runtime state отдельных jobs в workflow;
- persistent: workflow artifacts, release assets и golden-файлы в репозитории.

---

## Стратегия runner-ов для 3 платформ

Целевое состояние (строгое full E2E на 3 платформах):

- full happy-path E2E на Linux, Windows и macOS перед GA promotion.

Переходный MVP-режим (если на hosted runner-ах нет нужных runtime prerequisites):

- Linux full E2E является blocking.
- Windows/macOS выполняют проверку бандла и smoke команд.
- Windows/macOS full E2E становится blocking после появления self-hosted runner-ов
  с нужным runtime.

Так сохраняется скорость релизов при понятном пути к строгому 3-платформенному gating.

---

## Сравнение и диагностика

Pipeline golden-сравнения:

1. выполнить сценарий и сохранить raw outputs;
2. нормализовать нестабильные поля;
3. сравнить normalized outputs с committed golden snapshots.

При падении обязательно выгружать:

- raw outputs;
- normalized outputs;
- unified diffs;
- engine и workflow logs;
- scenario manifest и метаданные окружения.

---

## Утвержденные решения

- Для текущей задачи используется поэтапный gating:
  Linux full happy-path E2E blocking; Windows/macOS выполняют smoke checks.
- Следующий шаг сразу после текущей задачи:
  перейти на full blocking на Linux, Windows и macOS.
- MVP release-blocking сценарии:
  `hp-psql-chinook` и `hp-psql-sakila`.
- Liquibase happy-path:
  non-blocking в текущей задаче; перевод в blocking после включения 3-platform
  full blocking.
