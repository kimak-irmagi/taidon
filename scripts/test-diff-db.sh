#!/usr/bin/env bash
# Test sqlrs diff on two local DB script trees (testdata/diff-db/left and right).
# Usage:
#   ./scripts/test-diff-db.sh           — only diff (engine not required)
#   ./scripts/test-diff-db.sh --with-engine — build engine, start it, status, then diff

set -e
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

LEFT="${REPO_ROOT}/testdata/diff-db/left"
RIGHT="${REPO_ROOT}/testdata/diff-db/right"
WORK="${REPO_ROOT}/sqlrs-work"
RUN_DIR="${WORK}/state/sqlrs/run"
ENGINE_JSON="${WORK}/state/sqlrs/engine.json"

if [[ ! -d "$LEFT" || ! -d "$RIGHT" ]]; then
  echo "Missing testdata: $LEFT or $RIGHT"
  exit 1
fi

# Build CLI
SQLRS="${REPO_ROOT}/dist/bin/sqlrs"
mkdir -p "$(dirname "$SQLRS")"
echo "Building sqlrs..."
go build -o "$SQLRS" ./frontend/cli-go/cmd/sqlrs

WITH_ENGINE=
for a in "$@"; do [[ "$a" == "--with-engine" ]] && WITH_ENGINE=1; done

if [[ -n "$WITH_ENGINE" ]]; then
  ENGINE="${REPO_ROOT}/dist/bin/sqlrs-engine"
  if [[ ! -x "$ENGINE" ]]; then
    echo "Building sqlrs-engine..."
    go build -o "$ENGINE" ./backend/local-engine-go/cmd/sqlrs-engine
  fi
  mkdir -p "$RUN_DIR"
  export XDG_CONFIG_HOME="${WORK}/config"
  export XDG_STATE_HOME="${WORK}/state"
  export XDG_CACHE_HOME="${WORK}/cache"
  export SQLRS_DAEMON_PATH="$ENGINE"
  echo "Starting engine (background)..."
  "$ENGINE" --listen 127.0.0.1:0 --run-dir "$RUN_DIR" --write-engine-json "$ENGINE_JSON" --idle-timeout 30s &
  ENG_PID=$!
  trap "kill $ENG_PID 2>/dev/null || true" EXIT
  sleep 1
  echo "=== sqlrs status ==="
  "$SQLRS" status || true
  echo ""
fi

echo "=== sqlrs diff (human) ==="
"$SQLRS" diff --from-path "$LEFT" --to-path "$RIGHT" plan:psql -- -f main.sql

echo ""
echo "=== sqlrs diff (JSON) ==="
"$SQLRS" --output json diff --from-path "$LEFT" --to-path "$RIGHT" plan:psql -- -f main.sql

echo ""
echo "Done."
