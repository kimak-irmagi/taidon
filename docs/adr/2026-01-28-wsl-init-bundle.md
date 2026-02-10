# ADR: WSL2 init via bundled engine and btrfs bootstrap

Status: Accepted (CLI naming superseded by [ADR 2026-02-10: sqlrs init redesign](2026-02-10-sqlrs-init-redesign.md))
Date: 2026-01-28

## Decision Record 1: WSL2 init automation and bundled engine

- Timestamp: 2026-01-28T12:20:00.000+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should Windows local setup provision the Linux engine binary and btrfs storage for WSL2?
- Alternatives:
  - Bundle both Windows and Linux engine binaries in the Windows distribution.
  - Download Linux engine on-demand during `sqlrs init`.
  - Require manual installation of the Linux engine in WSL.
- Decision: Bundle Windows CLI + Windows engine + Linux engine (same CPU arch). `sqlrs init` copies the Linux engine into the WSL distro and manages btrfs volume setup (validate/init/re-init).
- Rationale: Keeps setup offline-friendly, minimizes user steps, and enables repeatable automation for WSL2 + btrfs.

## Decision Record 2: WSL auto-start behavior

- Timestamp: 2026-01-28T12:20:00.000+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: Should CLI auto-start the WSL distro by default?
- Alternatives:
  - Auto-start WSL by default; allow `--no-start` to disable.
  - Never auto-start WSL; require manual start.
- Decision: Auto-start WSL by default; `--no-start` disables auto-start.
- Rationale: Reduces friction for the common case while preserving explicit control.
