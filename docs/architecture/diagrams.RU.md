# Диаграммы архитектуры (MVP)

> Формат: mermaid. Открывать в Markdown с поддержкой mermaid или через онлайн-viewer.

## 1. Топология сервисов и коммуникаций (k8s)

```mermaid
flowchart LR
  subgraph Client
    UI["Frontend (SPA)"]
  end

  subgraph Edge
    GW["gateway (BFF/API)"]
  end

  subgraph ControlPlane
    IDP[idp]
    UP[user-profile]
    VCS[vcs-sync]
    AUD[audit-log]
    TELE[telemetry/exporter]
    META[(Control DB)]
  end

  subgraph DataPlane
    SR[sql-runner]
    SC[snapshot-cache]
    EM[env-manager]
    PG[(PostgreSQL sandboxes)]
    SNAP["Snapshot store (CoW layers)"]
  end

  UI -->|HTTPS| GW
  GW --> IDP
  GW --> UP
  GW --> SR
  GW --> VCS

  IDP --> META
  UP --> META
  VCS --> META
  AUD --> META

  SR --> SC
  SR --> EM
  SC <-->|metadata| META
  SC --> SNAP
  EM --> SNAP
  EM --> PG

  SR -->|logs| AUD
  GW -->|logs| AUD

  VCS -->|Git pull/push| GitRemote[(Remote Git)]

  TELE -->|metrics| Prometheus
  Prometheus --> Grafana
```

## 2. Внутренняя архитектура ключевых сервисов

### 2.1 sql-runner

```mermaid
flowchart TD
  API[API handler]
  Plan[Planner: парсит проект, делит tail/head, считает хэш tail]
  CacheClient[Snapshot-cache client]
  EnvBind[Env binding: запрос/поднятие песочницы через env-manager]
  Exec[Executor: подключение к Postgres, выполнение head]
  Tele[Telemetry/log hooks]

  API --> Plan --> CacheClient
  CacheClient -->|hit| EnvBind
  CacheClient -->|miss| EnvBind
  EnvBind --> Exec --> Tele
  Exec --> CacheClient
```

### 2.2 snapshot-cache

```mermaid
flowchart TD
  API[API/GRPC]
  Index["Metadata index (control DB)"]
  Storage["Storage driver (CoW layers)"]
  Policy["Eviction policy: cost/freq/size"]
  GC[GC/compaction]

  API --> Index
  API --> Storage
  Storage --> Index
  Index --> Policy --> Storage
  Storage --> GC --> Index
```

### 2.3 env-manager

```mermaid
flowchart TD
  API[API/queue]
  Policy[Policy engine: quotas/TTL/limits]
  Template[Spec builder: StatefulSet/Job/PVC]
  Kube[Kubernetes API]
  Watch[Watcher: Pod/PVC status, readiness]
  Metrics[Telemetry]

  API --> Policy --> Template --> Kube
  Kube --> Watch --> Metrics
  Template --> Metrics
```

### 2.4 gateway (BFF/API)

```mermaid
flowchart TD
  Edge[Ingress/TLS termination]
  Auth[Auth middleware: tokens/anon issuance]
  RL[Rate limiter + quotas]
  Router[API router/BFF composition]
  Clients[Service clients: idp/user-profile/sql-runner/vcs-sync]
  Obs[Telemetry/log hooks]

  Edge --> Auth --> RL --> Router --> Clients
  Router --> Obs
```
