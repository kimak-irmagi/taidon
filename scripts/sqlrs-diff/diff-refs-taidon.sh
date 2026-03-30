#!/usr/bin/env bash
# Compare psql file closures between two Git refs inside the taidon checkout.
#
# Why ./your/script.sql fails from frontend/cli-go:
#   In --from-ref/--to-ref mode, your process cwd is mapped under each temporary
#   worktree. Running from frontend/cli-go makes BaseDir
#   <worktree>/frontend/cli-go, so -f ./your/script.sql looks for a file that is
#   not there. Run from repo root (this script does `cd` there) and pass -f
#   relative to that cwd (e.g. examples/pgbench/query.sql).
#
# Psql note: plain \i resolves from the command base dir (here: repo root in the
# worktree), not the including file's directory. Scripts that only \ir other
# files work relative to the entry file; many examples/chinook / sakila / flights
# files \ir large assets that are gitignored — those paths are missing in a clean
# worktree unless you vendor them.
#
# Usage (from anywhere):
#   ./scripts/sqlrs-diff/diff-refs-taidon.sh
#   SQLRS=/tmp/sqlrs ./scripts/sqlrs-diff/diff-refs-taidon.sh
#   FROM_REF=main TO_REF=HEAD ./scripts/sqlrs-diff/diff-refs-taidon.sh
#   PSQL_FILE=examples/flights-multi-step/schema.sql ./scripts/sqlrs-diff/diff-refs-taidon.sh
#   OUTPUT=json ./scripts/sqlrs-diff/diff-refs-taidon.sh
#
# Liquibase (same cwd rules; changelog must exist in both refs in Git):
#   sqlrs diff --from-ref HEAD~1 --to-ref HEAD plan:lb -- update \
#     --changelog-file path/from/repo/root/to/master.xml
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel)"
cd "$REPO_ROOT"

FROM_REF="${FROM_REF:-HEAD~1}"
TO_REF="${TO_REF:-HEAD}"
PSQL_FILE="${PSQL_FILE:-examples/pgbench/query.sql}"
OUTPUT="${OUTPUT:-human}"

SQLRS="${SQLRS:-}"
if [[ -z "$SQLRS" ]]; then
  SQLRS="$REPO_ROOT/dist/bin/sqlrs"
  if [[ ! -x "$SQLRS" ]]; then
    mkdir -p "$(dirname "$SQLRS")"
    echo "Building sqlrs -> $SQLRS"
    go build -o "$SQLRS" ./frontend/cli-go/cmd/sqlrs
  fi
fi

if [[ ! -f "$PSQL_FILE" ]]; then
  echo "Missing file (from repo root): $REPO_ROOT/$PSQL_FILE"
  exit 1
fi

echo "repo:    $REPO_ROOT"
echo "refs:    $FROM_REF .. $TO_REF"
echo "entry:   $PSQL_FILE"
echo "sqlrs:   $SQLRS"
echo ""

DIFF_ARGS=(diff --from-ref "$FROM_REF" --to-ref "$TO_REF" plan:psql -- -f "$PSQL_FILE")
if [[ "$OUTPUT" == "json" ]]; then
  "$SQLRS" --output json "${DIFF_ARGS[@]}"
else
  "$SQLRS" "${DIFF_ARGS[@]}"
fi
