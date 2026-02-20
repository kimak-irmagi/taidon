# ADR: windows-latest WSL happy-path probe workflow

Status: Accepted
Date: 2026-02-20

## Decision Record 1: validate Windows hosted-runner happy-path via standalone workflow

- Timestamp: 2026-02-20T22:53:14.6043817+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should we validate `hp-psql-chinook` happy-path on standard
  `windows-latest` runners before integrating it into release-candidate gating?
- Alternatives:
  - Keep Windows smoke-only and postpone happy-path validation.
  - Add Windows happy-path directly into `release-local.yml` immediately.
  - Create a separate probe workflow on `windows-latest` that sets up WSL and
    runs the chinook happy-path end to end.
- Decision: Implement a standalone probe workflow
  (`e2e-windows-wsl-probe.yml`) that:
  - provisions WSL (`Vampire/setup-wsl`);
  - runs `hp-psql-chinook` inside WSL with `snapshot=copy`;
  - validates output against existing golden file;
  - uploads diagnostics for iteration.
- Rationale: This de-risks hosted Windows environment/tooling assumptions
  (WSL+Docker path) without breaking current release gating, and provides a
  direct path to later merge into `release-local.yml` once stable.

## Decision Record 2: include host docker setup action as additional bootstrap

- Timestamp: 2026-02-20T22:57:34.3032848+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should the Windows WSL probe include `docker/setup-docker-action`
  bootstrap?
- Alternatives:
  - Keep only in-WSL Docker bootstrap (`docker.io` + `dockerd`).
  - Add host Docker bootstrap action and keep in-WSL bootstrap fallback.
- Decision: Add `docker/setup-docker-action@v4` in the probe workflow before WSL
  execution, while retaining in-WSL daemon bootstrap logic.
- Rationale: This improves startup robustness on hosted runners and preserves the
  existing WSL-local fallback path if host-level setup is insufficient.

## Decision Record 3: add temporary push trigger for branch iteration

- Timestamp: 2026-02-20T23:05:03.4995942+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should the probe workflow be triggered before merge so it is
  visible and runnable during branch work?
- Alternatives:
  - Keep `workflow_dispatch` only.
  - Add temporary `push` trigger for non-main branches plus keep
    `workflow_dispatch`.
- Decision: Add temporary `push` trigger with `branches-ignore: [main]` while
  keeping `workflow_dispatch`.
- Rationale: This allows immediate branch-level validation before merge and
  avoids extra noise on `main`.
