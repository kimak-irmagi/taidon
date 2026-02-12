# Рефакторинг Prepare Manager (Предложение)

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

## 7. План миграции

### Phase 1: Введение компонентов и delegating wrappers

- Добавить внутренние структуры:
  - `jobCoordinator`
  - `taskExecutor`
  - `snapshotOrchestrator`
- Инициализировать их в `NewManager`.
- Оставить текущие методы `Manager`, но тяжёлые методы превратить в тонкие делегаты.

### Phase 2: Перенос тел методов по зонам ответственности

- Перенести тела оркестрации jobs в `jobCoordinator`.
- Перенести runtime/task execution в `taskExecutor`.
- Перенести snapshot cache/base-init в `snapshotOrchestrator`.

### Phase 3: Очистка и уменьшение дублирования

- Без изменения поведения убрать очевидное дублирование подготовки Liquibase запуска.
- Проверить сохранение существующих queue/event writes и error mapping.

## 8. Риски и меры

- Риск: дрейф поведения в последовательности task status/event.
  - Мера: сначала сохранить wrapper-делегацию и прогнать существующие тесты.
- Риск: регрессии конкурентности runner/heartbeat.
  - Мера: в первом проходе не менять lock-модель и ownership runner.
- Риск: скрытая связанность через прямой доступ к полям `Manager`.
  - Мера: переносить инкрементально, затем при необходимости ужесточить внутренние контракты.

## 9. Дизайн тестов (на утверждение до реализации)

Новые тесты:

1. `prepare/manager_delegation_test.go`
   - Проверка, что `Manager` делегирует тяжёлые flow в coordinator/executor/orchestrator.
2. `prepare/job_coordinator_test.go`
   - Покрытие переходов `runJob` для `plan_only` и execution-сценариев (queued -> running -> succeeded/failed).
3. `prepare/task_executor_test.go`
   - Покрытие acquire/reuse runtime и обновления статусов в `state_execute`.
4. `prepare/snapshot_orchestrator_test.go`
   - Покрытие invalidation грязного cached state и guard-логики base-state init.
5. `prepare/liquibase_consistency_test.go`
   - Гарантия паритета между planning-time и execution-time подготовкой Liquibase args/env.

Существующие тесты сохраняются и продолжают проверять поведенческие инварианты.
