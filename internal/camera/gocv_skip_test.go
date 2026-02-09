//go:build !gocv

package camera

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"robot_tracker_go/internal/utils"
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
				if utils.ContainsPath(path, p) {
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
			// Check for OpenCV libraries on Linux
			libs, err := filepath.Glob("/usr/local/lib/libopencv_*.so*")
			if err != nil || len(libs) == 0 {
				t.Error("OpenCV libraries not found in /usr/local/lib")
				t.Log("Install with: ./scripts/install-opencv.sh")
			} else {
				t.Logf("Found %d OpenCV libraries in /usr/local/lib", len(libs))
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
			// Check OPENCV_DIR is set for GoCV
			openCVDir := os.Getenv("OPENCV_DIR")
			if openCVDir == "" {
				t.Error("OPENCV_DIR not set")
				t.Log("Required: export OPENCV_DIR=/usr/local")
			} else {
				t.Logf("OPENCV_DIR = %s", openCVDir)
			}

			// Verify libraries exist
			libPath := filepath.Join(openCVDir, "lib", "libopencv_core.so")
			if _, err := os.Stat(libPath); os.IsNotExist(err) {
				t.Errorf("OpenCV core library not found: %s", libPath)
			} else {
				t.Logf("OpenCV core library found: %s", libPath)
			}
		}
	})
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
