#!/usr/bin/env bash
set -euo pipefail

script_under_test="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/check-docs-secret-samples.sh"

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

echo "===== verify script has secret patterns ====="

if ! rg -q "sk-\[A-Za-z0-9\]" "$script_under_test"; then
  echo "[FAIL] script missing OpenAI API key pattern"
  exit 1
fi

if ! rg -q "ghp_\[A-Za-z0-9\]" "$script_under_test"; then
  echo "[FAIL] script missing GitHub token pattern"
  exit 1
fi

if ! rg -q "AKIA\[0-9A-Z\]" "$script_under_test"; then
  echo "[FAIL] script missing AWS key pattern"
  exit 1
fi

if ! rg -q "AIza\[0-9A-Za-z_-\]" "$script_under_test"; then
  echo "[FAIL] script missing Google API key pattern"
  exit 1
fi

if ! rg -q "BEGIN.*KEY" "$script_under_test"; then
  echo "[FAIL] script missing private key pattern"
  exit 1
fi

echo "[OK] Script has secret patterns"

echo "===== verify script has allowed context patterns ====="

if ! rg -q 'YOUR' "$script_under_test"; then
  echo "[FAIL] script missing YOUR placeholder detection"
  exit 1
fi

if ! rg -q 'REDACTED' "$script_under_test"; then
  echo "[FAIL] script missing REDACTED detection"
  exit 1
fi

if ! rg -q 'example' "$script_under_test"; then
  echo "[FAIL] script missing example detection"
  exit 1
fi

if ! rg -q 'dummy' "$script_under_test"; then
  echo "[FAIL] script missing dummy detection"
  exit 1
fi

if ! rg -q 'placeholder' "$script_under_test"; then
  echo "[FAIL] script missing placeholder detection"
  exit 1
fi

echo "[OK] Script has allowed context patterns"

echo "===== verify script has proper file exclusions ====="

if ! rg -q -- "--glob '!docs/node_modules/\*\*'" "$script_under_test"; then
  echo "[FAIL] script doesn't exclude node_modules"
  exit 1
fi

if ! rg -q -- "--glob '!\*\*/\*\.min\.\*'" "$script_under_test"; then
  echo "[FAIL] script doesn't exclude minified files"
  exit 1
fi

if ! rg -q -- "--glob '!\*\*/\*\.svg'" "$script_under_test"; then
  echo "[FAIL] script doesn't exclude svg files"
  exit 1
fi

if ! rg -q -- "--glob '!\*\*/\*\.lock'" "$script_under_test"; then
  echo "[FAIL] script doesn't exclude lock files"
  exit 1
fi

echo "[OK] Script has proper file exclusions"

echo "===== verify script searches correct paths ====="

if ! rg -q 'docs README.md README_CN.md examples' "$script_under_test"; then
  echo "[FAIL] script doesn't search expected paths"
  exit 1
fi

echo "[OK] Script searches correct paths"

echo "===== verify script uses ripgrep with PCRE2 ====="

if ! rg -q "rg.*--pcre2" "$script_under_test"; then
  echo "[FAIL] script doesn't use PCRE2"
  exit 1
fi

if ! rg -q "rg.*--hidden" "$script_under_test"; then
  echo "[FAIL] script doesn't search hidden files"
  exit 1
fi

echo "[OK] Script uses ripgrep with PCRE2"

echo "===== verify script has proper messages ====="

if ! rg -q 'docs secret sample check passed' "$script_under_test"; then
  echo "[FAIL] script missing success message"
  exit 1
fi

if ! rg -q 'Potential secret detected' "$script_under_test"; then
  echo "[FAIL] script missing violation message"
  exit 1
fi

if ! rg -q 'Secret sample check failed' "$script_under_test"; then
  echo "[FAIL] script missing failure message"
  exit 1
fi

echo "[OK] Script has proper messages"

echo "===== verify script uses temp files ====="

if ! rg -q 'mktemp' "$script_under_test"; then
  echo "[FAIL] script doesn't use mktemp"
  exit 1
fi

if ! rg -q 'trap.*rm.*EXIT' "$script_under_test"; then
  echo "[FAIL] script doesn't clean up temp files"
  exit 1
fi

echo "[OK] Script uses temp files properly"

echo "===== verify script has context matching logic ====="

if ! rg -q 'allowed_context' "$script_under_test"; then
  echo "[FAIL] script missing allowed_context variable"
  exit 1
fi

if ! rg -q 'line_content' "$script_under_test"; then
  echo "[FAIL] script doesn't extract line content"
  exit 1
fi

if ! rg -q 'violations' "$script_under_test"; then
  echo "[FAIL] script doesn't track violations"
  exit 1
fi

echo "[OK] Script has context matching logic"

echo "===== verify script loops through patterns ====="

if ! rg -q 'for pattern in.*patterns' "$script_under_test"; then
  echo "[FAIL] script doesn't loop through patterns"
  exit 1
fi

echo "[OK] Script loops through patterns"

echo "===== verify script processes hits ====="

if ! rg -q 'while IFS=.*read' "$script_under_test"; then
  echo "[FAIL] script doesn't read hits line by line"
  exit 1
fi

echo "[OK] Script processes hits"

echo "[OK] check-docs-secret-samples script test suite passed"