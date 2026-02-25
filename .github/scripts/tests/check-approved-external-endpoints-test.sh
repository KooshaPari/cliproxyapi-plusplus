#!/usr/bin/env bash
set -euo pipefail

script_under_test="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/check-approved-external-endpoints.sh"

echo "===== verify script structure ====="

if ! head -n 1 "$script_under_test" | rg -q "^#!/usr/bin/env bash"; then
  echo "[FAIL] script missing proper shebang"
  exit 1
fi

if ! head -n 5 "$script_under_test" | rg -q "set -euo pipefail"; then
  echo "[FAIL] script missing set -euo pipefail"
  exit 1
fi

echo "[OK] Script has proper structure"

echo "===== verify script has required components ====="

if ! rg -q 'policy_file.*approved-external-endpoints.txt' "$script_under_test"; then
  echo "[FAIL] script doesn't reference policy file"
  exit 1
fi

if ! rg -q 'matches_policy\(\)' "$script_under_test"; then
  echo "[FAIL] script missing matches_policy function"
  exit 1
fi

if ! rg -q 'mapfile -t approved_hosts' "$script_under_test"; then
  echo "[FAIL] script doesn't read approved hosts"
  exit 1
fi

if ! rg -q 'mapfile -t discovered_hosts' "$script_under_test"; then
  echo "[FAIL] script doesn't discover hosts"
  exit 1
fi

if ! rg -q "https\?://" "$script_under_test"; then
  echo "[FAIL] script missing URL discovery pattern"
  exit 1
fi

echo "[OK] Script has URL discovery logic"

echo "===== verify script has exclusions ====="

if ! rg -q 'localhost.*127.0.0.1.*0.0.0.0' "$script_under_test"; then
  echo "[FAIL] script missing localhost exclusions"
  exit 1
fi

if ! rg -q 'example.com' "$script_under_test"; then
  echo "[FAIL] script missing example.com exclusions"
  exit 1
fi

if ! rg -q 'proxy.com.*proxy.local' "$script_under_test"; then
  echo "[FAIL] script missing proxy exclusions"
  exit 1
fi

if ! rg -q '%|\{' "$script_under_test"; then
  echo "[FAIL] script missing template variable exclusions"
  exit 1
fi

echo "[OK] Script has proper exclusions"

echo "===== verify script has proper glob patterns ====="

if ! rg -q -- "--glob '!docs/\*\*'" "$script_under_test"; then
  echo "[FAIL] script doesn't exclude docs"
  exit 1
fi

if ! rg -q -- "--glob '!\*\*/\*_test.go'" "$script_under_test"; then
  echo "[FAIL] script doesn't exclude test files"
  exit 1
fi

if ! rg -q -- "--glob '!\*\*/node_modules/\*\*'" "$script_under_test"; then
  echo "[FAIL] script doesn't exclude node_modules"
  exit 1
fi

echo "[OK] Script has proper glob patterns"

echo "===== verify script searches correct paths ====="

if ! rg -q 'cmd pkg sdk scripts .github/workflows' "$script_under_test"; then
  echo "[FAIL] script doesn't search expected directories"
  exit 1
fi

if ! rg -q 'README.md README_CN.md' "$script_under_test"; then
  echo "[FAIL] script doesn't search README files"
  exit 1
fi

echo "[OK] Script searches correct paths"

echo "===== verify script has error handling ====="

if ! rg -q 'Missing policy file' "$script_under_test"; then
  echo "[FAIL] script missing policy file check"
  exit 1
fi

if ! rg -q 'No approved hosts in policy file' "$script_under_test"; then
  echo "[FAIL] script missing empty policy check"
  exit 1
fi

if ! rg -q 'Found external hosts not in' "$script_under_test"; then
  echo "[FAIL] script missing violation message"
  exit 1
fi

echo "[OK] Script has proper error handling"

echo "===== verify script has success message ====="

if ! rg -q 'external endpoint policy check passed' "$script_under_test"; then
  echo "[FAIL] script missing success message"
  exit 1
fi

echo "[OK] Script has success message"

echo "===== verify script handles case-insensitive matching ====="

if ! rg -q "tr '\[:upper:\]' '\[:lower:\]'" "$script_under_test"; then
  echo "[FAIL] script doesn't normalize case"
  exit 1
fi

echo "[OK] Script handles case-insensitive matching"

echo "===== verify script has subdomain matching logic ====="

if ! rg -q '\*\.".*approved' "$script_under_test"; then
  echo "[FAIL] script missing subdomain matching"
  exit 1
fi

echo "[OK] Script has subdomain matching"

echo "===== verify script has correct exit behavior ====="

if ! rg -q 'exit 1' "$script_under_test"; then
  echo "[FAIL] script missing exit 1 for failures"
  exit 1
fi

echo "[OK] Script has correct exit behavior"

echo "===== verify script uses grep for filtering ====="

if ! rg -q "grep -Ev" "$script_under_test"; then
  echo "[FAIL] script doesn't use grep for filtering"
  exit 1
fi

echo "[OK] Script uses grep for filtering"

echo "===== verify script has host comparison logic ====="

if ! rg -q '\[.*==.*\]' "$script_under_test"; then
  echo "[FAIL] script missing host comparison logic"
  exit 1
fi

echo "[OK] Script has host comparison logic"

echo "===== verify script converts URLs to hosts ====="

if ! rg -q "awk.*print" "$script_under_test"; then
  echo "[FAIL] script doesn't extract hosts from URLs"
  exit 1
fi

if ! rg -q "cut -d/" "$script_under_test"; then
  echo "[FAIL] script doesn't parse URL paths"
  exit 1
fi

if ! rg -q "cut -d:" "$script_under_test"; then
  echo "[FAIL] script doesn't strip ports"
  exit 1
fi

echo "[OK] Script converts URLs to hosts"

echo "[OK] check-approved-external-endpoints script test suite passed"