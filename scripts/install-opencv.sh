#!/bin/bash
set -e

OPENCV_VERSION="${OPENCV_VERSION:-4.13.0}"
INSTALL_PREFIX="${OPENCV_PREFIX:-/usr/local}"
BUILD_DIR="${HOME}/opencv-build-${OPENCV_VERSION}"
PARALLEL_JOBS=$(nproc)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

uninstall_system_opencv() {
    log_warn "Uninstalling system OpenCV 4.6.0 to avoid library conflicts..."

    if dpkg -l | grep -q "^ii.*libopencv.*4.6.0"; then
        log_info "Removing OpenCV 4.6.0 packages..."
        sudo apt-get remove --purge -y \
            '.*opencv.*' \
            libopencv-dev \
            libopencv-*

        # Also remove any leftover config files
        sudo apt-get autoremove -y

        log_info "System OpenCV 4.6.0 removed"
    else
        log_info "No system OpenCV 4.6.0 found, skipping uninstall"
    fi
}

verify_build_tools() {
    local missing=()
    for cmd in cmake git make g++; do
        if ! command -v $cmd &> /dev/null; then
            missing+=($cmd)
        fi
    done

    if [ ${#missing[@]} -ne 0 ]; then
        log_error "Missing required build tools: ${missing[*]}"
        log_info "These should have been installed by install_dependencies()"
        exit 1
    fi
}

install_dependencies() {
    # First, remove system OpenCV to avoid conflicts
    uninstall_system_opencv

    log_info "Installing system dependencies..."

    if command -v cmake &> /dev/null && \
       command -v git &> /dev/null; then
        log_info "Build tools already installed"
    else
        log_info "Installing build tools..."
        sudo apt-get update
        sudo apt-get install -y \
            build-essential \
            cmake \
            git \
            pkg-config
    fi

    verify_build_tools

    log_info "Installing OpenCV dependencies..."

    local packages=(
        libgtk2.0-dev
        libavcodec-dev
        libavformat-dev
        libswscale-dev
        libtbb-dev
        libjpeg-dev
        libpng-dev
        libtiff-dev
        libdc1394-dev
        python3-dev
        python3-numpy
        libeigen3-dev
    )

    # libjasper-dev is not available in Ubuntu 24.04+
    # It's optional for OpenCV (used for JPEG-2000 support)
    if apt-cache show libjasper-dev &>/dev/null; then
        packages+=(libjasper-dev)
    else
        log_warn "libjasper-dev not available (optional, skipping)"
    fi

    # libvtk9-dev can fail on minimal systems, make it optional
    if apt-cache show libvtk9-dev &>/dev/null; then
        packages+=(libvtk9-dev)
    else
        log_warn "libvtk9-dev not available (optional, skipping)"
    fi

    sudo apt-get install -y "${packages[@]}"
}

download_sources() {
    log_info "Downloading OpenCV ${OPENCV_VERSION}..."

    cd /tmp

    if [ -d "opencv-${OPENCV_VERSION}" ]; then
        log_info "OpenCV source already downloaded"
    else
        log_info "Downloading OpenCV ${OPENCV_VERSION}..."
        wget -O opencv-${OPENCV_VERSION}.tar.gz \
            https://github.com/opencv/opencv/archive/${OPENCV_VERSION}.tar.gz
        tar xzf opencv-${OPENCV_VERSION}.tar.gz
        rm opencv-${OPENCV_VERSION}.tar.gz
    fi

    # Download opencv_contrib for extra modules (including aruco)
    if [ ! -d "opencv_contrib-${OPENCV_VERSION}" ]; then
        log_info "Downloading OpenCV contrib modules..."
        wget -O opencv_contrib-${OPENCV_VERSION}.tar.gz \
            https://github.com/opencv/opencv_contrib/archive/${OPENCV_VERSION}.tar.gz
        tar xzf opencv_contrib-${OPENCV_VERSION}.tar.gz
        rm opencv_contrib-${OPENCV_VERSION}.tar.gz
    else
        log_info "OpenCV contrib source already downloaded"
    fi

    if [ -d "${BUILD_DIR}" ]; then
        log_info "Build directory already exists, skipping configure"
        return 1  # Signal to skip configure step
    fi
    return 0  # Signal to continue with configure
}

configure_cmake() {
    log_info "Configuring OpenCV ${OPENCV_VERSION} with CMake..."

    mkdir -p ${BUILD_DIR}
    cd ${BUILD_DIR}

    cmake \
        -D CMAKE_BUILD_TYPE=Release \
        -D CMAKE_INSTALL_PREFIX=${INSTALL_PREFIX} \
        -D BUILD_SHARED_LIBS=ON \
        -D BUILD_TESTS=OFF \
        -D BUILD_PERF_TESTS=OFF \
        -D BUILD_EXAMPLES=OFF \
        -D BUILD_opencv_apps=OFF \
        -D WITH_OPENCL=ON \
        -D WITH_TBB=ON \
        -D WITH_V4L=ON \
        -D WITH_QT=OFF \
        -D WITH_GTK=ON \
        -D WITH_EIGEN=ON \
        -D WITH_FFMPEG=ON \
        -D OPENCV_EXTRA_MODULES_PATH=/tmp/opencv_contrib-${OPENCV_VERSION}/modules \
        /tmp/opencv-${OPENCV_VERSION}
}

build_opencv() {
    log_info "Building OpenCV ${OPENCV_VERSION} (using ${PARALLEL_JOBS} jobs)..."

    cd ${BUILD_DIR}
    make -j${PARALLEL_JOBS}
}

install_opencv() {
    log_info "Installing OpenCV ${OPENCV_VERSION} to ${INSTALL_PREFIX}..."

    cd ${BUILD_DIR}
    sudo make install
    sudo ldconfig

    log_info "OpenCV ${OPENCV_VERSION} installed successfully"
}

verify_installation() {
    log_info "Verifying OpenCV installation..."

    # Check for OpenCV libraries directly
    lib_path="${INSTALL_PREFIX}/lib"

    if [ -f "${lib_path}/libopencv_core.so.${OPENCV_VERSION}" ]; then
        log_info "Core library found: ${lib_path}/libopencv_core.so.${OPENCV_VERSION}"
    elif [ -f "${lib_path}/libopencv_core.so" ]; then
        log_info "Core library found: ${lib_path}/libopencv_core.so"
    else
        log_error "OpenCV core library not found in ${lib_path}"
        return 1
    fi

    # Count libraries
    lib_count=$(ls ${lib_path}/libopencv_*.so.* 2>/dev/null | wc -l)
    if [ "$lib_count" -gt 0 ]; then
        log_info "Found ${lib_count} OpenCV libraries in ${lib_path}"
    else
        log_error "No OpenCV libraries found in ${lib_path}"
        return 1
    fi

    # Try to get version from library if pkg-config is available
    if pkg-config --exists opencv4 2>/dev/null; then
        version=$(pkg-config --modversion opencv4)
        log_info "pkg-config reports OpenCV version: ${version}"
    else
        log_info "pkg-config not configured for OpenCV (OK - using CGO instead)"
    fi

    log_info "OpenCV ${OPENCV_VERSION} installation verified!"
}

print_environment_setup() {
    echo ""
    log_info "=========================================="
    log_info "OpenCV ${OPENCV_VERSION} installation complete!"
    log_info "=========================================="
    echo ""
    log_info "Environment variables for GoCV:"
    echo ""
    echo "  # Add to ~/.bashrc or ~/.profile:"
    echo "  export OPENCV_DIR=\"${INSTALL_PREFIX}\""
    echo "  export LD_LIBRARY_PATH=\"${INSTALL_PREFIX}/lib:\$LD_LIBRARY_PATH\""
    echo ""
    echo "  # Then reload:"
    echo "  source ~/.bashrc"
    echo ""
    log_info "Build command:"
    echo "  go build -tags=gocv -o robot_tracker ./cmd/main.go"
    echo ""
}

cleanup() {
    if [ -d "${BUILD_DIR}" ]; then
        log_info "Cleaning up build directory..."
        rm -rf ${BUILD_DIR}
        log_info "Build directory removed"
    else
        log_info "Build directory does not exist, nothing to clean"
    fi
}

main() {
    log_info "OpenCV ${OPENCV_VERSION} Build Script"
    log_info "==================================="

    install_dependencies

    if download_sources; then
        configure_cmake
    else
        log_info "Using existing build directory, skipping configure"
    fi

    build_opencv
    install_opencv
    verify_installation
    print_environment_setup

    log_info "Build complete!"
}

case "${1:-}" in
    --cleanup)
        cleanup
        ;;
    --uninstall)
        uninstall_system_opencv
        ;;
    --verify)
        verify_installation
        ;;
    --help|-h)
        echo "Usage: $0 [--cleanup|--uninstall|--verify|--help]"
        echo ""
        echo "Options:"
        echo "  --cleanup   Remove build artifacts"
        echo "  --uninstall Remove system OpenCV 4.6.0 packages"
        echo "  --verify    Verify OpenCV installation"
        echo "  --help      Show this help message"
        echo ""
        echo "Examples:"
        echo "  $0                  # Install OpenCV 4.13.0"
        echo "  $0 --uninstall      # Remove system OpenCV 4.6.0"
        echo "  $0 --verify         # Check OpenCV installation"
        ;;
    *)
        main
        ;;
esac
