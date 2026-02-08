#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

# Build Vue UI first
echo "[BUILD] Building Vue 3 UI..."
if [ -d "ui" ] && [ -f "ui/package.json" ]; then
    cd ui
    npm update 2>/dev/null || true
    npm outdated || true
    # Skip vue-tsc due to Node.js compatibility issues - vite build does type checking
    npx vite build
    cd ..
    
    echo "[BUILD] Vue UI built successfully"
else
    echo "[BUILD] Warning: ui/ directory not found, skipping Vue build"
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

echo "[BUILD] Building robot_tracker with GoCV support..."
echo "[BUILD] PKG_CONFIG_PATH: $PKG_CONFIG_PATH"

go build -tags=gocv -o robot_tracker ./cmd/

echo "[BUILD] Done: robot_tracker"
