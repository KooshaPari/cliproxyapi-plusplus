#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

# Compute changed files between base and head
BASE="${AIRLOCK_BASE_SHA:-HEAD~1}"
HEAD="${AIRLOCK_HEAD_SHA:-HEAD}"
CHANGED_FILES=$(git diff --name-only --diff-filter=ACMR "$BASE" "$HEAD" 2>/dev/null || git diff --name-only --cached)

# Filter by language
GO_FILES=$(echo "$CHANGED_FILES" | grep '\.go$' || true)
PY_FILES=$(echo "$CHANGED_FILES" | grep '\.py$' || true)

ERRORS=0

# --- Go ---
if [[ -n "$GO_FILES" ]]; then
  echo "=== Go: gofmt (auto-fix) ==="
  echo "$GO_FILES" | xargs -I{} gofmt -w "{}" 2>/dev/null || true

  echo "=== Go: golangci-lint ==="
  # Get unique directories containing changed Go files
  GO_DIRS=$(echo "$GO_FILES" | xargs -I{} dirname "{}" | sort -u | sed 's|$|/...|')
  # Run golangci-lint but only report issues in changed files
  LINT_OUTPUT=$(golangci-lint run --out-format line-number $GO_DIRS 2>&1 || true)
  if [[ -n "$LINT_OUTPUT" ]]; then
    # Filter to only issues in changed files
    FILTERED=""
    while IFS= read -r file; do
      MATCH=$(echo "$LINT_OUTPUT" | grep "^${file}:" || true)
      if [[ -n "$MATCH" ]]; then
        FILTERED="${FILTERED}${MATCH}"$'\n'
      fi
    done <<< "$GO_FILES"
    if [[ -n "${FILTERED// /}" ]] && [[ "${FILTERED}" != $'\n' ]]; then
      echo "$FILTERED"
      echo "golangci-lint: issues found in changed files"
      ERRORS=1
    else
      echo "golangci-lint: OK (issues only in unchanged files, skipping)"
    fi
  else
    echo "golangci-lint: OK"
  fi
fi

# --- Python ---
if [[ -n "$PY_FILES" ]]; then
  echo "=== Python: ruff format (auto-fix) ==="
  echo "$PY_FILES" | xargs ruff format 2>/dev/null || true

  echo "=== Python: ruff check --fix ==="
  echo "$PY_FILES" | xargs ruff check --fix 2>/dev/null || true

  echo "=== Python: ruff check (verify) ==="
  if echo "$PY_FILES" | xargs ruff check 2>&1; then
    echo "ruff check: OK"
  else
    echo "ruff check: issues found"
    ERRORS=1
  fi
fi

if [[ -z "$GO_FILES" && -z "$PY_FILES" ]]; then
  echo "No Go or Python files changed. Nothing to lint."
fi

exit $ERRORS
