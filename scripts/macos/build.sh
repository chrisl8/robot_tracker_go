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

build_info()    { echo -e "${BLUE}ℹ${RESET} $1"; }
build_success() { echo -e "${GREEN}✓${RESET} $1"; }
build_warning() { echo -e "${YELLOW}⚠${RESET} $1"; }
build_error()   { echo -e "${RED}✗${RESET} $1"; }

section_header "Build: Robot Tracker Go (macOS)" "$CYAN"

# Build Vue UI first
subsection_header "Vue 3 UI" "$YELLOW"
if [ -d "ui" ] && [ -f "ui/package.json" ]; then
    cd ui
    if [ ! -d "node_modules" ]; then
        build_info "Installing npm dependencies..."
        npm install
    fi
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

# Resolve Homebrew and OpenCV paths
HOMEBREW_PREFIX="$(brew --prefix 2>/dev/null || echo /opt/homebrew)"
OPENCV_PREFIX="$(brew --prefix opencv 2>/dev/null || echo "${HOMEBREW_PREFIX}/opt/opencv")"

if [ ! -d "${OPENCV_PREFIX}" ]; then
    build_error "OpenCV not found at ${OPENCV_PREFIX}"
    build_error "Install with: brew install opencv"
    exit 1
fi

# Set up OpenCV environment for GoCV using CGO
# Homebrew provides a proper opencv4.pc — let pkg-config handle library flags
# to avoid duplicate -l warnings from GoCV's own pkg-config invocation.
export OPENCV_DIR="${OPENCV_PREFIX}"
export CGO_CPPFLAGS="-I${OPENCV_PREFIX}/include/opencv4"
export PKG_CONFIG_PATH="${HOMEBREW_PREFIX}/lib/pkgconfig:${PKG_CONFIG_PATH:-}"
export DYLD_LIBRARY_PATH="${OPENCV_PREFIX}/lib:${DYLD_LIBRARY_PATH:-}"

subsection_header "Go Backend with GoCV" "$YELLOW"
build_info "Building robot_tracker with GoCV support..."
build_info "OpenCV prefix: ${OPENCV_PREFIX}"
build_info "PKG_CONFIG_PATH: $PKG_CONFIG_PATH"

go build -tags=gocv -o robot_tracker ./cmd/

# Sign the binary so macOS TCC recognizes it across rebuilds.
# Without signing, macOS re-prompts for camera permission after every build.
if security find-identity -v -p codesigning 2>/dev/null | grep -q '"'; then
    IDENTITY=$(security find-identity -v -p codesigning 2>/dev/null | head -1 | sed 's/.*"\(.*\)".*/\1/')
    if codesign -f -s "$IDENTITY" robot_tracker 2>/dev/null; then
        build_success "Binary signed with: ${IDENTITY}"
    else
        # Signing failed — most likely the keychain is locked (common over SSH)
        build_warning "Code signing failed (keychain is probably locked)"
        if [ -t 0 ]; then
            # Interactive terminal — offer to unlock
            build_info "Unlocking keychain to sign binary (enter your macOS login password)..."
            if security unlock-keychain "$HOME/Library/Keychains/login.keychain-db"; then
                if codesign -f -s "$IDENTITY" robot_tracker 2>/dev/null; then
                    build_success "Binary signed with: ${IDENTITY}"
                else
                    build_warning "Code signing still failed after keychain unlock"
                    build_warning "Camera permission will need re-approval after this rebuild"
                fi
            else
                build_warning "Keychain unlock failed"
                build_warning "Camera permission will need re-approval after this rebuild"
            fi
        else
            build_warning "Non-interactive session — cannot unlock keychain"
            build_warning "Camera permission will need re-approval after this rebuild"
        fi
    fi
else
    build_warning "No codesigning identity found — camera will re-prompt after each rebuild"
    build_warning "Install Xcode or create a signing certificate to fix this"
fi

section_header "Build Complete: robot_tracker" "$GREEN"
