#!/usr/bin/env bash
set -euo pipefail

required_paths=(
  "docs/changes/shared-modules/proxy-auth-access-sdk-v1/proposal.md"
  "docs/changes/shared-modules/proxy-auth-access-sdk-v1/tasks.md"
  "docs/contracts/proxy-auth-access-sdk.contract.json"
  "sdk/access_module_v1/README.md"
)

fail=0

for path in "${required_paths[@]}"; do
  if [[ ! -f "$path" ]]; then
    echo "ERROR: missing required artifact: $path"
    fail=1
  fi
done

contract_path="docs/contracts/proxy-auth-access-sdk.contract.json"
if [[ -f "$contract_path" ]]; then
  if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: jq is required to validate contract JSON"
    fail=1
  else
    if ! jq -e '.public_sdk_surface and .auth_provider_registry_contract and .semver_policy' "$contract_path" >/dev/null; then
      echo "ERROR: contract JSON is missing required top-level sections"
      fail=1
    fi
  fi
fi

if [[ "$fail" -ne 0 ]]; then
  echo "Validation FAILED: proxy auth access module artifacts are incomplete or invalid"
  exit 1
fi

echo "Validation OK: proxy auth access module artifacts are present and contract sections are valid"
