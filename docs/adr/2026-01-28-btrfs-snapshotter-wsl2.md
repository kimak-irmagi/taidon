# ADR: btrfs snapshotter via WSL2 on Windows

Status: Accepted
Date: 2026-01-28

## Decision Record 1: btrfs snapshotter + WSL2 for Windows CoW

- Timestamp: 2026-01-28T09:45:28.9276486+07:00
- User: @evilguest
- Agent: Codex (GPT-5)
- Question: How should we provide block-level CoW snapshots to Windows users while keeping Docker Desktop?
- Alternatives:
  - Implement a Windows-native snapshotter (ReFS/Block Cloning or VHDX-based).
  - Use ZFS inside WSL2 for snapshots.
  - Use a single Linux btrfs snapshotter and run the engine inside WSL2 on Windows.
- Decision: Implement a Linux-only btrfs snapshotter and use it on Windows by running the engine inside WSL2. No separate Windows snapshotter is planned for this stage.
- Rationale: Docker Desktop with Linux containers already runs inside WSL2; btrfs provides block-level CoW with mature tooling there, while Windows-native solutions are less compatible with Linux container data paths and add complexity.
