//go:build !gocv

package camera

import (
	"os"
	"runtime"
	"testing"
)

func TestOpenCVEnvironment(t *testing.T) {
	t.Run("OPENCV_DIR environment variable", func(t *testing.T) {
		openCVDir := os.Getenv("OPENCV_DIR")
		if openCVDir == "" {
			if runtime.GOOS == "windows" {
				t.Error("OPENCV_DIR environment variable is not set")
				t.Log("Hint: Run: $env:OPENCV_DIR = 'C:\\opencv\\build\\install'")
				t.Log("Then re-run tests with: go test -tags=gocv ./internal/camera/...")
			} else {
				t.Log("OPENCV_DIR not set (Linux: typically not needed with system OpenCV)")
			}
		} else {
			t.Logf("OPENCV_DIR = %s", openCVDir)
		}
	})

	t.Run("OpenCV availability", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			path := os.Getenv("PATH")
			if path == "" {
				t.Fatal("PATH environment variable is empty")
			}

			found := false
			for _, p := range []string{
				"C:\\opencv\\build\\install\\x64\\mingw\\bin",
				"C:\\opencv\\build\\install\\x64\\vc17\\bin",
				"C:\\opencv\\build\\install\\x64\\vc16\\bin",
			} {
				if containsPath(path, p) {
					found = true
					t.Logf("Found OpenCV bin in PATH: %s", p)
					break
				}
			}

			if !found {
				t.Error("OpenCV bin directory not found in PATH")
				t.Log("Hint: Run: $env:PATH = 'C:\\opencv\\build\\install\\x64\\mingw\\bin;' + $env:PATH")
			}
		} else {
			openCVVersion := checkOpenCVVersion()
			if openCVVersion == "" {
				t.Log("OpenCV not found via pkg-config - camera tests will be skipped")
			} else {
				t.Logf("System OpenCV version: %s", openCVVersion)
			}
		}
	})

	t.Run("GoCV build compatibility", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			binPath := os.Getenv("OPENCV_DIR")
			if binPath == "" {
				binPath = "C:\\opencv\\build\\install\\x64\\mingw\\bin"
			}
			binPath += "\\libopencv_core4130.dll"

			if _, err := os.Stat(binPath); os.IsNotExist(err) {
				t.Errorf("OpenCV core DLL not found at: %s", binPath)
				t.Log("Make sure OpenCV 4.13.0 is built and installed")
			} else {
				t.Logf("OpenCV core DLL found: %s", binPath)
			}
		} else {
			t.Log("Linux: GoCV requires building against compatible OpenCV headers")
			t.Log("To build GoCV on Linux: go build -tags=gocv ./cmd/main.go")
		}
	})
}

func checkOpenCVVersion() string {
	version, err := os.ReadFile("/usr/share/opencv4/version")
	if err == nil {
		return string(version)
	}
	return ""
}

func containsPath(path, target string) bool {
	if len(path) < len(target) {
		return false
	}
	for i := 0; i <= len(path)-len(target); i++ {
		if path[i:i+len(target)] == target {
			return true
		}
	}
	return false
}

func TestGoCVCameraRequiresGoCV(t *testing.T) {
	t.Skip("GoCV not available - OpenCV headers or gocv version mismatch")

	t.Run("CameraConfig defaults", func(t *testing.T) {
		config := CameraConfig{
			Name:     "test",
			CameraID: 0,
		}

		if config.Name != "test" {
			t.Errorf("Config.Name = %s, want test", config.Name)
		}
		if config.CameraID != 0 {
			t.Errorf("Config.CameraID = %d, want 0", config.CameraID)
		}
	})

	t.Run("NewGoCVCamera function", func(t *testing.T) {
		t.Skip("GoCV not available")
	})
}

func TestGoCVIntegration(t *testing.T) {
	t.Skip("GoCV not available - run 'go test -tags=gocv ./internal/camera/...' after fixing gocv/OpenCV compatibility")

	t.Run("Full camera integration test", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Log("This test requires:")
			t.Log("  1. OpenCV 4.13.0 built and installed")
			t.Log("  2. gocv.io/x/gocv v0.43.0 compatible with OpenCV headers")
			t.Log("  3. Run: $env:PATH = 'C:\\opencv\\build\\install\\x64\\mingw\\bin;' + $env:PATH")
			t.Log("  4. Then run: go test -tags=gocv ./internal/camera/...")
		} else {
			t.Log("This test requires:")
			t.Log("  1. OpenCV development libraries installed (libopencv-dev)")
			t.Log("  2. GoCV compatible with system OpenCV version")
			t.Log("  3. Run: go build -tags=gocv ./cmd/main.go")
		}
	})

	t.Run("Camera creation and frame capture", func(t *testing.T) {
		t.Skip("GoCV not available")
	})

	t.Run("Video file playback", func(t *testing.T) {
		t.Skip("GoCV not available")
	})
}
