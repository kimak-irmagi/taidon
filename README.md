# Taidon

Taidon is an open-source platform for safe, reproducible SQL experimentation.  
It provides isolated environments, fast snapshot-based databases, and a unified API for executing SQL workloads without side effects.

This repository is a **monorepo** containing all components of the platform:
frontend applications, backend microservices, shared libraries, research materials, and documentation.

---

## Overview

Taidon is designed to support:

- teaching SQL;
- prototyping database-related tools;
- research on query execution and database behavior;
- reproducible environments for demonstrations and workshops.

Core ideas:

- **Reproducibility** — every SQL session starts from a known snapshot.
- **Isolation** — user actions never affect other sessions.
- **Speed** — environments start quickly and scale horizontally.
- **Extensibility** — multiple database backends, pluggable components, internal/external integrations.

The platform will eventually include:

- web UI,
- a set of microservices for SQL execution,
- snapshot and environment orchestration,
- research datasets and performance experiments,
- developer-oriented CLI tools.

---

## Project Structure

```plain
taidon/
  README.md
  CONTRIBUTING.md
  CODE_OF_CONDUCT.md
  LICENSE
  core.ru.md
  vision.ru.md
  package.json
  pnpm-lock.yaml

  docs/                # Architecture, design docs, ADR, user guides
  research/            # LaTeX papers, benchmarks, datasets, experiments

  frontend/
    cli/               # sqlrs CLI
    main/              # Main SPA application (React)
    editor/            # Query editor component
    plan-viewer/       # Query plan visualization

  backend/
    gateway/           # API gateway (BFF) for the frontend
    services/
      audit-log/       # Action/event logging
      autoscaler/      # Autoscaling workers
      env-manager/     # Database container orchestration + snapshots
      snapshot-cache/  # Snapshot lifecycle and warm cache
      sql-runner/      # Fast execution of SQL chains
      telemetry/       # Metrics and usage data
      user-profile/    # Users, orgs, roles, quotas
      vsc-sync/        # Integration with VCS / local FS
    libs/              # Shared backend libraries (types, utils, clients)

  infra/
    docker/            # Dockerfiles for services and DBs
    k8s/               # Kubernetes manifests / Helm charts
    local-dev/         # docker-compose for local development

  scripts/
    dev/               # Development helpers
    external/          # External assets and manifests
    maintenance/       # Database migrations, cleanup tools

  examples/
    chinook/
    flights/
    pgbench/
    sakila/
    README.md

  tools/               # One-off tooling (fetchers, utilities)
  dist/                # Generated CLI bundles
  results/             # Generated experiment artifacts
  sqlrs-work/          # Generated runtime workspace
  node_modules/        # Installed dependencies
```

---

## Subprojects

### **Frontend**

Located under `frontend/`.  
The main application lives in `frontend/main`, with `frontend/editor` and `frontend/plan-viewer` as UI components, and `frontend/cli` hosting the sqlrs CLI.

Each subproject contains its own `README.md` with setup instructions.

---

### **Backend**

Backend services are split into microservices under `backend/services/`, with a shared façade in `backend/gateway/`.

Common libraries, API contracts, and utilities live in `backend/libs/`.

Each service includes its own documentation and tooling.

---

### **Documentation**

Architecture, specifications, ADRs, and design notes live in `docs/`.
Start from:

- [`docs/README.md`](docs/README.md) — doc index
- [`docs/architecture/README.md`](docs/architecture/README.md) — architecture overview and per-service docs
- [`docs/requirements-architecture.md`](docs/requirements-architecture.md) - core requirements
- [`docs/roadmap.md`](docs/roadmap.md) — roadmap and milestones

---

### **Research**

Experimental data, benchmarks, LaTeX papers, and notebooks live in `research/`.

This section supports the development of snapshot strategies, SQL execution models, and performance analysis.

---

## Contributing

We welcome contributions from students, volunteers, and professionals.

Please read the contribution guidelines:

- **[CONTRIBUTING.md](./CONTRIBUTING.md)**

---

## Code of Conduct

We are committed to providing a welcoming and inclusive environment.

Please review our Code of Conduct:

- **[CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)**

---

## License

This project is distributed under the **Apache License 2.0**.

Full text:

- **[LICENSE](./LICENSE)**

---
