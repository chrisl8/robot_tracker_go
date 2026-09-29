#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.."

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[0;37m'
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

subsection_header() {
    local title="$1"
    local color="${2:-$BLUE}"
    echo ""
    echo -e "${color}─── ${WHITE}${BOLD}${title}${RESET} ${color}────────────────────────────────────────${RESET}"
}

test_passed()  { echo -e "${GREEN}✓${RESET} $1"; }
test_info()    { echo -e "${BLUE}ℹ${RESET} $1"; }
test_warning() { echo -e "${YELLOW}⚠${RESET} $1"; }
test_error()   { echo -e "${RED}✗${RESET} $1"; }

VERBOSE=false

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

if [ -d "ui" ] && [ -f "ui/package.json" ]; then
    cd "$SCRIPT_DIR/../../ui"

    subsection_header "npm outdated" "$YELLOW"
    npm outdated || true

    subsection_header "ESLint" "$YELLOW"
    npm run lint:check || exit 1

    subsection_header "Prettier Format Check" "$YELLOW"
    npm run format:check || exit 1

    subsection_header "TypeScript Type Check" "$YELLOW"
    npx vue-tsc --noEmit || exit 1

    subsection_header "Dead Code Check (knip)" "$YELLOW"
    npm run knip:check || exit 1

    subsection_header "Vue Unit Tests" "$YELLOW"
    npm run test:run

    subsection_header "Playwright Integration Tests" "$YELLOW"
    # Stop the LaunchAgent service if running so Playwright starts its own --demo instance
    # instead of reusing the live service on the same port.
    SERVICE_LABEL="com.chrisl8.robot-tracker"
    SERVICE_WAS_RUNNING=false
    if launchctl list "$SERVICE_LABEL" &>/dev/null; then
        SERVICE_PID=$(launchctl list "$SERVICE_LABEL" 2>/dev/null | awk -F'= ' '/"PID"/ {gsub(/[^0-9]/,"",$2); print $2}')
        if [ -n "$SERVICE_PID" ] && [ "$SERVICE_PID" != "0" ]; then
            test_warning "Stopping robot-tracker service (PID ${SERVICE_PID}) to avoid port conflict..."
            launchctl stop "$SERVICE_LABEL" 2>/dev/null || true
            SERVICE_WAS_RUNNING=true
            sleep 1
        fi
    fi
    # Ensure Playwright browsers are installed (idempotent — skips if already present)
    npx playwright install
    npm run test:integration
    # Restart service if it was running before
    if [ "$SERVICE_WAS_RUNNING" = true ]; then
        test_info "Restarting robot-tracker service..."
        launchctl start "$SERVICE_LABEL" 2>/dev/null || true
    fi

    cd "$SCRIPT_DIR/../.."
    section_header "Vue UI Tests Passed" "$GREEN"
else
    test_warning "ui/ directory not found, skipping Vue tests"
fi

section_header "Go Tests" "$CYAN"

# Resolve Homebrew and OpenCV paths
HOMEBREW_PREFIX="$(brew --prefix 2>/dev/null || echo /opt/homebrew)"
OPENCV_PREFIX="$(brew --prefix opencv 2>/dev/null || echo "${HOMEBREW_PREFIX}/opt/opencv")"

# Let pkg-config handle library flags; just set paths for runtime linking
export OPENCV_DIR="${OPENCV_PREFIX}"
export CGO_CPPFLAGS="-I${OPENCV_PREFIX}/include/opencv4"
export PKG_CONFIG_PATH="${HOMEBREW_PREFIX}/lib/pkgconfig:${PKG_CONFIG_PATH:-}"
export DYLD_LIBRARY_PATH="${OPENCV_PREFIX}/lib:${DYLD_LIBRARY_PATH:-}"

if [[ ":$PATH:" != *":$HOME/go/bin:"* ]]; then
    export PATH="$HOME/go/bin:$PATH"
fi

cd "$SCRIPT_DIR/../.."

subsection_header "Go Code Analysis Tools" "$YELLOW"

test_info "Running golangci-lint..."
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

section_header "All Tests Passed!" "$GREEN"
