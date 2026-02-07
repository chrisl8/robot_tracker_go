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

check_prerequisites() {
    log_info "Checking prerequisites..."
    local missing=()

    for cmd in cmake git make g++; do
        if ! command -v $cmd &> /dev/null; then
            missing+=($cmd)
        fi
    done

    if [ ${#missing[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing[*]}"
        log_info "Install with: sudo apt-get install build-essential cmake git"
        exit 1
    fi
}

install_dependencies() {
    log_info "Installing system dependencies..."

    if dpkg -l | grep -q "^ii.*libjasper-dev"; then
        log_info "Dependencies already installed"
        return 0
    fi

    sudo apt-get update
    sudo apt-get install -y \
        build-essential \
        cmake \
        git \
        pkg-config \
        libgtk2.0-dev \
        libavcodec-dev \
        libavformat-dev \
        libswscale-dev \
        libtbb-dev \
        libjpeg-dev \
        libpng-dev \
        libtiff-dev \
        libjasper-dev \
        libdc1394-dev \
        python3-dev \
        python3-numpy \
        libvtk9-dev \
        libeigen3-dev
}

download_sources() {
    log_info "Downloading OpenCV ${OPENCV_VERSION}..."

    cd /tmp

    if [ -d "opencv-${OPENCV_VERSION}" ]; then
        log_info "OpenCV source already downloaded"
        return 0
    fi

    wget -O opencv-${OPENCV_VERSION}.tar.gz \
        https://github.com/opencv/opencv/archive/${OPENCV_VERSION}.tar.gz

    tar xzf opencv-${OPENCV_VERSION}.tar.gz
    rm opencv-${OPENCV_VERSION}.tar.gz
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

    if pkg-config --exists opencv4; then
        version=$(pkg-config --modversion opencv4)
        log_info "OpenCV version: ${version}"

        if [ "$version" = "${OPENCV_VERSION}" ]; then
            log_info "Version matches expected: ${OPENCV_VERSION}"
        else
            log_warn "Version mismatch: got ${version}, expected ${OPENCV_VERSION}"
        fi
    else
        log_error "opencv4.pc not found in pkg-config"
        return 1
    fi

    lib_path="${INSTALL_PREFIX}/lib"
    if [ -f "${lib_path}/libopencv_core.so" ]; then
        log_info "Core library found: ${lib_path}/libopencv_core.so"
    else
        log_error "Core library not found at ${lib_path}"
        return 1
    fi
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
    log_info "Cleaning up build directory..."
    rm -rf ${BUILD_DIR}
}

main() {
    log_info "OpenCV ${OPENCV_VERSION} Build Script"
    log_info "==================================="

    check_prerequisites
    install_dependencies
    download_sources
    configure_cmake
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
    --verify)
        verify_installation
        ;;
    --help|-h)
        echo "Usage: $0 [--cleanup|--verify|--help]"
        echo ""
        echo "Options:"
        echo "  --cleanup  Remove build artifacts"
        echo "  --verify   Verify OpenCV installation"
        echo "  --help     Show this help message"
        ;;
    *)
        main
        ;;
esac
