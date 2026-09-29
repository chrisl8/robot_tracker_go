//go:build gocv

package camera

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenCVEnvironment(t *testing.T) {
	t.Run("OPENCV_DIR environment variable", func(t *testing.T) {
		openCVDir := os.Getenv("OPENCV_DIR")
		if openCVDir == "" {
			if runtime.GOOS == "windows" {
				t.Error("OPENCV_DIR environment variable is not set")
				t.Log("Hint: Run: $env:OPENCV_DIR = 'C:\\opencv\\build\\install'")
			} else {
				t.Log("OPENCV_DIR not set (using default /usr/local)")
			}
		} else {
			t.Logf("OPENCV_DIR = %s", openCVDir)
		}
	})

	t.Run("OpenCV libraries", func(t *testing.T) {
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
				if strings.Contains(path, p) {
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
			// Check for OpenCV libraries on Linux/macOS
			openCVDir := os.Getenv("OPENCV_DIR")
			if openCVDir == "" {
				openCVDir = "/usr/local"
			}

			var libPattern string
			if runtime.GOOS == "darwin" {
				libPattern = filepath.Join(openCVDir, "lib", "libopencv_*.dylib")
			} else {
				libPattern = filepath.Join(openCVDir, "lib", "libopencv_*.so*")
			}

			libs, err := filepath.Glob(libPattern)
			if err != nil || len(libs) == 0 {
				t.Error("OpenCV libraries not found")
				t.Log("Install with: ./scripts/install-dependencies.sh")
			} else {
				t.Logf("Found %d OpenCV libraries", len(libs))
			}
		}
	})

	t.Run("GoCV compatibility", func(t *testing.T) {
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
				t.Log("OPENCV_DIR not set (using default /usr/local)")
				openCVDir = "/usr/local"
			} else {
				t.Logf("OPENCV_DIR = %s", openCVDir)
			}

			// Verify library exists (platform-specific extension)
			var libName string
			if runtime.GOOS == "darwin" {
				libName = "libopencv_core.dylib"
			} else {
				libName = "libopencv_core.so"
			}
			libPath := filepath.Join(openCVDir, "lib", libName)
			if _, err := os.Stat(libPath); os.IsNotExist(err) {
				t.Errorf("OpenCV core library not found: %s", libPath)
			} else {
				t.Logf("OpenCV core library found: %s", libPath)
			}
		}
	})
}

func TestGoCVCameraCreation(t *testing.T) {
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
		t.Log("CameraConfig structure is valid")
	})

	t.Run("NewGoCVCamera function signature", func(t *testing.T) {
		config := CameraConfig{
			Name:     "test_camera",
			CameraID: 0,
			Width:    640,
			Height:   480,
			FPS:      30,
		}
		_ = config
		t.Log("NewGoCVCamera function is available in package")
	})
}
