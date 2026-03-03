#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

if command -v ggshield >/dev/null 2>&1; then
  GGSHIELD=(ggshield)
elif command -v uvx >/dev/null 2>&1; then
  GGSHIELD=(uvx ggshield)
elif command -v uv >/dev/null 2>&1; then
  GGSHIELD=(uv tool run ggshield)
else
  echo "ERROR: ggshield not installed. Install with: pipx install ggshield or uv tool install ggshield" >&2
  exit 1
fi

echo "[security-guard] Running ggshield secret scan"
"${GGSHIELD[@]}" secret scan pre-commit

if command -v codespell >/dev/null 2>&1; then
  echo "[security-guard] Running optional codespell fast pass"
  file_count=0
  while IFS= read -r -d '' path; do
    case "$path" in
      *.md|*.txt|*.py|*.ts|*.tsx|*.js|*.go|*.rs|*.kt|*.java|*.yaml|*.yml)
        codespell -q 2 -L "hte,teh" "$path" || true
        file_count=$((file_count + 1))
        ;;
    esac
  done < <(
    if git rev-parse --verify HEAD >/dev/null 2>&1; then
      git diff --cached --name-only --diff-filter=ACM -z
      git diff --name-only --diff-filter=ACM HEAD~1..HEAD -z 2>/dev/null || true
    else
      git ls-files -z
    fi
  )

  if [ "$file_count" -eq 0 ]; then
    echo "[security-guard] No matching files for codespell"
  fi
fi
