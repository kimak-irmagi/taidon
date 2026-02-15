# Рефакторинг Prepare Manager

Status: Реализовано (2026-02-12)

## 1. Контекст

В `internal/prepare` сейчас очень крупный `Manager` со смешанными зонами ответственности:

- Координация жизненного цикла джобов (`Submit`, `Recover`, `runJob`, события, retention).
- Оркестрация выполнения задач (runtime, запуск `psql/liquibase`, создание instance).
- Оркестрация снапшотов (инициализация base, invalidation грязного cache, lock/marker flow).

Такая структура повышает связанность и снижает безопасность изменений, особенно в
`runJob`, `executeStateTask` и логике snapshot/error handling.

## 2. Цели

- Сохранить внешнее поведение и HTTP/API-контракты без изменений.
- Убрать концентрацию ответственности в `prepare.Manager`.
- Изолировать execution/snapshot-логику от job-координации.
- Выполнить рефакторинг поэтапно и безопасно для тестов.

## 3. Не-цели

- Без изменений endpoint API.
- Без изменений схемы хранения.
- Без изменений queue-модели.
- Без изменения семантики plan/task/event flow.

## 4. Целевая структура компонентов

`prepare.Manager` остаётся внешним фасадом для `httpapi`, но делегирует во внутренние
компоненты:

- `JobCoordinator`
  - Владеет жизненным циклом jobs и переходами queue/event.
  - Владеет планированием/загрузкой задач и оркестрацией их выполнения.
  - Вызывает `TaskExecutor` для `state_execute` и `prepare_instance`.
- `TaskExecutor`
  - Владеет получением/стартом/очисткой runtime и выполнением шагов (`psql`, `lb`).
  - Владеет созданием instance из подготовленного state.
  - Делегирует snapshot-специфичные решения в `SnapshotOrchestrator`.
- `SnapshotOrchestrator`
  - Владеет инициализацией base-state и правилами invalidation грязных state.
  - Владеет проверками до/после snapshot и hygiene логикой state cache.

## 5. Направление зависимостей

- `Manager` -> `JobCoordinator`
- `JobCoordinator` -> `TaskExecutor`
- `TaskExecutor` -> `SnapshotOrchestrator`
- `SnapshotOrchestrator` -> абстракции storage/runtime/statefs/dbms через зависимости `Manager`

Новые package-level зависимости не добавляются. Компоненты остаются в `internal/prepare`.

## 6. Влияние на публичный API

Публичная поверхность `prepare.Manager` не меняется:

- `Submit`, `Recover`, `Get`, `ListJobs`, `ListTasks`, `Delete`
- `EventsSince`, `WaitForEvent`

HTTP handlers и CLI-контракты сохраняются.

## 7. Статус реализации

Split завершён.

- `Manager` остался фасадом для внешних/package-вызовов.
- Тяжёлые orchestration-методы перенесены из `Manager.*Impl` в:
  - `jobCoordinator` (`runJob`, planning, загрузка задач, Liquibase planning flow).
  - `taskExecutor` (runtime/task execution, создание instance, Liquibase execution).
  - `snapshotOrchestrator` (инициализация base + invalidation dirty cache).
- Удалена промежуточная прокладка `Manager -> component -> Manager.*Impl`.
- Переходы queue/event и error mapping сохранены совместимыми по поведению.

## 8. Риски и меры

- Риск: дрейф поведения в последовательности task status/event.
  - Мера: сохранить facade-методы, точки queue/event write и прогонять component + integration тесты.
- Риск: регрессии конкурентности runner/heartbeat.
  - Мера: в первом проходе не менять lock-модель и ownership runner.
- Риск: скрытая связанность через прямой доступ к полям `Manager`.
  - Мера: переносить инкрементально, затем при необходимости ужесточить внутренние контракты.

## 9. Набор верификации

Реализованная верификация ориентирована на поведенческие контракты:

1. `prepare/job_coordinator_test.go`
   - Переходы `runJob` для `plan_only` и execution flow.
2. `prepare/task_executor_test.go`
   - runtime reuse и обновление metadata/status в `state_execute`.
3. `prepare/snapshot_orchestrator_test.go`
   - invalidation грязного cached state и guard-логика base-state init.
4. `prepare/liquibase_consistency_test.go`
   - консистентность подготовки Liquibase request между planning и execution.
5. Существующие `prepare`-тесты
   - проверка порядка queue/event, error mapping и инвариантов lifecycle.
