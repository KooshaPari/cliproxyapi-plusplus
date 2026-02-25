#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SYSTEMD_UNIT_NAME="cliproxyapi-plusplus"
SYSTEMD_UNIT_PATH="/etc/systemd/system/${SYSTEMD_UNIT_NAME}.service"
SYSTEMD_ENV_PATH="/etc/default/${SYSTEMD_UNIT_NAME}"
LAUNCHD_LABEL="com.router-for-me.cliproxyapi-plusplus"
LAUNCHD_PLIST_SRC="$REPO_ROOT/examples/launchd/${LAUNCHD_LABEL}.plist"
LAUNCHD_PLIST_DST="$HOME/Library/LaunchAgents/${LAUNCHD_LABEL}.plist"
SYSTEMD_SERVICE_SRC="$REPO_ROOT/examples/systemd/${SYSTEMD_UNIT_NAME}.service"
SYSTEMD_ENV_SRC="$REPO_ROOT/examples/systemd/${SYSTEMD_UNIT_NAME}.env"
WINDOWS_SCRIPT_SRC="$REPO_ROOT/examples/windows/cliproxyapi-plusplus-service.ps1"

usage() {
  cat <<'USAGE'
Usage: cliproxy-service.sh <install|start|stop|restart|status>

Examples:
  ./scripts/cliproxy-service.sh status
  ./scripts/cliproxy-service.sh start
  ./scripts/cliproxy-service.sh install
USAGE
}

need_cmd() {
  cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "[FAIL] required command '$cmd' not found" >&2
    exit 1
  fi
}

os() {
  uname_s="$(uname -s 2>/dev/null || true)"
  case "$uname_s" in
    Darwin*)
      echo darwin
      ;;
    Linux*)
      echo linux
      ;;
    *MINGW*|*MSYS*|*CYGWIN*|Windows_NT*)
      echo windows
      ;;
    *)
      echo unknown
      ;;
  esac
}

platform="$(os)"

install_linux() {
  need_cmd sudo
  need_cmd systemctl
  need_cmd cp

  if [ ! -f "$SYSTEMD_SERVICE_SRC" ] || [ ! -f "$SYSTEMD_ENV_SRC" ]; then
    echo "[FAIL] missing example service files under examples/systemd" >&2
    exit 1
  fi

  sudo mkdir -p /etc/default /var/log/cliproxyapi
  sudo cp "$SYSTEMD_SERVICE_SRC" "$SYSTEMD_UNIT_PATH"
  sudo cp "$SYSTEMD_ENV_SRC" "$SYSTEMD_ENV_PATH"
  sudo systemctl daemon-reload
  sudo systemctl enable --now "$SYSTEMD_UNIT_NAME"
  echo "[OK] systemd service installed and started: $SYSTEMD_UNIT_NAME"
}

install_macos() {
  need_cmd launchctl

  if [ ! -f "$LAUNCHD_PLIST_SRC" ]; then
    echo "[FAIL] missing launchd example plist at $LAUNCHD_PLIST_SRC" >&2
    exit 1
  fi

  mkdir -p "$HOME/Library/LaunchAgents"
  cp "$LAUNCHD_PLIST_SRC" "$LAUNCHD_PLIST_DST"
  launchctl bootstrap "gui/$(id -u)" "$LAUNCHD_PLIST_DST"
  launchctl kickstart -k "gui/$(id -u)/$LAUNCHD_LABEL"
  echo "[OK] launchd service installed and started: $LAUNCHD_LABEL"
}

install_windows() {
  if command -v powershell >/dev/null 2>&1; then
    if [ ! -f "$WINDOWS_SCRIPT_SRC" ]; then
      echo "[FAIL] missing Windows service script at $WINDOWS_SCRIPT_SRC" >&2
      exit 1
    fi
    echo "[INFO] Run in elevated PowerShell (Windows host):"
    echo "  powershell -ExecutionPolicy Bypass -File \"$WINDOWS_SCRIPT_SRC\" -Action install"
    return
  fi
  echo "[FAIL] PowerShell not available in this environment" >&2
  exit 1
}

start_linux() {
  need_cmd systemctl
  if ! systemctl list-unit-files | grep -q "^${SYSTEMD_UNIT_NAME}\\.service"; then
    echo "[FAIL] systemd unit missing. Run: task service:install"
    exit 1
  fi
  sudo systemctl start "$SYSTEMD_UNIT_NAME"
  echo "[OK] started $SYSTEMD_UNIT_NAME"
}

start_macos() {
  need_cmd launchctl
  launchctl bootstrap "gui/$(id -u)" "$LAUNCHD_PLIST_DST" >/dev/null 2>&1 || true
  launchctl kickstart -k "gui/$(id -u)/$LAUNCHD_LABEL"
  echo "[OK] started $LAUNCHD_LABEL"
}

stop_linux() {
  need_cmd systemctl
  if ! systemctl list-unit-files | grep -q "^${SYSTEMD_UNIT_NAME}\\.service"; then
    echo "[FAIL] systemd unit missing. Run: task service:install"
    exit 1
  fi
  sudo systemctl stop "$SYSTEMD_UNIT_NAME"
  echo "[OK] stopped $SYSTEMD_UNIT_NAME"
}

stop_macos() {
  need_cmd launchctl
  launchctl bootout "gui/$(id -u)" "$LAUNCHD_LABEL" >/dev/null 2>&1 || true
  echo "[OK] stopped $LAUNCHD_LABEL"
}

restart_linux() {
  need_cmd systemctl
  if ! systemctl list-unit-files | grep -q "^${SYSTEMD_UNIT_NAME}\\.service"; then
    echo "[FAIL] systemd unit missing. Run: task service:install"
    exit 1
  fi
  sudo systemctl restart "$SYSTEMD_UNIT_NAME"
  echo "[OK] restarted $SYSTEMD_UNIT_NAME"
}

restart_macos() {
  need_cmd launchctl
  launchctl kickstart -k "gui/$(id -u)/$LAUNCHD_LABEL"
  echo "[OK] restarted $LAUNCHD_LABEL"
}

status_linux() {
  need_cmd systemctl
  systemctl --no-pager status "$SYSTEMD_UNIT_NAME" || true
}

status_macos() {
  need_cmd launchctl
  launchctl print "gui/$(id -u)/$LAUNCHD_LABEL" 2>/dev/null || {
    echo "[WARN] launchd service not loaded: $LAUNCHD_LABEL"
    exit 0
  }
}

status_windows() {
  if command -v powershell >/dev/null 2>&1; then
    powershell -Command "Get-Service -Name $SYSTEMD_UNIT_NAME -ErrorAction SilentlyContinue | Format-List -Property Name,Status,StartType"
    exit 0
  fi
  echo "[WARN] check service status in Windows Service Manager or use PowerShell script"
}

if [ "$#" -ne 1 ]; then
  usage
  exit 1
fi

action="$1"

case "$action" in
  install)
    case "$platform" in
      linux) install_linux ;;
      darwin) install_macos ;;
      windows) install_windows ;;
      *) echo "[FAIL] unsupported platform: $platform"; exit 1 ;;
    esac
    ;;
  start)
    case "$platform" in
      linux) start_linux ;;
      darwin) start_macos ;;
      windows) echo "[INFO] Start with PowerShell: powershell -ExecutionPolicy Bypass -File \"$WINDOWS_SCRIPT_SRC\" -Action start" ;;
      *) echo "[FAIL] unsupported platform: $platform"; exit 1 ;;
    esac
    ;;
  stop)
    case "$platform" in
      linux) stop_linux ;;
      darwin) stop_macos ;;
      windows) echo "[INFO] Stop with PowerShell: powershell -ExecutionPolicy Bypass -File \"$WINDOWS_SCRIPT_SRC\" -Action stop" ;;
      *) echo "[FAIL] unsupported platform: $platform"; exit 1 ;;
    esac
    ;;
  restart)
    case "$platform" in
      linux) restart_linux ;;
      darwin) restart_macos ;;
      windows) echo "[INFO] Restart with PowerShell: powershell -ExecutionPolicy Bypass -File \"$WINDOWS_SCRIPT_SRC\" -Action start" ;;
      *) echo "[FAIL] unsupported platform: $platform"; exit 1 ;;
    esac
    ;;
  status)
    case "$platform" in
      linux) status_linux ;;
      darwin) status_macos ;;
      windows) status_windows ;;
      *) echo "[FAIL] unsupported platform: $platform"; exit 1 ;;
    esac
    ;;
  *)
    usage
    exit 1
    ;;
esac
