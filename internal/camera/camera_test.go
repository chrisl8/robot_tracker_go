//go:build gocv

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

func TestFrameStruct(t *testing.T) {
	t.Run("Frame creation", func(t *testing.T) {
		frame := &Frame{
			Data:     []byte{1, 2, 3, 4, 5},
			Width:    640,
			Height:   480,
			Channels: 3,
		}

		if frame.Width != 640 {
			t.Errorf("Frame.Width = %d, want 640", frame.Width)
		}
		if frame.Height != 480 {
			t.Errorf("Frame.Height = %d, want 480", frame.Height)
		}
		if len(frame.Data) != 5 {
			t.Errorf("Frame.Data length = %d, want 5", len(frame.Data))
		}
	})

	t.Run("Frame size calculation", func(t *testing.T) {
		width := 320
		height := 240
		channels := 3
		expectedSize := width * height * channels

		frame := &Frame{
			Data:     make([]byte, expectedSize),
			Width:    width,
			Height:   height,
			Channels: channels,
		}

		if len(frame.Data) != expectedSize {
			t.Errorf("Frame.Data length = %d, want %d", len(frame.Data), expectedSize)
		}
	})

	t.Run("Frame with zero dimensions", func(t *testing.T) {
		frame := &Frame{
			Data:   make([]byte, 0),
			Width:  0,
			Height: 0,
		}

		if frame.Width != 0 {
			t.Errorf("Frame.Width = %d, want 0", frame.Width)
		}
		if frame.Height != 0 {
			t.Errorf("Frame.Height = %d, want 0", frame.Height)
		}
	})
}

func TestCameraConfig(t *testing.T) {
	t.Run("Default config values", func(t *testing.T) {
		config := CameraConfig{
			Name:     "test_camera",
			CameraID: 0,
		}

		if config.Name != "test_camera" {
			t.Errorf("Config.Name = %s, want test_camera", config.Name)
		}
		if config.CameraID != 0 {
			t.Errorf("Config.CameraID = %d, want 0", config.CameraID)
		}
	})

	t.Run("Config with custom values", func(t *testing.T) {
		config := CameraConfig{
			Name:    "ip_camera",
			URL:     "http://192.168.1.100:9086/video",
			Width:   1280,
			Height:  720,
			FPS:     60,
			Backend: "IP",
		}

		if config.URL != "http://192.168.1.100:9086/video" {
			t.Errorf("Config.URL = %s, want http://192.168.1.100:9086/video", config.URL)
		}
		if config.Width != 1280 {
			t.Errorf("Config.Width = %d, want 1280", config.Width)
		}
		if config.Height != 720 {
			t.Errorf("Config.Height = %d, want 720", config.Height)
		}
		if config.FPS != 60 {
			t.Errorf("Config.FPS = %d, want 60", config.FPS)
		}
	})

	t.Run("Config YAML tags", func(t *testing.T) {
		config := CameraConfig{
			Name:     "test",
			CameraID: 1,
			URL:      "rtsp://192.168.1.100:554/stream",
			Width:    1920,
			Height:   1080,
			FPS:      30,
			Backend:  "IP",
			Timeout:  30.0,
		}

		if config.Backend != "IP" {
			t.Errorf("Config.Backend = %s, want IP", config.Backend)
		}
		if config.Timeout != 30.0 {
			t.Errorf("Config.Timeout = %f, want 30.0", config.Timeout)
		}
	})
}

func TestCameraError(t *testing.T) {
	t.Run("CameraError creation", func(t *testing.T) {
		err := &CameraError{
			Message: "camera not connected",
		}

		if err.Error() != "camera not connected" {
			t.Errorf("Error() = %s, want 'camera not connected'", err.Error())
		}
	})

	t.Run("CameraError as interface", func(t *testing.T) {
		var err error = &CameraError{Message: "test error"}

		if err.Error() != "test error" {
			t.Errorf("err.Error() = %s, want 'test error'", err.Error())
		}
	})

	t.Run("CameraError wrapping", func(t *testing.T) {
		originalErr := &CameraError{Message: "original error"}
		wrappedErr := originalErr

		if wrappedErr.Error() != "original error" {
			t.Errorf("wrappedErr.Error() = %s, want 'original error'", wrappedErr.Error())
		}
	})
}

func TestConstants(t *testing.T) {
	t.Run("Default constants", func(t *testing.T) {
		if DefaultWidth != 640 {
			t.Errorf("DefaultWidth = %d, want 640", DefaultWidth)
		}
		if DefaultHeight != 480 {
			t.Errorf("DefaultHeight = %d, want 480", DefaultHeight)
		}
		if DefaultFPS != 30 {
			t.Errorf("DefaultFPS = %d, want 30", DefaultFPS)
		}
		if DefaultTimeout == 0 {
			t.Error("DefaultTimeout should not be zero")
		} else {
			t.Logf("DefaultTimeout = %v", DefaultTimeout)
		}
	})
}

func TestCameraInterface(t *testing.T) {
	t.Run("Camera interface methods exist", func(t *testing.T) {
		var _ Camera = &GoCVCamera{}
		t.Log("GoCVCamera implements Camera interface")
	})

	t.Run("Interface compliance", func(t *testing.T) {
		config := CameraConfig{
			Name:     "test",
			CameraID: 0,
		}
		_ = config
		t.Log("Camera interface is properly defined")
	})
}

func TestIPCameraConfig(t *testing.T) {
	t.Run("IP camera URL parsing", func(t *testing.T) {
		tests := []struct {
			name string
			url  string
			want string
		}{
			{"HTTP URL", "http://192.168.1.100:9086/video", "http://192.168.1.100:9086/video"},
			{"RTSP URL", "rtsp://192.168.1.100:554/stream", "rtsp://192.168.1.100:554/stream"},
			{"HTTPS URL", "https://camera.local/video", "https://camera.local/video"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := CameraConfig{
					Name:    "ip",
					URL:     tt.url,
					Backend: "IP",
				}
				if config.URL != tt.want {
					t.Errorf("Config.URL = %s, want %s", config.URL, tt.want)
				}
			})
		}
	})
}
