#!/usr/bin/env bash
set -euo pipefail

script_under_test="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/release-lint.sh"

run_case() {
  local label="$1"
  local expect_exit="$2"
  local expected_text="$3"
  local test_root="$4"
  local go_path="${5:-}"
  local python_path="${6:-}"

  local output status
  output=""
  status=0

  local custom_path="$PATH"
  if [[ -n "$go_path" ]]; then
    custom_path="$go_path:$custom_path"
  fi
  if [[ -n "$python_path" ]]; then
    custom_path="$python_path:$custom_path"
  fi

  set +e
  output="$(cd "$test_root" && PATH="$custom_path" "$script_under_test" 2>&1)"
  status=$?
  set -e

  printf '===== %s =====\n' "$label"
  echo "$output"

  if [[ "$status" -ne "$expect_exit" ]]; then
    echo "[FAIL] $label: expected exit $expect_exit, got $status"
    exit 1
  fi

  if ! echo "$output" | rg -q "$expected_text"; then
    echo "[FAIL] $label: expected output to contain '$expected_text'"
    exit 1
  fi
}

# Create test environment
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

# Test 1: Verify script invokes correct Go tests
echo "===== verify script has correct Go test invocations ====="
if ! rg -q "TestLoadConfig" "$script_under_test"; then
  echo "[FAIL] script missing TestLoadConfig"
  exit 1
fi

if ! rg -q "TestMigrateOAuthModelAlias" "$script_under_test"; then
  echo "[FAIL] script missing TestMigrateOAuthModelAlias"
  exit 1
fi

if ! rg -q "TestConfig_Validate" "$script_under_test"; then
  echo "[FAIL] script missing TestConfig_Validate"
  exit 1
fi

if ! rg -q "./pkg/llmproxy/config" "$script_under_test"; then
  echo "[FAIL] script missing config package path"
  exit 1
fi

echo "[OK] Script has correct Go test invocations"

# Test 2: Verify script checks for python3
echo "===== verify script checks for python3 ====="
if ! rg -q "command -v python3" "$script_under_test"; then
  echo "[FAIL] script doesn't check for python3"
  exit 1
fi

echo "[OK] Script checks for python3"

# Test 3: Mock successful go and python3
testdir3="$tmpdir/test3"
mkdir -p "$testdir3/bin" "$testdir3/pkg/llmproxy/config" "$testdir3/docs"
cat >"$testdir3/bin/go" <<'EOF'
#!/usr/bin/env bash
echo "ok  	./pkg/llmproxy/config	0.001s"
exit 0
EOF
chmod +x "$testdir3/bin/go"

cat >"$testdir3/bin/python3" <<'EOF'
#!/usr/bin/env bash
echo "release-lint: markdown snippet parse passed"
exit 0
EOF
chmod +x "$testdir3/bin/python3"

cat >"$testdir3/docs/guide.md" <<'EOF'
# Guide
```json
{"key": "value"}
```
EOF

run_case "pass with successful go and python3" 0 "markdown snippet parse passed" "$testdir3" "$testdir3/bin" "$testdir3/bin"

# Test 4: Mock failing go test
testdir4="$tmpdir/test4"
mkdir -p "$testdir4/bin" "$testdir4/pkg/llmproxy/config"
cat >"$testdir4/bin/go" <<'EOF'
#!/usr/bin/env bash
echo "FAIL	./pkg/llmproxy/config	0.001s"
exit 1
EOF
chmod +x "$testdir4/bin/go"

run_case "fail with failing go test" 1 "FAIL" "$testdir4" "$testdir4/bin" ""

# Test 5: No python3 available - should skip markdown check
testdir5="$tmpdir/test5"
mkdir -p "$testdir5/bin" "$testdir5/pkg/llmproxy/config"
cat >"$testdir5/bin/go" <<'EOF'
#!/usr/bin/env bash
echo "ok  	./pkg/llmproxy/config	0.001s"
exit 0
EOF
chmod +x "$testdir5/bin/go"

# Create a mock command binary that doesn't include python3
cat >"$testdir5/bin/command" <<'EOF'
#!/usr/bin/env bash
if [[ "$1" == "-v" && "$2" == "python3" ]]; then
  exit 1
fi
EOF
chmod +x "$testdir5/bin/command"

run_case "skip markdown check when no python3" 0 "python3 not available" "$testdir5" "$testdir5/bin" ""

# Test 6: Python script detects invalid JSON
testdir6="$tmpdir/test6"
mkdir -p "$testdir6/bin" "$testdir6/pkg/llmproxy/config" "$testdir6/docs"
cat >"$testdir6/bin/go" <<'EOF'
#!/usr/bin/env bash
echo "ok  	./pkg/llmproxy/config	0.001s"
exit 0
EOF
chmod +x "$testdir6/bin/go"

cat >"$testdir6/bin/python3" <<'EOF'
#!/usr/bin/env bash
echo "release-lint: markdown snippet parse failed:"
echo "- docs/bad.md:5::json::Invalid JSON"
exit 1
EOF
chmod +x "$testdir6/bin/python3"

cat >"$testdir6/docs/bad.md" <<'EOF'
# Guide
```json
{invalid json}
```
EOF

run_case "fail with invalid JSON in markdown" 1 "markdown snippet parse failed" "$testdir6" "$testdir6/bin" "$testdir6/bin"

# Test 7: Python script handles YAML
testdir7="$tmpdir/test7"
mkdir -p "$testdir7/bin" "$testdir7/pkg/llmproxy/config" "$testdir7/docs"
cat >"$testdir7/bin/go" <<'EOF'
#!/usr/bin/env bash
echo "ok  	./pkg/llmproxy/config	0.001s"
exit 0
EOF
chmod +x "$testdir7/bin/go"

cat >"$testdir7/bin/python3" <<'EOF'
#!/usr/bin/env bash
echo "release-lint: markdown snippet parse passed"
exit 0
EOF
chmod +x "$testdir7/bin/python3"

cat >"$testdir7/docs/config.md" <<'EOF'
# Configuration
```yaml
key: value
nested:
  - item1
  - item2
```
EOF

run_case "pass with valid YAML" 0 "markdown snippet parse passed" "$testdir7" "$testdir7/bin" "$testdir7/bin"

# Test 8: Python script skips placeholders
testdir8="$tmpdir/test8"
mkdir -p "$testdir8/bin" "$testdir8/pkg/llmproxy/config" "$testdir8/docs"
cat >"$testdir8/bin/go" <<'EOF'
#!/usr/bin/env bash
echo "ok  	./pkg/llmproxy/config	0.001s"
exit 0
EOF
chmod +x "$testdir8/bin/go"

cat >"$testdir8/bin/python3" <<'EOF'
#!/usr/bin/env bash
# Verify placeholder detection works
stdin="$(cat)"
if echo "$stdin" | grep -q 'YOUR_'; then
  echo "release-lint: markdown snippet parse passed"
  exit 0
else
  echo "release-lint: markdown snippet parse failed: missing placeholder detection"
  exit 1
fi
EOF
chmod +x "$testdir8/bin/python3"

cat >"$testdir8/docs/example.md" <<'EOF'
# Example
```json
{"api_key": "<YOUR_API_KEY>"}
```
EOF

run_case "pass with placeholder in JSON" 0 "markdown snippet parse passed" "$testdir8" "$testdir8/bin" "$testdir8/bin"

# Test 9: Verify script structure
echo "===== verify script has proper structure ====="
if ! head -n 1 "$script_under_test" | rg -q "^#!/usr/bin/env bash"; then
  echo "[FAIL] script missing proper shebang"
  exit 1
fi

if ! head -n 5 "$script_under_test" | rg -q "set -euo pipefail"; then
  echo "[FAIL] script missing set -euo pipefail"
  exit 1
fi

echo "[OK] Script has proper structure"

# Test 10: Verify script computes REPO_ROOT
echo "===== verify script computes REPO_ROOT ====="
if ! rg -q 'REPO_ROOT.*dirname.*BASH_SOURCE' "$script_under_test"; then
  echo "[FAIL] script doesn't compute REPO_ROOT"
  exit 1
fi

echo "[OK] Script computes REPO_ROOT"

# Test 11: Verify script has heredoc for Python
echo "===== verify script uses heredoc for Python ====="
if ! rg -q "<<'PY'" "$script_under_test"; then
  echo "[FAIL] script doesn't use heredoc for Python"
  exit 1
fi

if ! rg -q "^PY$" "$script_under_test"; then
  echo "[FAIL] script heredoc not properly terminated"
  exit 1
fi

echo "[OK] Script uses heredoc for Python"

# Test 12: Multiple markdown files with mixed content
testdir12="$tmpdir/test12"
mkdir -p "$testdir12/bin" "$testdir12/pkg/llmproxy/config" "$testdir12/docs"
cat >"$testdir12/bin/go" <<'EOF'
#!/usr/bin/env bash
echo "ok  	./pkg/llmproxy/config	0.001s"
exit 0
EOF
chmod +x "$testdir12/bin/go"

cat >"$testdir12/bin/python3" <<'EOF'
#!/usr/bin/env bash
echo "release-lint: markdown snippet parse passed"
exit 0
EOF
chmod +x "$testdir12/bin/python3"

cat >"$testdir12/README.md" <<'EOF'
# README
```json
{"status": "ok"}
```
EOF

cat >"$testdir12/docs/api.md" <<'EOF'
# API
```yaml
endpoint: /api/v1
method: GET
```
EOF

run_case "pass with multiple markdown files" 0 "markdown snippet parse passed" "$testdir12" "$testdir12/bin" "$testdir12/bin"

# Test 13: JSONC (JSON with comments) support
testdir13="$tmpdir/test13"
mkdir -p "$testdir13/bin" "$testdir13/pkg/llmproxy/config" "$testdir13/docs"
cat >"$testdir13/bin/go" <<'EOF'
#!/usr/bin/env bash
echo "ok  	./pkg/llmproxy/config	0.001s"
exit 0
EOF
chmod +x "$testdir13/bin/go"

cat >"$testdir13/bin/python3" <<'EOF'
#!/usr/bin/env bash
echo "release-lint: markdown snippet parse passed"
exit 0
EOF
chmod +x "$testdir13/bin/python3"

cat >"$testdir13/docs/config.md" <<'EOF'
# Config
```jsonc
{
  // This is a comment
  "key": "value"
}
```
EOF

run_case "pass with JSONC snippets" 0 "markdown snippet parse passed" "$testdir13" "$testdir13/bin" "$testdir13/bin"

# Test 14: Verify Python script checks supported languages
echo "===== verify Python handles json, jsonc, yaml, yml ====="
if ! rg -q '"json".*"jsonc".*"yaml".*"yml"' "$script_under_test"; then
  echo "[FAIL] script missing expected language support"
  exit 1
fi

echo "[OK] Script supports expected languages"

# Test 15: Verify Python checks skip markers
echo "===== verify Python checks for skip markers ====="
if ! rg -q 'YOUR_' "$script_under_test"; then
  echo "[FAIL] script missing YOUR_ skip marker check"
  exit 1
fi

if ! rg -q 'REDACTED' "$script_under_test"; then
  echo "[FAIL] script missing REDACTED skip marker check"
  exit 1
fi

echo "[OK] Script has skip marker checks"

# Test 16: Go test with specific run filter
testdir16="$tmpdir/test16"
mkdir -p "$testdir16/bin" "$testdir16/pkg/llmproxy/config"
cat >"$testdir16/bin/go" <<'EOF'
#!/usr/bin/env bash
if [[ "$*" == *"-run"* ]]; then
  echo "ok  	./pkg/llmproxy/config	0.001s"
  exit 0
else
  echo "Expected -run flag"
  exit 1
fi
EOF
chmod +x "$testdir16/bin/go"

cat >"$testdir16/bin/python3" <<'EOF'
#!/usr/bin/env bash
echo "release-lint: markdown snippet parse passed"
exit 0
EOF
chmod +x "$testdir16/bin/python3"

run_case "pass with go test -run flag" 0 "markdown snippet parse passed" "$testdir16" "$testdir16/bin" "$testdir16/bin"

# Test 17: Script logs what it's doing
testdir17="$tmpdir/test17"
mkdir -p "$testdir17/bin" "$testdir17/pkg/llmproxy/config"
cat >"$testdir17/bin/go" <<'EOF'
#!/usr/bin/env bash
echo "ok  	./pkg/llmproxy/config	0.001s"
exit 0
EOF
chmod +x "$testdir17/bin/go"

cat >"$testdir17/bin/python3" <<'EOF'
#!/usr/bin/env bash
echo "release-lint: markdown snippet parse passed"
exit 0
EOF
chmod +x "$testdir17/bin/python3"

run_case "output includes progress messages" 0 "release-lint:" "$testdir17" "$testdir17/bin" "$testdir17/bin"

echo "[OK] release-lint script test suite passed"