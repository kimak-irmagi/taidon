#!/usr/bin/env bash
# Demo: sqlrs diff --from-ref / --to-ref on two commits that change the same -f file.
#
# Why: ref mode checks out each ref into a temp worktree; the wrapped path (-f …)
# must exist in BOTH trees. This script builds a tiny repo where that holds.
#
# Usage (from anywhere; script must live inside the taidon repo):
#   ./scripts/sqlrs-diff/demo-diff-refs.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR" && git rev-parse --show-toplevel 2>/dev/null)" || true
if [[ -z "${REPO_ROOT:-}" ]]; then
  REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
fi
cd "$REPO_ROOT"

SQLRS="${REPO_ROOT}/dist/bin/sqlrs"
if [[ ! -x "$SQLRS" ]]; then
  echo "Building sqlrs -> $SQLRS"
  mkdir -p "$(dirname "$SQLRS")"
  go build -o "$SQLRS" ./frontend/cli-go/cmd/sqlrs
fi

DEMO_DIR="$(mktemp -d "${TMPDIR:-/tmp}/sqlrs-demo-diff-refs.XXXXXX")"
cleanup() { rm -rf "$DEMO_DIR"; }
trap cleanup EXIT

cd "$DEMO_DIR"
git init -q
git config user.email "demo@example.com"
git config user.name "sqlrs diff demo"

mkdir -p schema
cat > schema/a.sql <<'SQL'
-- revision A
SELECT 1 AS side;
SQL
git add schema/a.sql
git commit -q -m "rev A: SELECT 1"

cat > schema/a.sql <<'SQL'
-- revision B
SELECT 2 AS side;
SQL
git add schema/a.sql
git commit -q -m "rev B: SELECT 2"

echo ">>> Demo repo: $DEMO_DIR"
echo ">>> Commits: $(git rev-parse --short HEAD~1) (from) vs $(git rev-parse --short HEAD) (to)"
echo ""

echo "=== Human output ==="
"$SQLRS" diff --from-ref HEAD~1 --to-ref HEAD plan:psql -- -f schema/a.sql

echo ""
echo "=== JSON (excerpt: look for modified / hashes) ==="
"$SQLRS" --output json diff --from-ref HEAD~1 --to-ref HEAD plan:psql -- -f schema/a.sql \
  | head -c 2000
echo ""
echo "..."
echo ""
echo "Done. Tip: use HEAD~1 and HEAD (or two branch names) when both revisions contain the same -f path."
