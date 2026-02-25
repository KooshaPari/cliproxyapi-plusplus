#!/usr/bin/env bash
set -euo pipefail

script_under_test="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/check-workflow-token-permissions.sh"

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

# Test 1: No workflows directory - should pass
testdir1="$tmpdir/test1"
mkdir -p "$testdir1/.github"
run_case "pass with no workflows" 0 "workflow token permission check passed" "$testdir1"

# Test 2: Empty workflows directory - should pass
testdir2="$tmpdir/test2"
mkdir -p "$testdir2/.github/workflows"
run_case "pass with empty workflows" 0 "workflow token permission check passed" "$testdir2"

# Test 3: Clean workflow with read-only permissions - should pass
testdir3="$tmpdir/test3"
mkdir -p "$testdir3/.github/workflows"
cat >"$testdir3/.github/workflows/test.yml" <<'EOF'
name: Test
on:
  pull_request:
permissions:
  contents: read
  pull-requests: read
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "pass with read-only permissions" 0 "workflow token permission check passed" "$testdir3"

# Test 4: Workflow with write-all permissions - should fail
testdir4="$tmpdir/test4"
mkdir -p "$testdir4/.github/workflows"
cat >"$testdir4/.github/workflows/test.yml" <<'EOF'
name: Test
on: push
permissions: write-all
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "fail with write-all permissions" 1 "uses permissions: write-all" "$testdir4"

# Test 5: Pull request workflow with disallowed write permission - should fail
testdir5="$tmpdir/test5"
mkdir -p "$testdir5/.github/workflows"
cat >"$testdir5/.github/workflows/pr.yml" <<'EOF'
name: PR
on:
  pull_request:
permissions:
  contents: write
  pull-requests: read
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "fail with contents write on pull_request" 1 "pull_request workflow grants 'contents: write'" "$testdir5"

# Test 6: Pull request workflow with security-events write - should pass
testdir6="$tmpdir/test6"
mkdir -p "$testdir6/.github/workflows"
cat >"$testdir6/.github/workflows/security.yml" <<'EOF'
name: Security
on:
  pull_request:
permissions:
  contents: read
  security-events: write
jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "pass with security-events write" 0 "workflow token permission check passed" "$testdir6"

# Test 7: Pull request workflow with id-token write - should pass
testdir7="$tmpdir/test7"
mkdir -p "$testdir7/.github/workflows"
cat >"$testdir7/.github/workflows/oidc.yml" <<'EOF'
name: OIDC
on:
  pull_request:
permissions:
  contents: read
  id-token: write
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "pass with id-token write" 0 "workflow token permission check passed" "$testdir7"

# Test 8: Pull request workflow with pages write - should pass
testdir8="$tmpdir/test8"
mkdir -p "$testdir8/.github/workflows"
cat >"$testdir8/.github/workflows/pages.yml" <<'EOF'
name: Pages
on:
  pull_request:
permissions:
  pages: write
  contents: read
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "pass with pages write" 0 "workflow token permission check passed" "$testdir8"

# Test 9: Non-pull_request workflow with write permissions - should pass
testdir9="$tmpdir/test9"
mkdir -p "$testdir9/.github/workflows"
cat >"$testdir9/.github/workflows/push.yml" <<'EOF'
name: Push
on:
  push:
    branches: [main]
permissions:
  contents: write
  pull-requests: write
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "pass with write on push event" 0 "workflow token permission check passed" "$testdir9"

# Test 10: Pull request workflow with actions write - should fail
testdir10="$tmpdir/test10"
mkdir -p "$testdir10/.github/workflows"
cat >"$testdir10/.github/workflows/pr.yml" <<'EOF'
name: PR
on:
  pull_request:
permissions:
  actions: write
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "fail with actions write on pull_request" 1 "pull_request workflow grants 'actions: write'" "$testdir10"

# Test 11: Pull request workflow with pull-requests write - should fail
testdir11="$tmpdir/test11"
mkdir -p "$testdir11/.github/workflows"
cat >"$testdir11/.github/workflows/pr.yml" <<'EOF'
name: PR
on:
  pull_request:
permissions:
  pull-requests: write
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "fail with pull-requests write" 1 "pull_request workflow grants 'pull-requests: write'" "$testdir11"

# Test 12: Mixed workflow triggers including pull_request - should fail with write
testdir12="$tmpdir/test12"
mkdir -p "$testdir12/.github/workflows"
cat >"$testdir12/.github/workflows/mixed.yml" <<'EOF'
name: Mixed
on:
  push:
  pull_request:
permissions:
  contents: write
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "fail with write on mixed triggers including pull_request" 1 "pull_request workflow grants 'contents: write'" "$testdir12"

# Test 13: Multiple workflows with one violation
testdir13="$tmpdir/test13"
mkdir -p "$testdir13/.github/workflows"
cat >"$testdir13/.github/workflows/good.yml" <<'EOF'
name: Good
on:
  pull_request:
permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
cat >"$testdir13/.github/workflows/bad.yml" <<'EOF'
name: Bad
on:
  pull_request:
permissions:
  issues: write
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "fail with one bad workflow" 1 "issues: write" "$testdir13"

# Test 14: Workflow with .yaml extension - should be checked
testdir14="$tmpdir/test14"
mkdir -p "$testdir14/.github/workflows"
cat >"$testdir14/.github/workflows/test.yaml" <<'EOF'
name: Test
on:
  pull_request:
permissions:
  checks: write
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "fail with .yaml extension and write permission" 1 "checks: write" "$testdir14"

# Test 15: All three allowed write permissions together - should pass
testdir15="$tmpdir/test15"
mkdir -p "$testdir15/.github/workflows"
cat >"$testdir15/.github/workflows/all-allowed.yml" <<'EOF'
name: All Allowed
on:
  pull_request:
permissions:
  security-events: write
  id-token: write
  pages: write
  contents: read
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "pass with all allowed write permissions" 0 "workflow token permission check passed" "$testdir15"

# Test 16: Job-level permissions (not top-level) - should still be checked
testdir16="$tmpdir/test16"
mkdir -p "$testdir16/.github/workflows"
cat >"$testdir16/.github/workflows/job-perms.yml" <<'EOF'
name: Job Perms
on:
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    permissions:
      deployments: write
    steps:
      - uses: actions/checkout@v4
EOF
run_case "fail with job-level write permission" 1 "deployments: write" "$testdir16"

# Test 17: Packages write permission - should fail
testdir17="$tmpdir/test17"
mkdir -p "$testdir17/.github/workflows"
cat >"$testdir17/.github/workflows/packages.yml" <<'EOF'
name: Packages
on:
  pull_request:
permissions:
  packages: write
jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "fail with packages write" 1 "packages: write" "$testdir17"

# Test 18: Statuses write permission - should fail
testdir18="$tmpdir/test18"
mkdir -p "$testdir18/.github/workflows"
cat >"$testdir18/.github/workflows/statuses.yml" <<'EOF'
name: Statuses
on:
  pull_request:
permissions:
  statuses: write
jobs:
  status:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "fail with statuses write" 1 "statuses: write" "$testdir18"

# Test 19: workflow_run trigger (not pull_request) - should pass with write
testdir19="$tmpdir/test19"
mkdir -p "$testdir19/.github/workflows"
cat >"$testdir19/.github/workflows/workflow-run.yml" <<'EOF'
name: Workflow Run
on:
  workflow_run:
    workflows: ["CI"]
    types: [completed]
permissions:
  contents: write
  pull-requests: write
jobs:
  post-ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "pass with workflow_run trigger" 0 "workflow token permission check passed" "$testdir19"

# Test 20: Schedule trigger - should pass with write
testdir20="$tmpdir/test20"
mkdir -p "$testdir20/.github/workflows"
cat >"$testdir20/.github/workflows/scheduled.yml" <<'EOF'
name: Scheduled
on:
  schedule:
    - cron: '0 0 * * *'
permissions:
  contents: write
jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
EOF
run_case "pass with schedule trigger" 0 "workflow token permission check passed" "$testdir20"

echo "[OK] check-workflow-token-permissions script test suite passed"