# Taidon Roadmap

This roadmap prioritises scenarios, use cases, and components to maximise early product value while keeping a clear path to team (on-prem) and cloud/education offerings.

---

## Goals and Non-Goals

### Goals

- Deliver a fast, reproducible database sandbox for developers (local-first)
- Provide a single invariant core (Engine + API) across all deployment profiles
- Enable CI/CD integration for team adoption
- Preserve a clean upgrade path to public cloud sharing and education workflows

### Non-Goals (initially)

- Full multi-tenant billing and payments
- Support for many database engines at once
- Full browser-based IDE hosting (VS Code-in-browser)

---

## Roadmap Overview

```mermaid
gantt
    title Taidon Roadmap (High Level)
    dateFormat  YYYY-MM-DD
    axisFormat  %b %Y

    section Foundation
    Core API + Engine skeleton                 :a1, 2026-01-01, 30d
    Local runtime + sandbox lifecycle          :a2, after a1, 45d
    Liquibase adapter (apply migrations)       :a3, after a2, 30d

    section Product MVP (Local)
    Drop-in connection (proxy/adapter)         :b1, after a2, 45d
    CLI UX + deterministic runs                :b2, after a2, 45d
    State cache v1 (reuse states)              :b3, after a3, 45d
    Git-aware CLI (ref/diff/provenance)        :b4, after b3, 30d

    section Team (On-Prem)
    Orchestrator + quotas + TTL                :c1, after b2, 45d
    K8s gateway + controller topology          :c6, after c1, 30d
    Artifact store (S3/fs) + audit log         :c2, after c1, 45d
    CI/CD integration templates                :c3, after c1, 30d
    Auth (OIDC) + RBAC (basic)                 :c4, after c1, 45d
    Autoscaling (sandboxes + cache workers)    :c5, after c2, 30d

    section Cloud (Sharing)
    Sharing artefacts (immutable runs)         :d1, after c2, 45d
    Public read-only pages                     :d2, after d1, 30d
    Anti-abuse limits (rate/quota)             :d3, after d1, 30d

    section Research / Optional
    Cloud Git repo integration (VCS sync)      :o1, after d2, 45d
    GitHub PR automation (warmup/diff checks)  :o2, after o1, 45d

    section Education
    Course/Assignment/Submission model         :e1, after d2, 45d
    Autograding runner                         :e2, after e1, 45d
```

> Dates are placeholders to visualise ordering. The roadmap is milestone-driven.

---

## Milestones

### M0. Architecture Baseline

**Outcome**: stable concepts and contracts before heavy implementation.

- Freeze canonical entities: project, sandbox, run, artefact, share
- Freeze core API surface (create/apply/run/destroy + status/events)
- Decide runtime isolation approach for MVP (local containers vs other)

**Key documents to produce next**:

- [`api-contract.md`](api-contract.md)
- [`sandbox-lifecycle.md`](sandbox-lifecycle.md)
- [`state-cache-design.md`](architecture/state-cache-design.md)

---

### M1. Local MVP (Scenario A1)

**Primary scenario**: A1 local development with Liquibase.

**Target use cases**:

- UC-1 Provision isolated database sandbox
- UC-2 Apply migrations (Liquibase / SQL)
- UC-3 Run tests / queries / scripts
- UC-4 Cache and reuse database states

**Deliverables**:

- Taidon Engine + API (local mode)
- Local runtime (containers) with sandbox lifecycle
- Liquibase adapter (apply changelog)
- CLI:
  - `sqlrs apply`, `sqlrs run`, `sqlrs destroy`
  - `sqlrs status`, `sqlrs logs`
- Cache v1:
  - cache key: `db_engine + base_image + changelog_hash + seed_hash`
  - reuse by snapshot/clone strategy (implementation-dependent)

**Optional (nice-to-have)**:

- VS Code extension v0:
  - list sandboxes
  - apply migrations
  - show logs and run results

**Exit criteria**:

- A cold start produces a working sandbox
- A warm start reuses cached state and is significantly faster
- Migrations are deterministic and reproducible

---

### M2. Drop-in Replacement (Developer Experience)

**Purpose**: make adoption nearly effortless.

**Target use cases**:

- UC-1, UC-2, UC-3 (with minimal config)

**Deliverables**:

- Connection drop-in strategy (choose one for MVP):
  - Local proxy that exposes `localhost:<port>` as Postgres/MySQL, or
  - Driver/URL adapter (for selected stacks)
- Config conventions:
  - discover migrations from repo layout
  - profiles (dev/test) and secrets handling
- Git-aware CLI (passive):
  - `--ref` (blob/worktree), `diff`, provenance, cache explain
- VS Code extension v1 (optional):
  - one-click copy DSN
  - open SQL editor (via existing VS Code DB tooling)

**Exit criteria**:

- A developer can run tests with a single env var change (or one config line)

---

### M3. Team On-Prem (Scenario A2)

**Primary scenario**: shared Taidon for a team/department.

**Target use cases**:

- UC-5 Integrate with CI/CD
- UC-4 Cache and reuse database states (shared)
- UC-1..UC-3 at scale

**Deliverables**:

- Orchestrator service:
  - job queue
  - quotas
  - TTL policies
- K8s shared deployment baseline:
  - single entrypoint Gateway (TCP)
  - Controller-managed DB runner pods
- Artifact store:
  - logs, reports, exports
  - retention policies
- Auth and RBAC (basic):
  - OIDC login
  - organisation/team scopes
- CI templates:
  - GitHub Actions / GitLab CI examples
- Autoscaling controller (sandboxes + cache workers):
  - HPA/VPA profiles using backlog/cache metrics
  - Warm pool for fast start; graceful drain on scale-in

**Exit criteria**:

- Multiple developers can run isolated sandboxes concurrently
- Quotas prevent resource exhaustion
- CI pipelines can provision and teardown sandboxes reliably

---

### M4. Public Cloud Sharing (Scenario B3)

**Primary scenario**: quick experiments and public sharing.

**Target use cases**:

- UC-6 Share experiment results

**Deliverables**:

- Immutable run snapshots:
  - shareable artefact bundles
  - redaction of secrets
- Public read-only pages:
  - view results
  - reproduce button (clone into user workspace)
- Anti-abuse controls:
  - rate limiting
  - sandbox limits
  - TTL enforcement

**Exit criteria**:

- A user can share a run via link
- Another user can reproduce it in a controlled environment

---

### R1. Cloud Git Integration (Optional / Research)

**Purpose**: connect the cloud sandbox with Git repositories.

**Deliverables**:

- VCS/Git connector (API) with private-repo support
- Bind project to branch/commit; start sandbox from the selected revision
- One-time tokens/SSO for Git (secrets managed in cloud)
- Optional auto-sync/pull to refresh sandbox state

**Exit criteria**:

- User can attach a Git repo and start a sandbox from a chosen branch/commit
- Repo updates are available in the sandbox without manual re-import

---

### R2. GitHub PR Automation (Optional / Research)

**Purpose**: automate warmup and diffs around PRs without leaking secrets.

**Deliverables**:

- GitHub App / webhook receiver
- PR slash commands:
  - `/taidon warmup --prepare <path>`
  - `/taidon diff --from-ref base --to-ref head --prepare <path>`
  - `/taidon compare --from-ref base --from-prepare <path> --to-ref head --to-prepare <path> --run "..."`
- Check Runs:
  - warmup status
  - Taidon-aware diff summary
- Warmup via `sqlrs run` in prepare-only mode (no DSN in PR)
- Eviction hints from Git events (merge/tag/PR closed)

**Exit criteria**:

- PR label or slash command triggers warmup in a controlled runner
- Check Run shows diff/warmup summary without exposing secrets

---

### M5. Education (Scenarios C4a/C4b)

**Primary scenario**: assignments, submissions, and grading.

**Target use cases**:

- UC-7 Prepare assignments
- UC-8 Submit and review results

**Deliverables**:

- Course/assignment/submission model
- Autograding runner:
  - instructor-defined checks (SQL/tests)
  - structured grading report
- Instructor dashboard:
  - list submissions
  - compare results

**Exit criteria**:

- Instructor can publish assignment template
- Students submit runs
- Instructor can evaluate consistently

---

## Prioritisation Rationale

1. Local MVP (A1) yields immediate product value and validates core performance
2. Drop-in adoption reduces friction and improves feedback velocity
3. Team on-prem (A2) unlocks the enterprise path (quotas, auth, CI)
4. Sharing (B3) becomes safe once artefacts, auth, and quotas exist
5. Education (C4) builds naturally on sharing plus role-based access

---

## Scope Controls and Decisions to Lock Early

- Database engines for MVP (start with one, expand later)
- Runtime isolation mechanism (container/k8s) and snapshot approach
- Cache semantics and invalidation rules
- Security boundaries (network egress, time limits, resource limits)
- API stability policy (versioning strategy)

---

## Risks and Mitigations

- Performance regressions due to heavy isolation
  - Mitigation: cache-first architecture and benchmark gates
- Non-deterministic migrations and flaky states
  - Mitigation: strict hashing, pinned images, reproducible seed strategy
- Cloud abuse vectors (untrusted SQL)
  - Mitigation: strong sandboxing, quotas, network policies, TTL

---

## Next Documents to Detail

- [`api-contract.md`](api-contract.md) (REST/gRPC + events)
- [`sql-runner-api.md`](architecture/sql-runner-api.md) (timeouts, cancel, streaming, cache-aware planning)
- [`runtime-and-isolation.md`](runtime-and-isolation.md) (local + k8s)
- [`liquibase-integration.md`](architecture/liquibase-integration.md) (modes, config discovery)
- [`state-cache-design.md`](architecture/state-cache-design.md) (snapshotting, hashing, retention)
- [`cli-spec.md`](cli-spec.md) (commands and exit codes)
- [`security-model.md`](security-model.md) (cloud-hardening, redaction, audit)
- [`runtime-snapshotting.md`](architecture/runtime-snapshotting.md) (details of the snapshot mechanics)
- [`git-aware-passive.md`](architecture/git-aware-passive.md) (CLI by ref, zero-copy, provenance)
- [`git-aware-active.md`](architecture/git-aware-active.md) (PR automation, warmup/diff checks)
- [`k8s-architecture.md`](architecture/k8s-architecture.md) (single entry gateway in k8s)
