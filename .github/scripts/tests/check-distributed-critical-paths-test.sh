#!/usr/bin/env bash
set -euo pipefail

script_under_test="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/check-distributed-critical-paths.sh"

run_case() {
  local label="$1"
  local expect_exit="$2"
  local expected_text="$3"
  local test_root="$4"

  local output status
  output=""
  status=0

  set +e
  output="$(cd "$test_root" && bash "$script_under_test" 2>&1)"
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

# Test 1: Verify script has correct test invocations
echo "===== verify script contains expected test patterns ====="
if ! rg -q "TestMultiSourceSecret_FileHandling" "$script_under_test"; then
  echo "[FAIL] script missing TestMultiSourceSecret_FileHandling test"
  exit 1
fi

if ! rg -q "TestMultiSourceSecret_CacheBehavior" "$script_under_test"; then
  echo "[FAIL] script missing TestMultiSourceSecret_CacheBehavior test"
  exit 1
fi

if ! rg -q "TestMultiSourceSecret_Concurrency" "$script_under_test"; then
  echo "[FAIL] script missing TestMultiSourceSecret_Concurrency test"
  exit 1
fi

if ! rg -q "TestAmpModule_OnConfigUpdated_CacheInvalidation" "$script_under_test"; then
  echo "[FAIL] script missing TestAmpModule_OnConfigUpdated_CacheInvalidation test"
  exit 1
fi

if ! rg -q "TestRegisterManagementRoutes" "$script_under_test"; then
  echo "[FAIL] script missing TestRegisterManagementRoutes test"
  exit 1
fi

if ! rg -q "TestEnsureCacheControl" "$script_under_test"; then
  echo "[FAIL] script missing TestEnsureCacheControl test"
  exit 1
fi

if ! rg -q "TestCacheControlOrder" "$script_under_test"; then
  echo "[FAIL] script missing TestCacheControlOrder test"
  exit 1
fi

if ! rg -q "TestCountOpenAIChatTokens" "$script_under_test"; then
  echo "[FAIL] script missing TestCountOpenAIChatTokens test"
  exit 1
fi

if ! rg -q "TestCountClaudeChatTokens" "$script_under_test"; then
  echo "[FAIL] script missing TestCountClaudeChatTokens test"
  exit 1
fi

if ! rg -q "TestBuildProviderMetricsFromSnapshot_FailoverAndQueueTelemetry" "$script_under_test"; then
  echo "[FAIL] script missing TestBuildProviderMetricsFromSnapshot_FailoverAndQueueTelemetry test"
  exit 1
fi

if ! rg -q "TestCacheSignature_BasicStorageAndRetrieval" "$script_under_test"; then
  echo "[FAIL] script missing TestCacheSignature_BasicStorageAndRetrieval test"
  exit 1
fi

if ! rg -q "TestCacheSignature_ExpirationLogic" "$script_under_test"; then
  echo "[FAIL] script missing TestCacheSignature_ExpirationLogic test"
  exit 1
fi

echo "[OK] All expected test patterns found in script"

# Test 2: Verify script validates correct packages
echo "===== verify script validates correct packages ====="
if ! rg -q "./pkg/llmproxy/api/modules/amp" "$script_under_test"; then
  echo "[FAIL] script missing amp package validation"
  exit 1
fi

if ! rg -q "./pkg/llmproxy/runtime/executor" "$script_under_test"; then
  echo "[FAIL] script missing executor package validation"
  exit 1
fi

if ! rg -q "./pkg/llmproxy/usage" "$script_under_test"; then
  echo "[FAIL] script missing usage package validation"
  exit 1
fi

if ! rg -q "./pkg/llmproxy/cache" "$script_under_test"; then
  echo "[FAIL] script missing cache package validation"
  exit 1
fi

echo "[OK] All expected packages found in script"

# Test 3: Verify script uses correct go test flags
echo "===== verify script uses correct go test flags ====="
if ! rg -q "go test -count=1" "$script_under_test"; then
  echo "[FAIL] script missing -count=1 flag"
  exit 1
fi

if ! rg -q "\-run" "$script_under_test"; then
  echo "[FAIL] script missing -run flag"
  exit 1
fi

echo "[OK] Script uses correct go test flags"

# Test 4: Verify script has proper structure
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

# Test 5: Verify script has validation messages
echo "===== verify script has validation messages ====="
if ! rg -q "distributed-critical-paths" "$script_under_test"; then
  echo "[FAIL] script missing validation messages"
  exit 1
fi

if ! rg -q "validating filesystem-sensitive paths" "$script_under_test"; then
  echo "[FAIL] script missing filesystem validation message"
  exit 1
fi

if ! rg -q "validating ops endpoint route registration" "$script_under_test"; then
  echo "[FAIL] script missing ops endpoint validation message"
  exit 1
fi

if ! rg -q "validating compute/cache-sensitive paths" "$script_under_test"; then
  echo "[FAIL] script missing compute/cache validation message"
  exit 1
fi

if ! rg -q "validating queue telemetry to provider metrics path" "$script_under_test"; then
  echo "[FAIL] script missing queue telemetry validation message"
  exit 1
fi

if ! rg -q "validating signature cache primitives" "$script_under_test"; then
  echo "[FAIL] script missing signature cache validation message"
  exit 1
fi

if ! rg -q "all targeted checks passed" "$script_under_test"; then
  echo "[FAIL] script missing success message"
  exit 1
fi

echo "[OK] Script has all validation messages"

# Test 6: Create mock go binary that succeeds
testdir6="$tmpdir/test6"
mkdir -p "$testdir6"
cat >"$testdir6/go" <<'EOF'
#!/usr/bin/env bash
echo "ok  	package/test	0.001s"
exit 0
EOF
chmod +x "$testdir6/go"

# Test with mock successful go command
echo "===== test with mock successful go command ====="
PATH="$testdir6:$PATH" run_case "pass with successful go tests" 0 "all targeted checks passed" "$testdir6"

# Test 7: Create mock go binary that fails
testdir7="$tmpdir/test7"
mkdir -p "$testdir7"
cat >"$testdir7/go" <<'EOF'
#!/usr/bin/env bash
echo "FAIL	package/test	0.001s"
exit 1
EOF
chmod +x "$testdir7/go"

# Test with mock failing go command
echo "===== test with mock failing go command ====="
PATH="$testdir7:$PATH" run_case "fail with failing go tests" 1 "FAIL" "$testdir7"

echo "[OK] check-distributed-critical-paths script test suite passed"