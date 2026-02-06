# GoCV + OpenCV 4.13.0 Diagnostic Tools

## Purpose
This directory contains diagnostic tools to verify gocv + OpenCV compatibility.

## Results
**GoCV v0.43.0 is FULLY COMPATIBLE with OpenCV 4.13.0**

## Test Executables

### gocv_final.exe
Direct OpenCV CGO test - verifies OpenCV 4.13.0 works with CGO
```bash
./gocv_final.exe
# Output: OpenCV version information
```

### test_gocv_simple.exe
Full gocv package test - verifies gocv works with OpenCV 4.13.0
```bash
./test_gocv_simple.exe
# Output: GoCV and OpenCV version information
```

## Setup Scripts

### setup_env.bat (Windows)
Sets up the environment for building and running with GoCV.

### setup_env.sh (Bash)
Sets up the environment for building and running with GoCV.

## Usage

### Quick Start
```cmd
set PATH=C:\mingw64\bin;C:\opencv\build\install\x64\mingw\bin;%PATH%
go build -o robot_tracker.exe ./cmd/main.go
```

### Full Documentation
See `DIAGNOSTIC_RESULTS.md` for complete diagnostic report.
