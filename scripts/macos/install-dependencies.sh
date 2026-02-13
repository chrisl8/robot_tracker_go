#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.."

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Ensure Homebrew is available
if ! command -v brew &>/dev/null; then
    log_error "Homebrew is not installed."
    log_error "Install it from https://brew.sh and re-run this script."
    exit 1
fi

log_info "Installing system dependencies via Homebrew..."

BREW_PACKAGES=(opencv cmake pkg-config node)

for pkg in "${BREW_PACKAGES[@]}"; do
    if brew list "$pkg" &>/dev/null; then
        log_info "$pkg already installed"
    else
        log_info "Installing $pkg..."
        brew install "$pkg"
    fi
done

# Ensure Go is available (may be installed via Homebrew or official installer)
if ! command -v go &>/dev/null; then
    log_info "Installing Go via Homebrew..."
    brew install go
else
    log_info "Go already installed: $(go version)"
fi

# Add Go bin to PATH for tool installs
if [[ ":$PATH:" != *":$HOME/go/bin:"* ]]; then
    export PATH="$HOME/go/bin:$PATH"
fi

log_info "Installing Go analysis tools..."

if ! command -v golangci-lint &>/dev/null; then
    log_info "Installing golangci-lint..."
    curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$HOME/bin" v2.8.0
else
    log_info "golangci-lint already installed"
fi

GO_TOOLS=(
    "honnef.co/go/tools/cmd/staticcheck@latest"
    "github.com/kisielk/errcheck@latest"
    "github.com/fzipp/gocyclo/cmd/gocyclo@latest"
    "github.com/securego/gosec/v2/cmd/gosec@latest"
    "golang.org/x/vuln/cmd/govulncheck@latest"
    "golang.org/x/tools/cmd/deadcode@latest"
    "go.uber.org/nilaway/cmd/nilaway@latest"
)

TOOL_NAMES=(staticcheck errcheck gocyclo gosec govulncheck deadcode nilaway)

for i in "${!GO_TOOLS[@]}"; do
    tool="${TOOL_NAMES[$i]}"
    pkg="${GO_TOOLS[$i]}"
    if ! command -v "$tool" &>/dev/null; then
        log_info "Installing $tool..."
        go install "$pkg"
    else
        log_info "$tool already installed"
    fi
done

log_info ""
log_info "=========================================="
log_info "Dependencies installed successfully!"
log_info "=========================================="
log_info ""
log_info "OpenCV installed at: $(brew --prefix opencv)"
log_info ""
log_info "Serial port notes for macOS:"
log_info "  Arduino serial ports appear as /dev/cu.usbserial-* or /dev/tty.usbmodem-*"
log_info "  Update config/tracking_config.yaml serial_port accordingly."
log_info ""
log_info "Next steps:"
log_info "  ./scripts/build.sh           # Build Vue UI + Go backend"
log_info "  ./scripts/run.sh --demo      # Run in demo mode (no camera)"
log_info "  ./scripts/test.sh --verbose  # Run all tests"
