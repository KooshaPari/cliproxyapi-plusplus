#!/usr/bin/env bash

set -euo pipefail

ALLOWED_OWNERS=(KooshaPari atoms-tech)
USAGE_OWNERS=$(printf '%s, ' "${ALLOWED_OWNERS[@]}")
USAGE_OWNERS="${USAGE_OWNERS%, }"

usage() {
  cat <<USAGE
Usage:
  ./scripts/github-owned-guard.sh <github_target>

  github_target can be:
    - owner/repo
    - https://github.com/owner/repo/...
    - git@github.com:owner/repo

  Exit codes:
    0  target owner is allowed
    2  target owner is not allowed or target could not be parsed

  Allowed owners: ${USAGE_OWNERS}
USAGE
}

if [[ $# -lt 1 ]]; then
  usage
  exit 2
fi

TARGET="${1}"

parse_owner() {
  local input="$1"
  local owner repo

  input="${input%.git}"

  if [[ "$input" == http*://github.com/* ]]; then
    input="${input#https://github.com/}"
    input="${input#http://github.com/}"
    IFS='/' read -r owner repo _ <<< "$input"
  elif [[ "$input" == git@github.com:* ]]; then
    input="${input#git@github.com:}"
    IFS='/' read -r owner repo _ <<< "$input"
  else
    IFS='/' read -r owner repo _ <<< "$input"
  fi

  if [[ -z "${owner:-}" || -z "${repo:-}" ]]; then
    echo "Failed to parse owner/repo from: $1" >&2
    return 2
  fi

  printf '%s' "$owner"
}

OWNER="$(parse_owner "$TARGET")"

for allowed in "${ALLOWED_OWNERS[@]}"; do
  if [[ "$OWNER" == "$allowed" ]]; then
    exit 0
  fi
done

echo "Blocked: github owner '$OWNER' is not allowed for issue/PR/comment actions." >&2
echo "Allowed owners: ${USAGE_OWNERS}" >&2
echo "Target: $TARGET" >&2
exit 2
