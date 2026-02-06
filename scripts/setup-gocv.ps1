<#
.SYNOPSIS
    Configures environment for building GoCV applications on Windows.
#>

# Set OpenCV environment variables for GoCV
$env:OPENCV_DIR = "C:\opencv\build\install"

# Add OpenCV to PATH
$env:PATH = "$env:OPENCV_DIR\x64\mingw\bin;$env:PATH"

# CGO flags for gocv
$env:CGO_CPPFLAGS = "-I$env:OPENCV_DIR\include"
$env:CGO_LDFLAGS = "-L$env:OPENCV_DIR\x64\mingw\lib -lopencv_core4130 -lopencv_imgproc4130 -lopencv_videoio4130 -lopencv_highgui4130 -lopencv_imgcodecs4130 -lopencv_objdetect4130 -lopencv_dnn4130 -lopencv_features2d4130 -lopencv_calib3d4130"

Write-Host "GoCV environment configured:" -ForegroundColor Green
Write-Host "  OPENCV_DIR: $env:OPENCV_DIR" -ForegroundColor Cyan
Write-Host "  PATH includes: $env:OPENCV_DIR\x64\mingw\bin" -ForegroundColor Cyan
Write-Host ""
Write-Host "You can now run:" -ForegroundColor Yellow
Write-Host "  go test ./internal/camera/... -v" -ForegroundColor Yellow
Write-Host "  go build -o robot_tracker.exe ./cmd/main.go" -ForegroundColor Yellow
