#!/usr/bin/env bash
set -euo pipefail

script_under_test="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/check-phase-doc-placeholder-tokens.sh"

run_case() {
  local label="$1"
  local expect_exit="$2"
  local expected_text="$3"
  local test_root="$4"

  local output status
  output=""
  status=0

  set +e
  output="$(cd "$test_root" && "$script_under_test" 2>&1)"
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

# Test 1: No planning reports directory - should pass
testdir1="$tmpdir/test1"
mkdir -p "$testdir1/docs/planning"
run_case "pass with no reports directory" 0 "no unresolved placeholder-like tokens" "$testdir1"

# Test 2: Empty reports directory - should pass
testdir2="$tmpdir/test2"
mkdir -p "$testdir2/docs/planning/reports"
run_case "pass with empty reports directory" 0 "no unresolved placeholder-like tokens" "$testdir2"

# Test 3: Clean report with no placeholders - should pass
testdir3="$tmpdir/test3"
mkdir -p "$testdir3/docs/planning/reports"
cat >"$testdir3/docs/planning/reports/implementation-2026-02-23.md" <<'EOF'
# Implementation Report

## Status
All items implemented successfully.

## Tasks
- CPB-0001: Feature A
- CPB-0002: Feature B

## Notes
Everything is properly defined and implemented.
EOF
run_case "pass with clean report" 0 "no unresolved placeholder-like tokens" "$testdir3"

# Test 4: Natural language "undefined" mention - should pass
testdir4="$tmpdir/test4"
mkdir -p "$testdir4/docs/planning/reports"
cat >"$testdir4/docs/planning/reports/implementation-2026-02-23.md" <<'EOF'
# Implementation Report

## Notes
The behavior is undefined in edge cases according to the specification.
Some fields remain undefined until configuration is loaded.
EOF
run_case "pass with natural language undefined" 0 "no unresolved placeholder-like tokens" "$testdir4"

# Test 5: undefinedBKM- pattern - should fail
testdir5="$tmpdir/test5"
mkdir -p "$testdir5/docs/planning/reports"
cat >"$testdir5/docs/planning/reports/implementation-2026-02-23.md" <<'EOF'
# Implementation Report

## Tasks
- undefinedBKM-001: Feature A
- CPB-0002: Feature B
EOF
run_case "fail with undefinedBKM- token" 1 "unresolved placeholder-like tokens detected" "$testdir5"

# Test 6: undefinedXYZundefined pattern - should fail
testdir6="$tmpdir/test6"
mkdir -p "$testdir6/docs/planning/reports"
cat >"$testdir6/docs/planning/reports/implementation-2026-02-23.md" <<'EOF'
# Implementation Report

## Tasks
- undefinedCPB0001undefined: Feature A
- CPB-0002: Feature B
EOF
run_case "fail with undefinedXundefined token" 1 "unresolved placeholder-like tokens detected" "$testdir6"

# Test 7: undefined with uppercase/numbers - should fail
testdir7="$tmpdir/test7"
mkdir -p "$testdir7/docs/planning/reports"
cat >"$testdir7/docs/planning/reports/implementation-2026-02-23.md" <<'EOF'
# Implementation Report

## Tasks
- undefinedCPB_001undefined: Feature A
EOF
run_case "fail with undefined_uppercase_undefined" 1 "unresolved placeholder-like tokens detected" "$testdir7"

# Test 8: undefinedBKM with hyphens - should fail
testdir8="$tmpdir/test8"
mkdir -p "$testdir8/docs/planning/reports"
cat >"$testdir8/docs/planning/reports/implementation-2026-02-23.md" <<'EOF'
# Implementation Report

Reference: undefinedBKM-test-123
EOF
run_case "fail with undefinedBKM-hyphenated" 1 "unresolved placeholder-like tokens detected" "$testdir8"

# Test 9: Multiple reports with mixed content - should fail if any bad
testdir9="$tmpdir/test9"
mkdir -p "$testdir9/docs/planning/reports"
cat >"$testdir9/docs/planning/reports/report1.md" <<'EOF'
# Report 1
Clean content.
EOF
cat >"$testdir9/docs/planning/reports/report2.md" <<'EOF'
# Report 2
Has undefinedBKM-001 placeholder.
EOF
run_case "fail with placeholder in second report" 1 "unresolved placeholder-like tokens detected" "$testdir9"

# Test 10: undefinedBKM in code block - should still fail
testdir10="$tmpdir/test10"
mkdir -p "$testdir10/docs/planning/reports"
cat >"$testdir10/docs/planning/reports/implementation.md" <<'EOF'
# Implementation Report

Example template:
```
ID: undefinedBKM-placeholder
```
EOF
run_case "fail with placeholder in code block" 1 "unresolved placeholder-like tokens detected" "$testdir10"

# Test 11: undefined with lowercase only - should pass
testdir11="$tmpdir/test11"
mkdir -p "$testdir11/docs/planning/reports"
cat >"$testdir11/docs/planning/reports/implementation.md" <<'EOF'
# Implementation Report

The value is undefined in this context.
We have undefined behavior here.
The undefinedvariable is not set.
EOF
run_case "pass with lowercase undefined" 0 "no unresolved placeholder-like tokens" "$testdir11"

# Test 12: Edge case - undefined at line boundaries
testdir12="$tmpdir/test12"
mkdir -p "$testdir12/docs/planning/reports"
cat >"$testdir12/docs/planning/reports/implementation.md" <<'EOF'
# Implementation Report

Task: undefinedCPB-001undefined
Status: Complete
EOF
run_case "fail with undefined at line boundaries" 1 "unresolved placeholder-like tokens detected" "$testdir12"

# Test 13: Verify line numbers in output
testdir13="$tmpdir/test13"
mkdir -p "$testdir13/docs/planning/reports"
cat >"$testdir13/docs/planning/reports/test.md" <<'EOF'
Line 1
Line 2
Line 3 undefinedBKM-test
Line 4
EOF
run_case "fail with line number in output" 1 "test.md:3:" "$testdir13"

# Test 14: Non-markdown files should be ignored
testdir14="$tmpdir/test14"
mkdir -p "$testdir14/docs/planning/reports"
cat >"$testdir14/docs/planning/reports/data.txt" <<'EOF'
This has undefinedBKM-001 but is not markdown
EOF
cat >"$testdir14/docs/planning/reports/report.md" <<'EOF'
Clean markdown report
EOF
run_case "pass with placeholder in non-markdown file" 0 "no unresolved placeholder-like tokens" "$testdir14"

# Test 15: undefinedBKM_ with underscores - should fail
testdir15="$tmpdir/test15"
mkdir -p "$testdir15/docs/planning/reports"
cat >"$testdir15/docs/planning/reports/report.md" <<'EOF'
Task ID: undefinedBKM_test_001
EOF
run_case "fail with undefinedBKM_ underscores" 1 "unresolved placeholder-like tokens detected" "$testdir15"

# Test 16: Nested undefined patterns - should fail
testdir16="$tmpdir/test16"
mkdir -p "$testdir16/docs/planning/reports"
cat >"$testdir16/docs/planning/reports/report.md" <<'EOF'
Pattern: undefined123undefined
Another: undefinedABC-123undefined
EOF
run_case "fail with nested undefined patterns" 1 "unresolved placeholder-like tokens detected" "$testdir16"

# Test 17: Script location check
echo "===== verify script uses correct root directory ====="
if ! rg -q 'ROOT.*dirname.*BASH_SOURCE' "$script_under_test"; then
  echo "[FAIL] script doesn't compute root directory correctly"
  exit 1
fi
echo "[OK] Script computes root directory"

# Test 18: Script checks correct path
echo "===== verify script checks docs/planning/reports ====="
if ! rg -q 'docs/planning/reports' "$script_under_test"; then
  echo "[FAIL] script doesn't check docs/planning/reports"
  exit 1
fi
echo "[OK] Script checks correct path"

# Test 19: Script uses correct pattern
echo "===== verify script uses correct regex pattern ====="
if ! rg -q "undefinedBKM-.*undefined.*undefined" "$script_under_test"; then
  echo "[FAIL] script doesn't have correct pattern"
  exit 1
fi
echo "[OK] Script has correct pattern"

echo "[OK] check-phase-doc-placeholder-tokens script test suite passed"