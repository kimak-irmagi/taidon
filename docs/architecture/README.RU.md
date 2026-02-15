# Документы по архитектуре

Точки входа по архитектуре и дизайну сервисов Taidon.

- [`mvp-architecture.RU.md`](mvp-architecture.RU.md) - состав MVP-сервисов и цели.
- [`diagrams.RU.md`](diagrams.RU.md) - высокоуровневые диаграммы компонентов и k8s топологии.
- [`local-deployment-architecture.RU.md`](local-deployment-architecture.RU.md) - локальный профиль (тонкий CLI, эфемерный engine).
- [`shared-deployment-architecture.RU.md`](shared-deployment-architecture.RU.md) - team/cloud профиль (gateway/orchestrator, multi-tenant).
- [`engine-internals.RU.md`](engine-internals.RU.md) - внутренняя структура sqlrs engine.
- [`runtime-snapshotting.RU.md`](runtime-snapshotting.RU.md) - модель хранения, snapshot/backends (OverlayFS/копирование/etc).
- [`state-cache-design.RU.md`](state-cache-design.RU.md) - cache keys, триггеры, retention, локальный layout.
- [`sql-runner-api.RU.md`](sql-runner-api.RU.md) - поверхность API runner, таймауты, отмена, стриминг.
- [`liquibase-integration.RU.md`](liquibase-integration.RU.md) - стратегия Liquibase провайдера и структурированные логи.
- [`k8s-architecture.RU.md`](k8s-architecture.RU.md) - Kubernetes архитектура с единой точкой входа через gateway.
- [`cli-contract.RU.md`](cli-contract.RU.md) - CLI контракт и команды.
- [`cli-architecture.RU.md`](cli-architecture.RU.md) - CLI флоу для local vs remote и загрузки исходников.
- [`cli-component-structure.RU.md`](cli-component-structure.RU.md) - внутренняя структура CLI.
- [`local-engine-component-structure.RU.md`](local-engine-component-structure.RU.md) - внутренняя структура локального engine.
- [`prepare-manager-refactor.RU.md`](prepare-manager-refactor.RU.md) - разбиение prepare manager на coordinator/executor/snapshot роли.
- [`statefs-component-structure.RU.md`](statefs-component-structure.RU.md) - контракт StateFS и изоляция ФС-логики в engine.
- [`local-engine-storage-schema.RU.md`](local-engine-storage-schema.RU.md) - схема SQLite для локального engine.
- [`../api-guides/sqlrs-engine.openapi.yaml`](../api-guides/sqlrs-engine.openapi.yaml) - спецификация OpenAPI 3.1 для локального engine (MVP).
- [`query-analysis-workflow-review.RU.md`](query-analysis-workflow-review.RU.md) - заметки по workflow анализа запросов.
- [`git-aware-passive.RU.md`](git-aware-passive.RU.md) - сценарии работы с git, инициируемые локальным CLI.
- [`git-aware-active.RU.md`](git-aware-active.RU.md) - сценарии github-интеграции, требующие сервисного API.
