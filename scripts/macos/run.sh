#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.."

# Color definitions
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

section_header() {
    local title="$1"
    local color="${2:-$CYAN}"
    echo ""
    echo -e "${color}══════════════════════════════════════════════════════════════════════${RESET}"
    echo -e "${color}  ${BOLD}${title}${RESET}"
    echo -e "${color}══════════════════════════════════════════════════════════════════════${RESET}"
    echo ""
}

if [[ ":$PATH:" != *":$HOME/go/bin:"* ]]; then
    export PATH="$HOME/go/bin:$PATH"
fi

# Resolve Homebrew and OpenCV paths
HOMEBREW_PREFIX="$(brew --prefix 2>/dev/null || echo /opt/homebrew)"
OPENCV_PREFIX="$(brew --prefix opencv 2>/dev/null || echo "${HOMEBREW_PREFIX}/opt/opencv")"

# Let pkg-config handle library flags; just set paths for runtime linking
export OPENCV_DIR="${OPENCV_PREFIX}"
export CGO_CPPFLAGS="-I${OPENCV_PREFIX}/include/opencv4"
export PKG_CONFIG_PATH="${HOMEBREW_PREFIX}/lib/pkgconfig:${PKG_CONFIG_PATH:-}"
export DYLD_LIBRARY_PATH="${OPENCV_PREFIX}/lib:${DYLD_LIBRARY_PATH:-}"

if [ ! -f "./robot_tracker" ]; then
    echo "[RUN] Building robot_tracker first..."
    "${SCRIPT_DIR}/build.sh"
fi

section_header "Run: Robot Tracker Go (macOS)" "$CYAN"

exec ./robot_tracker "$@"
