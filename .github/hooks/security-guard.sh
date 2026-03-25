#!/usr/bin/env bash
set -euo pipefail

HOOK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$HOOK_DIR/../.." && pwd)"

if [ -x "$PROJECT_ROOT/.venv/bin/pre-commit" ]; then
  PRE_COMMIT="$PROJECT_ROOT/.venv/bin/pre-commit"
elif command -v pre-commit >/dev/null 2>&1; then
  PRE_COMMIT="pre-commit"
else
  echo "pre-commit executable not found; trying to install via pip"
  python -m pip install --quiet pre-commit
  PRE_COMMIT="pre-commit"
fi

"$PRE_COMMIT" run --hook-stage pre-commit --config "$PROJECT_ROOT/.pre-commit-config.yaml" --show-diff-on-failure
