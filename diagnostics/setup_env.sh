#!/bin/bash
# GoCV + OpenCV 4.13.0 Environment Setup Script
# Run this before building or running the robot_tracker_go application

# Add MinGW GCC to PATH (required for CGO compilation)
export PATH="/c/mingw64/bin:$PATH"

# Add OpenCV DLLs to PATH (required for runtime)
export PATH="/c/opencv/build/install/x64/mingw/bin:$PATH"

# Set CGO flags for compilation
export CGO_CPPFLAGS="-IC:/opencv/build/install/include"
export CGO_LDFLAGS="-LC:/opencv/build/install/x64/mingw/lib"

echo "Environment configured for GoCV + OpenCV 4.13.0"
echo "GCC: $(which gcc 2>/dev/null || echo 'NOT FOUND')"
echo "OpenCV DLLs: $(ls /c/opencv/build/install/x64/mingw/bin/libopencv_core4130.dll 2>/dev/null | head -1)"
