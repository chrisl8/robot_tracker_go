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
test_passed() {
    echo -e "${GREEN}✓${RESET} $1"
}

test_info() {
    echo -e "${BLUE}ℹ${RESET} $1"
}

test_warning() {
    echo -e "${YELLOW}⚠${RESET} $1"
}

test_error() {
    echo -e "${RED}✗${RESET} $1"
}

VERBOSE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--verbose]"
            echo ""
            echo "Options:"
            echo "  --verbose, -v  Enable verbose test output"
            echo "  --help, -h     Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--verbose]"
            exit 1
            ;;
    esac
done

section_header "Vue UI Tests" "$CYAN"

# Run Vue tests if ui directory exists
if [ -d "ui" ] && [ -f "ui/package.json" ]; then
    cd "$SCRIPT_DIR/../ui"

    # This is the slowest part and generally accomplishes nothing on a typical run.
    # Return this to service if you are using test less often for only manually edited code.
    # subsection_header "npm update" "$YELLOW"
    # npm update 2>/dev/null || true

    subsection_header "npm outdated" "$YELLOW"
    npm outdated || true

    subsection_header "ESLint" "$YELLOW"
    npm run lint:check || exit 1

    subsection_header "Prettier Format Check" "$YELLOW"
    npm run format:check || exit 1

    subsection_header "TypeScript Type Check" "$YELLOW"
    npx vue-tsc --noEmit || exit 1

    subsection_header "Dead Code Check (knip)" "$YELLOW"
    # Configuration in knip.jsonc ignores:
    # - API types (src/types/api.ts, obstacle.ts) for OpenAPI documentation
    # - @vueuse/core dependency (used but not detected)
    npm run knip:check || exit 1

    subsection_header "Vue Unit Tests" "$YELLOW"
    npm run test:run

    subsection_header "Playwright Integration Tests" "$YELLOW"
    npm run test:integration


    cd "$SCRIPT_DIR/.."
    section_header "Vue UI Tests Passed" "$GREEN"
else
    test_warning "ui/ directory not found, skipping Vue tests"
fi

section_header "Go Tests" "$CYAN"

# Set up OpenCV environment for GoCV using CGO
export OPENCV_DIR="/usr/local"
export CGO_CPPFLAGS="-I/usr/local/include/opencv4"

# Get list of all OpenCV 4.13.0 libraries
OPENCV_LIBS=""
for lib in /usr/local/lib/libopencv_*.so.4.13.0; do
    [ -f "$lib" ] || continue
    libname=$(basename "$lib" .so.4.13.0)
    OPENCV_LIBS="$OPENCV_LIBS -l${libname#lib}"
done

# Create pkg-config file if not exists
PKG_CONFIG_DIR="${HOME}/.pkg-config"
mkdir -p "${PKG_CONFIG_DIR}"

if [ ! -f "${PKG_CONFIG_DIR}/opencv4.pc" ]; then
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
fi

export PKG_CONFIG_PATH="${PKG_CONFIG_DIR}:${PKG_CONFIG_PATH}"
export CGO_LDFLAGS="-L/usr/local/lib $OPENCV_LIBS -Wl,-rpath,/usr/local/lib"
export LD_LIBRARY_PATH="/usr/local/lib:$LD_LIBRARY_PATH"
if [[ ":$PATH:" != *":$HOME/go/bin:"* ]]; then
    export PATH="$HOME/go/bin:$PATH"
fi

# Run Go tests
cd "$SCRIPT_DIR/.."

subsection_header "Go Code Analysis Tools" "$YELLOW"

test_info "Running golang-lint..."
golangci-lint run ./... || exit 1

test_info "Running go vet..."
go vet -tags=gocv ./... || exit 1

test_info "Running go staticcheck..."
staticcheck -tags=gocv ./... || exit 1

test_info "Running go errcheck..."
errcheck -tags=gocv ./... || exit 1

test_info "Running go gocyclo..."
gocyclo -over 60 . || exit 1

test_info "Running go gosec..."
gosec -quiet -tags=gocv ./... || exit 1

test_info "Running govulncheck..."
govulncheck -tags=gocv ./... || exit 1

test_info "Running go deadcode..."
deadcode ./... 2>/dev/null || true

test_info "Running nilaway..."
nilaway -tags=gocv ./... || exit 1

subsection_header "Go Unit Tests" "$YELLOW"
if [ "$VERBOSE" = true ]; then
    test_info "Running Go tests (verbose with race detection)..."
    go test -race -tags=gocv ./... -coverprofile=coverage.out -v || exit 1
else
    test_info "Running Go tests (with race detection)..."
    go test -race -tags=gocv ./... -coverprofile=coverage.out || exit 1
fi

subsection_header "Go Fuzz Tests" "$YELLOW"
test_info "Running fuzz tests for serial protocol..."
go test -fuzz=FuzzDecode -fuzztime=10s ./internal/controller/ || true
go test -fuzz=FuzzEncodeCommand -fuzztime=10s ./internal/controller/ || true

section_header "All Tests Passed!" "$GREEN"
