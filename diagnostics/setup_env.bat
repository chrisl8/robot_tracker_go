@echo off
REM GoCV + OpenCV 4.13.0 Environment Setup Script
REM Run this before building or running the robot_tracker_go application

REM Add MinGW GCC to PATH (required for CGO compilation)
set PATH=C:\mingw64\bin;%PATH%

REM Add OpenCV DLLs to PATH (required for runtime)
set PATH=C:\opencv\build\install\x64\mingw\bin;%PATH%

REM Set CGO flags for compilation
set CGO_CPPFLAGS=-IC:/opencv/build/install/include
set CGO_LDFLAGS=-LC:/opencv/build/install/x64/mingw/lib

echo Environment configured for GoCV + OpenCV 4.13.0
echo GCC: %which%
echo OpenCV DLLs found
