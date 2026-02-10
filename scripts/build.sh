#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[0;37m'
BOLD='\033[1m'
RESET='\033[0m'

# Section separator function
section_header() {
    local title="$1"
    local color="${2:-$CYAN}"
    echo ""
    echo -e "${color}══════════════════════════════════════════════════════════════════════${RESET}"
    echo -e "${color}  ${BOLD}${title}${RESET}"
    echo -e "${color}══════════════════════════════════════════════════════════════════════${RESET}"
    echo ""
}

# Subsection separator function
subsection_header() {
    local title="$1"
    local color="${2:-$BLUE}"
    echo ""
    echo -e "${color}─── ${WHITE}${BOLD}${title}${RESET} ${color}────────────────────────────────────────${RESET}"
}

# Status functions
build_info() {
    echo -e "${BLUE}ℹ${RESET} $1"
}

build_success() {
    echo -e "${GREEN}✓${RESET} $1"
}

build_warning() {
    echo -e "${YELLOW}⚠${RESET} $1"
}

build_error() {
    echo -e "${RED}✗${RESET} $1"
}

section_header "Build: Robot Tracker Go" "$CYAN"

# Build Vue UI first
subsection_header "Vue 3 UI" "$YELLOW"
if [ -d "ui" ] && [ -f "ui/package.json" ]; then
    cd ui
    # Only run npm update if node_modules is missing or outdated
    if [ ! -d "node_modules" ] || [ ! -d "node_modules/.package-lock.json" ] && [ ! -f "package-lock.json" ]; then
        build_info "Installing/updating npm dependencies..."
        npm update 2>/dev/null || true
    fi
    # Skip vue-tsc due to Node.js compatibility issues - vite build does type checking
    build_info "Building Vue UI with Vite..."
    npx vite build
    cd ..

    build_success "Vue UI built successfully"
else
    build_warning "ui/ directory not found, skipping Vue build"
fi

# Set up Go environment
if [[ ":$PATH:" != *":$HOME/go/bin:"* ]]; then
    export PATH="$HOME/go/bin:$PATH"
fi

# Set up OpenCV environment for GoCV using CGO
export OPENCV_DIR="/usr/local"
export CGO_CPPFLAGS="-I/usr/local/include/opencv4"

# Get list of all OpenCV 4.13.0 libraries from /usr/local/lib
OPENCV_LIBS=""
for lib in /usr/local/lib/libopencv_*.so.4.13.0; do
    [ -f "$lib" ] || continue
    libname=$(basename "$lib" .so.4.13.0)
    OPENCV_LIBS="$OPENCV_LIBS -l${libname#lib}"
done

# If no 4.13.0 libraries found, try any version
if [ -z "$OPENCV_LIBS" ]; then
    for lib in /usr/local/lib/libopencv_*.so.*; do
        [ -f "$lib" ] || continue
        libname=$(basename "$lib" | sed 's/libopencv_\(.*\)\.so\..*/\1/')
        OPENCV_LIBS="$OPENCV_LIBS -l${libname}"
    done
fi

# Create pkg-config file to satisfy GoCV's pkg-config check
PKG_CONFIG_DIR="${HOME}/.pkg-config"
mkdir -p "${PKG_CONFIG_DIR}"

cat > "${PKG_CONFIG_DIR}/opencv4.pc" << EOF
prefix=/usr/local
exec_prefix=\${prefix}
libdir=\${exec_prefix}/lib
includedir=\${prefix}/include/opencv4

Name: OpenCV
Description: Open Source Computer Vision Library
Version: 4.13.0
Libs: -L\${libdir} $OPENCV_LIBS
Cflags: -I\${includedir}
Libs.private: -ldl -lm -lpthread -lrt
EOF

export PKG_CONFIG_PATH="${PKG_CONFIG_DIR}:${PKG_CONFIG_PATH}"
export CGO_LDFLAGS="-L/usr/local/lib $OPENCV_LIBS -Wl,-rpath,/usr/local/lib"

# Prepend our OpenCV libraries to LD_LIBRARY_PATH (runtime)
export LD_LIBRARY_PATH="/usr/local/lib:$LD_LIBRARY_PATH"

subsection_header "Go Backend with GoCV" "$YELLOW"
build_info "Building robot_tracker with GoCV support..."
build_info "PKG_CONFIG_PATH: $PKG_CONFIG_PATH"

go build -tags=gocv -o robot_tracker ./cmd/

section_header "Build Complete: robot_tracker" "$GREEN"
