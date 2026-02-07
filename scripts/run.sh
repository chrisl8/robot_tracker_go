#!/bin/bash
set -e

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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ ! -f "${SCRIPT_DIR}/../robot_tracker" ]; then
    echo "[RUN] Building robot_tracker first..."
    "${SCRIPT_DIR}/build.sh"
fi

echo "[RUN] Starting robot_tracker..."

# Run with any passed arguments
exec ./robot_tracker "$@"
