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
    npm install 2>/dev/null || true
    npm outdated

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

# Run Go tests
if [ "$VERBOSE" = true ]; then
    echo "[TEST] Running Go tests (verbose)..."
    go test ./... -v
else
    echo "[TEST] Running Go tests..."
    go test ./...
fi

echo "[TEST] Done"
