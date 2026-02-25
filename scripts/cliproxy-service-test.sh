#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="./scripts/cliproxy-service.sh"
MOCK_DIR="$(mktemp -d)"
trap 'rm -rf "${MOCK_DIR}"' EXIT

assert_contains() {
  local output="$1"
  local expected="$2"
  if ! printf '%s\n' "${output}" | grep -qF "${expected}"; then
    echo "[FAIL] missing expected output: ${expected}"
    exit 1
  fi
}

assert_not_contains() {
  local output="$1"
  local expected="$2"
  if printf '%s\n' "${output}" | grep -qF "${expected}"; then
    echo "[FAIL] unexpectedly saw: ${expected}"
    exit 1
  fi
}

run_case() {
  local label="$1"
  local expected_code="$2"
  shift 2
  local output
  local status

  echo "## ${label}"

  set +e
  output="$("$@" 2>&1)"
  status=$?
  set -e

  if [ "${status}" -ne "${expected_code}" ]; then
    echo "[FAIL] ${label}: expected exit code ${expected_code}, got ${status}"
    echo "${output}"
    exit 1
  fi
  printf '%s\n' "${output}"
}

make_fake_bin() {
  local mock_dir="$1"
  local platform="${2:-Linux}"
  local unit_present="${3:-1}"
  local launchctl_print_fail="${4:-0}"
  local powershell_present="${5:-0}"

  mkdir -p "${mock_dir}"

  cat > "${mock_dir}/uname" <<'EOF'
#!/usr/bin/env sh
printf '%s\n' "${CLIPROXY_TEST_PLATFORM}"
EOF
  chmod +x "${mock_dir}/uname"

  cat > "${mock_dir}/systemctl" <<'EOF'
#!/usr/bin/env sh
set -eu

if [ "${1:-}" = "list-unit-files" ]; then
  if [ "${CLIPROXY_SYSTEMD_UNIT_PRESENT:-1}" = "1" ]; then
    echo "cliproxyapi-plusplus.service        enabled"
  fi
  exit 0
fi

echo "[fake-systemctl] $*"
exit 0
EOF
  chmod +x "${mock_dir}/systemctl"

  cat > "${mock_dir}/launchctl" <<'EOF'
#!/usr/bin/env sh
set -eu

if [ "${1:-}" = "print" ] && [ "${CLIPROXY_LAUNCHCTL_PRINT_FAIL:-0}" -ne 0 ]; then
  echo "[fake-launchctl] service not found" >&2
  exit 1
fi

echo "[fake-launchctl] $*"
exit 0
EOF
  chmod +x "${mock_dir}/launchctl"

  cat > "${mock_dir}/id" <<'EOF'
#!/usr/bin/env sh
echo "1001"
EOF
  chmod +x "${mock_dir}/id"

  cat > "${mock_dir}/sudo" <<'EOF'
#!/usr/bin/env sh
set -eu
if [ "${1:-}" = "cp" ]; then
  exit 0
fi
"$@"
EOF
  chmod +x "${mock_dir}/sudo"

  cat > "${mock_dir}/cp" <<'EOF'
#!/usr/bin/env sh
exit 0
EOF
  chmod +x "${mock_dir}/cp"

  cat > "${mock_dir}/mkdir" <<'EOF'
#!/usr/bin/env sh
exit 0
EOF
  chmod +x "${mock_dir}/mkdir"

  if [ "${powershell_present}" -eq 1 ]; then
    cat > "${mock_dir}/powershell" <<'EOF'
#!/usr/bin/env sh
echo "[fake-powershell] $*"
exit 0
EOF
    chmod +x "${mock_dir}/powershell"
  else
    rm -f "${mock_dir}/powershell"
  fi

  export CLIPROXY_TEST_PLATFORM="${platform}"
  export CLIPROXY_SYSTEMD_UNIT_PRESENT="${unit_present}"
  export CLIPROXY_LAUNCHCTL_PRINT_FAIL="${launchctl_print_fail}"
}

run_case "usage when no args" 1 env PATH="${MOCK_DIR}:$PATH" "${SCRIPT_PATH}"
run_case "usage on invalid action" 1 env PATH="${MOCK_DIR}:$PATH" "${SCRIPT_PATH}" nope

make_fake_bin "${MOCK_DIR}" "Linux" "1" "0" "0"

run_linux_status="$(run_case "linux status prints status output" 0 env PATH="${MOCK_DIR}:$PATH" CLIPROXY_TEST_PLATFORM=Linux "${SCRIPT_PATH}" status)"
assert_contains "${run_linux_status}" "[fake-systemctl] --no-pager status cliproxyapi-plusplus"

run_linux_start_missing="$(run_case "linux start fails when unit missing" 1 env PATH="${MOCK_DIR}:$PATH" CLIPROXY_TEST_PLATFORM=Linux CLIPROXY_SYSTEMD_UNIT_PRESENT=0 "${SCRIPT_PATH}" start)"
assert_contains "${run_linux_start_missing}" "systemd unit missing. Run: task service:install"

make_fake_bin "${MOCK_DIR}" "Linux" "1" "0" "0"
run_linux_start="$(run_case "linux start with installed unit" 0 env PATH="${MOCK_DIR}:$PATH" CLIPROXY_TEST_PLATFORM=Linux CLIPROXY_SYSTEMD_UNIT_PRESENT=1 "${SCRIPT_PATH}" start)"
assert_contains "${run_linux_start}" "[OK] started cliproxyapi-plusplus"

make_fake_bin "${MOCK_DIR}" "Darwin" "1" "0" "0"
run_macos_status="$(run_case "mac status prints launchctl output" 0 env PATH="${MOCK_DIR}:$PATH" CLIPROXY_TEST_PLATFORM=Darwin "${SCRIPT_PATH}" status)"
assert_contains "${run_macos_status}" "[fake-launchctl]"

make_fake_bin "${MOCK_DIR}" "Windows_NT" "1" "0" "0"
run_windows_status="$(run_case "windows status warns when powershell is missing" 0 env PATH="${MOCK_DIR}:$PATH" CLIPROXY_TEST_PLATFORM=Windows_NT "${SCRIPT_PATH}" status)"
assert_contains "${run_windows_status}" "check service status in Windows Service Manager"

make_fake_bin "${MOCK_DIR}" "Linux" "1" "0" "1"
run_install="$(run_case "linux install command succeeds" 0 env PATH="${MOCK_DIR}:$PATH" CLIPROXY_TEST_PLATFORM=Linux "${SCRIPT_PATH}" install)"
assert_contains "${run_install}" "[OK] systemd service installed and started"

echo "[OK] cliproxy service helper tests passed"
