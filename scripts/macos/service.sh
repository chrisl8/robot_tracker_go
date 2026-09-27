#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

SERVICE_LABEL="com.chrisl8.robot-tracker"
PLIST_PATH="$HOME/Library/LaunchAgents/${SERVICE_LABEL}.plist"
LOG_FILE="$HOME/Library/Logs/robot-tracker.log"

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

info()    { echo -e "${CYAN}ℹ${RESET} $1"; }
success() { echo -e "${GREEN}✓${RESET} $1"; }
warn()    { echo -e "${YELLOW}⚠${RESET} $1"; }
error()   { echo -e "${RED}✗${RESET} $1"; }

usage() {
    echo ""
    echo -e "${BOLD}Robot Tracker — macOS LaunchAgent Service Manager${RESET}"
    echo ""
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  install     Generate LaunchAgent plist and load it"
    echo "  uninstall   Stop, unload, and remove the LaunchAgent"
    echo "  start       Start the robot tracker service"
    echo "  stop        Stop the robot tracker service"
    echo "  restart     Stop and start the service"
    echo "  status      Show whether the service is running"
    echo "  log         Tail the service log file"
    echo ""
    echo "The LaunchAgent runs in your GUI login session, so it has"
    echo "camera access — unlike processes started over SSH directly."
    echo ""
}

generate_plist() {
    # Resolve Homebrew and OpenCV paths
    local homebrew_prefix
    homebrew_prefix="$(brew --prefix 2>/dev/null || echo /opt/homebrew)"
    local opencv_prefix
    opencv_prefix="$(brew --prefix opencv 2>/dev/null || echo "${homebrew_prefix}/opt/opencv")"

    cat <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${SERVICE_LABEL}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${PROJECT_DIR}/scripts/run.sh</string>
        <string>--log-file</string>
        <string>${LOG_FILE}</string>
    </array>

    <key>WorkingDirectory</key>
    <string>${PROJECT_DIR}</string>

    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>${HOME}/go/bin:${homebrew_prefix}/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
        <key>HOME</key>
        <string>${HOME}</string>
        <key>OPENCV_DIR</key>
        <string>${opencv_prefix}</string>
        <key>CGO_CPPFLAGS</key>
        <string>-I${opencv_prefix}/include/opencv4</string>
        <key>PKG_CONFIG_PATH</key>
        <string>${homebrew_prefix}/lib/pkgconfig</string>
        <key>DYLD_LIBRARY_PATH</key>
        <string>${opencv_prefix}/lib</string>
    </dict>

    <key>RunAtLoad</key>
    <false/>

    <key>KeepAlive</key>
    <false/>

    <key>StandardOutPath</key>
    <string>${HOME}/Library/Logs/robot-tracker-stdout.log</string>

    <key>StandardErrorPath</key>
    <string>${HOME}/Library/Logs/robot-tracker-stderr.log</string>
</dict>
</plist>
PLIST
}

cmd_install() {
    info "Generating LaunchAgent plist..."
    info "Project directory: ${PROJECT_DIR}"

    # Ensure LaunchAgents directory exists
    mkdir -p "$HOME/Library/LaunchAgents"

    # Unload existing if present
    if launchctl list "$SERVICE_LABEL" &>/dev/null; then
        info "Unloading existing LaunchAgent..."
        launchctl unload "$PLIST_PATH" 2>/dev/null || true
    fi

    # Generate and write plist
    generate_plist > "$PLIST_PATH"
    success "Plist written to: ${PLIST_PATH}"

    # Load the agent (makes it available for start/stop, doesn't run it)
    launchctl load "$PLIST_PATH"
    success "LaunchAgent loaded"

    echo ""
    info "Ready. Use './scripts/service.sh start' to run the tracker."
    info "Log file: ${LOG_FILE}"
}

cmd_uninstall() {
    if [ ! -f "$PLIST_PATH" ]; then
        warn "LaunchAgent plist not found at: ${PLIST_PATH}"
        warn "Nothing to uninstall."
        return
    fi

    # Stop if running
    if launchctl list "$SERVICE_LABEL" &>/dev/null; then
        info "Stopping and unloading LaunchAgent..."
        launchctl stop "$SERVICE_LABEL" 2>/dev/null || true
        launchctl unload "$PLIST_PATH" 2>/dev/null || true
    fi

    rm -f "$PLIST_PATH"
    success "LaunchAgent removed: ${PLIST_PATH}"
}

cmd_start() {
    if ! launchctl list "$SERVICE_LABEL" &>/dev/null; then
        error "LaunchAgent not loaded. Run './scripts/service.sh install' first."
        exit 1
    fi

    # Check if already running
    local pid
    pid=$(launchctl list "$SERVICE_LABEL" 2>/dev/null | awk -F'= ' '/"PID"/ {gsub(/[^0-9]/,"",$2); print $2}')
    if [ -n "$pid" ] && [ "$pid" != "0" ]; then
        warn "Service is already running (PID ${pid})"
        return
    fi

    info "Starting robot tracker..."
    launchctl start "$SERVICE_LABEL"

    # Wait for process to appear (up to 15 seconds — launchd throttles restarts)
    local attempts=0
    pid=""
    while [ $attempts -lt 30 ]; do
        sleep 0.5
        pid=$(launchctl list "$SERVICE_LABEL" 2>/dev/null | awk -F'= ' '/"PID"/ {gsub(/[^0-9]/,"",$2); print $2}')
        if [ -n "$pid" ] && [ "$pid" != "0" ]; then
            break
        fi
        attempts=$((attempts + 1))
    done

    if [ -z "$pid" ] || [ "$pid" = "0" ]; then
        error "Service failed to start. Check the log:"
        error "  $0 log"
        return 1
    fi

    # Wait for web server to become responsive (up to 15 seconds)
    info "Waiting for web server (PID ${pid})..."
    local ready=false
    for i in $(seq 1 30); do
        if curl -sf http://localhost:9086/api/status >/dev/null 2>&1; then
            ready=true
            break
        fi
        # Check if process is still alive
        local check_pid
        check_pid=$(launchctl list "$SERVICE_LABEL" 2>/dev/null | awk -F'= ' '/"PID"/ {gsub(/[^0-9]/,"",$2); print $2}')
        if [ -z "$check_pid" ] || [ "$check_pid" = "0" ]; then
            error "Service exited during startup. Check the log:"
            error "  $0 log"
            return 1
        fi
        sleep 0.5
    done

    if $ready; then
        success "Robot tracker started and ready (PID ${pid})"
        success "Web UI: http://localhost:9086"
    else
        warn "Service running (PID ${pid}) but web server not yet responding"
        warn "Camera permission dialog may be showing — check the Mac's screen"
        warn "Log: $0 log"
    fi
}

cmd_stop() {
    if ! launchctl list "$SERVICE_LABEL" &>/dev/null; then
        warn "LaunchAgent not loaded."
        return
    fi

    info "Stopping robot tracker..."
    launchctl stop "$SERVICE_LABEL" 2>/dev/null || true
    success "Stop signal sent"
}

cmd_restart() {
    cmd_stop

    # Wait for process to fully exit (launchd throttles restarts for 10s)
    info "Waiting for process to exit..."
    local attempts=0
    while [ $attempts -lt 24 ]; do
        local pid
        pid=$(launchctl list "$SERVICE_LABEL" 2>/dev/null | awk -F'= ' '/"PID"/ {gsub(/[^0-9]/,"",$2); print $2}')
        if [ -z "$pid" ] || [ "$pid" = "0" ]; then
            break
        fi
        sleep 0.5
        attempts=$((attempts + 1))
    done

    cmd_start
}

cmd_status() {
    if ! launchctl list "$SERVICE_LABEL" &>/dev/null; then
        echo -e "${YELLOW}●${RESET} ${BOLD}robot-tracker${RESET} — not loaded"
        echo "  Run './scripts/service.sh install' to set up the LaunchAgent."
        return
    fi

    local launchctl_info
    launchctl_info=$(launchctl list "$SERVICE_LABEL" 2>/dev/null)

    local pid
    pid=$(echo "$launchctl_info" | awk -F'= ' '/"PID"/ {gsub(/[^0-9]/,"",$2); print $2}')
    local exit_code
    exit_code=$(echo "$launchctl_info" | awk -F'= ' '/"LastExitStatus"/ {gsub(/[^0-9]/,"",$2); print $2}')

    if [ -n "$pid" ] && [ "$pid" != "0" ]; then
        echo -e "${GREEN}●${RESET} ${BOLD}robot-tracker${RESET} — running (PID ${pid})"
        # Check if web server is responsive
        if curl -sf --max-time 2 http://localhost:9086/api/status >/dev/null 2>&1; then
            echo "  Web UI:  http://localhost:9086 (responding)"
        else
            echo "  Web UI:  not responding (camera permission dialog may be showing)"
        fi
    else
        echo -e "${RED}●${RESET} ${BOLD}robot-tracker${RESET} — stopped"
        if [ -n "$exit_code" ] && [ "$exit_code" != "0" ]; then
            echo "  Last exit code: ${exit_code}"
        fi
    fi

    echo "  Label:   ${SERVICE_LABEL}"
    echo "  Plist:   ${PLIST_PATH}"
    echo "  Log:     ${LOG_FILE}"
    echo "  Project: ${PROJECT_DIR}"
}

cmd_log() {
    if [ ! -f "$LOG_FILE" ]; then
        warn "Log file not found: ${LOG_FILE}"
        warn "The service may not have run yet."
        return
    fi
    exec tail -f "$LOG_FILE"
}

# --- Main ---
case "${1:-}" in
    install)   cmd_install ;;
    uninstall) cmd_uninstall ;;
    start)     cmd_start ;;
    stop)      cmd_stop ;;
    restart)   cmd_restart ;;
    status)    cmd_status ;;
    log)       cmd_log ;;
    *)         usage ;;
esac
