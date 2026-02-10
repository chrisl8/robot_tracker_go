#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

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

echo "[TEST] Running Vue UI tests..."

# Run Vue tests if ui directory exists
if [ -d "ui" ] && [ -f "ui/package.json" ]; then
    cd "$SCRIPT_DIR/../ui"
    npm update 2>/dev/null || true
    npm outdated || true

    echo "[TEST] Running ESLint..."
    npm run lint:check || exit 1

    echo "[TEST] Running Prettier format check..."
    npm run format:check || exit 1

    echo "[TEST] Running TypeScript type check..."
    npx vue-tsc --noEmit || exit 1

    echo "[TEST] Running knip dead code check..."
    # Note: knip may report false positives for:
    # - Dynamic imports in tests (test dependencies)
    # - API types exported for documentation purposes
    # - TRACK_COLORS (used via getTrackColor but not detected)
    npm run knip:check 2>/dev/null || echo "[TEST] knip found issues (non-blocking)"

    # Run unit tests
    echo "[TEST] Running Vue unit tests..."
    npm run test:run

    # Run integration tests
    echo "[TEST] Running Playwright integration tests..."
    npm run test:integration


    cd "$SCRIPT_DIR/.."
    echo "[TEST] Vue UI tests passed"
else
    echo "[TEST] Warning: ui/ directory not found, skipping Vue tests"
fi

echo "[TEST] Running Go tests..."

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

echo "[TEST] Running Go code tests..."
echo "[TEST] Running golang-lint..."
golangci-lint run ./... || exit 1

echo ""
echo "[TEST] Running go vet..."
go vet -tags=gocv ./... || exit 1

echo ""
echo "[TEST] Running go staticcheck..."
staticcheck -tags=gocv ./... || exit 1

echo ""
echo "[TEST] Running go errcheck..."
errcheck -tags=gocv ./... || exit 1

echo ""
echo "[TEST] Running go gocyclo..."
gocyclo -over 25 . || exit 1

echo ""
echo "[TEST] Running go gosec..."
gosec -tags=gocv ./... || exit 1

echo ""
echo "[TEST] Running govulncheck..."
govulncheck -tags=gocv ./... || exit 1

echo ""
echo "[TEST] Running go deadcode..."
deadcode ./... 2>/dev/null || true

echo ""
echo "[TEST] Running nilaway..."
nilaway -tags=gocv ./... || exit 1

if [ "$VERBOSE" = true ]; then
    echo "[TEST] Running Go tests (verbose with race detection)..."
    go test -race -tags=gocv ./... -coverprofile=coverage.out -v || exit 1
else
    echo "[TEST] Running Go tests (with race detection)..."
    go test -race -tags=gocv ./... -coverprofile=coverage.out || exit 1
fi

echo "[TEST] Done"
