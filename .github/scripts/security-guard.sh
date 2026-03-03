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
  determine_base_ref() {
    local base_ref="HEAD~1"
    if git rev-parse --verify HEAD >/dev/null 2>&1 && [ -n "${GITHUB_BASE_REF:-}" ]; then
      base_ref="$(git merge-base HEAD "origin/${GITHUB_BASE_REF}")" || base_ref="HEAD~1"
      if [ "$base_ref" = "" ] || [ "$base_ref" = " " ]; then
        base_ref="HEAD~1"
      fi
    fi
    printf "%s\n" "$base_ref"
  }

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
      base_ref="$(determine_base_ref)"
      git diff --name-only --diff-filter=ACM "${base_ref}..HEAD" -z 2>/dev/null || true
    else
      git ls-files -z
    fi
  )
  if [ "$file_count" -eq 0 ]; then
    echo "[security-guard] No matching files for codespell"
  fi
fi
