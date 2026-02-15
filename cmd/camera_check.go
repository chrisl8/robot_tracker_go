//go:build gocv

package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gocv.io/x/gocv"

	"robot_tracker_go/internal/utils"
)

// getMacOSCameraNames parses system_profiler output to get camera names in order.
func getMacOSCameraNames() []string {
	if runtime.GOOS != "darwin" {
		return nil
	}

	cmd := exec.Command("system_profiler", "SPCameraDataType")
	output, err := cmd.Output()
	if err != nil {
		utils.Logf("Warning: Could not run system_profiler: %v", err)
		return nil
	}

	var names []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Camera names are indented with 4 spaces and end with ":"
		// They appear before their "Model ID:" lines
		if strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, "Camera:") &&
			!strings.Contains(trimmed, "Model ID:") && !strings.Contains(trimmed, "Unique ID:") {
			name := strings.TrimSuffix(trimmed, ":")
			if name != "" {
				names = append(names, name)
			}
		}
	}

	return names
}

// listCameras enumerates available cameras and displays their properties.
func listCameras() {
	fmt.Println()
	fmt.Println("=== Camera Detection ===")

	// Get macOS camera names for cross-reference
	var macOSCameras []string
	if runtime.GOOS == "darwin" {
		fmt.Println("Cameras detected by macOS:")
		macOSCameras = getMacOSCameraNames()
		if len(macOSCameras) == 0 {
			fmt.Println("  (none)")
		}
		for _, name := range macOSCameras {
			fmt.Printf("  - %s\n", name)
		}
		fmt.Println()
	}

	fmt.Println("Testing camera IDs 0-9 with OpenCV/GoCV:")
	fmt.Println()

	foundCount := 0
	for i := 0; i <= 9; i++ {
		cam, err := gocv.VideoCaptureDevice(i)
		if err != nil || cam == nil {
			continue
		}

		if !cam.IsOpened() {
			_ = cam.Close()
			continue
		}

		// Get camera properties
		width := cam.Get(gocv.VideoCaptureFrameWidth)
		height := cam.Get(gocv.VideoCaptureFrameHeight)
		fps := cam.Get(gocv.VideoCaptureFPS)

		_ = cam.Close()

		if width == 0 || height == 0 {
			continue
		}

		fmt.Printf("  Camera %d: %.0fx%.0f @ %.0f fps", i, width, height, fps)

		// Try to correlate with macOS camera names by index
		if foundCount < len(macOSCameras) {
			fmt.Printf("  (likely: %s)", macOSCameras[foundCount])
		}
		fmt.Println()

		foundCount++
	}

	if foundCount == 0 {
		fmt.Println("  No cameras found via OpenCV")
		if len(macOSCameras) > 0 {
			fmt.Println()
			fmt.Printf("  macOS sees %d camera(s) but OpenCV cannot access them.\n", len(macOSCameras))
			fmt.Println("  This is almost certainly a camera permission issue.")
		}
		fmt.Println()
		fmt.Println("Troubleshooting:")
		fmt.Println("  1. Check that a camera is connected (run: system_profiler SPCameraDataType)")
		fmt.Println("  2. Grant camera permission to your terminal app:")
		fmt.Println("     System Settings > Privacy & Security > Camera > enable your terminal (Terminal, iTerm2, etc.)")
		fmt.Println("  3. Restart your terminal after granting permission")
		fmt.Println("  4. Close other apps that may be using the camera (Zoom, FaceTime, Photo Booth, etc.)")
	} else {
		fmt.Printf("\nFound %d camera(s)\n", foundCount)
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Test a specific camera:  ./scripts/run.sh --test-camera <id>")
		fmt.Println("  2. Update config/tracking_config.yaml with the correct camera_id")
	}
	fmt.Println()
}

// testCamera tests a specific camera by capturing frames.
func testCamera(cameraID int) {
	fmt.Printf("\n=== Testing Camera %d ===\n\n", cameraID)

	cam, err := gocv.VideoCaptureDevice(cameraID)
	if err != nil || cam == nil {
		fmt.Printf("Failed to open camera %d\n", cameraID)
		fmt.Println()
		fmt.Println("Possible causes:")
		fmt.Println("  - Camera ID does not exist (run --list-cameras to see available IDs)")
		fmt.Println("  - Camera is in use by another application")
		if runtime.GOOS == "darwin" {
			fmt.Println("  - Camera permission not granted to your terminal app")
			fmt.Println()
			fmt.Println("To fix on macOS:")
			fmt.Println("  1. Open System Settings > Privacy & Security > Camera")
			fmt.Println("  2. Enable your terminal app (Terminal, iTerm2, etc.)")
			fmt.Println("  3. Restart your terminal after granting permission")
		}
		return
	}
	defer func() { _ = cam.Close() }()

	if !cam.IsOpened() {
		fmt.Printf("Camera %d could not be opened\n", cameraID)
		return
	}

	width := cam.Get(gocv.VideoCaptureFrameWidth)
	height := cam.Get(gocv.VideoCaptureFrameHeight)
	fps := cam.Get(gocv.VideoCaptureFPS)

	fmt.Println("Camera opened successfully")
	fmt.Printf("  Resolution: %.0fx%.0f\n", width, height)
	fmt.Printf("  FPS: %.0f\n", fps)
	fmt.Println()

	const numTestFrames = 10
	fmt.Printf("Capturing %d test frames...\n", numTestFrames)

	mat := gocv.NewMat()
	defer func() { _ = mat.Close() }()

	var captureTimesMs []int64
	var successCount int

	for i := 0; i < numTestFrames; i++ {
		start := time.Now()

		if ok := cam.Read(&mat); !ok {
			fmt.Printf("  Frame %d: failed to read\n", i+1)
			continue
		}

		if mat.Empty() {
			fmt.Printf("  Frame %d: empty frame\n", i+1)
			continue
		}

		elapsed := time.Since(start)
		captureTimesMs = append(captureTimesMs, elapsed.Milliseconds())
		successCount++

		// Save the first non-empty frame as a test image
		if successCount == 1 {
			testImagePath := fmt.Sprintf("/tmp/camera_test_%d.jpg", cameraID)
			if gocv.IMWrite(testImagePath, mat) {
				fmt.Printf("  Test frame saved to %s\n", testImagePath)
			}
		}
	}

	fmt.Println()

	if successCount == 0 {
		fmt.Printf("Failed to capture any frames from camera %d\n", cameraID)
		fmt.Println()
		fmt.Println("Troubleshooting:")
		fmt.Println("  - Close other apps that may be using the camera (Zoom, FaceTime, Photo Booth, etc.)")
		if runtime.GOOS == "darwin" {
			fmt.Println("  - Grant camera permission: System Settings > Privacy & Security > Camera > enable your terminal")
			fmt.Println("  - Restart your terminal after granting permission")
		}
		return
	}

	fmt.Printf("Captured %d/%d frames successfully\n", successCount, numTestFrames)

	if len(captureTimesMs) > 0 {
		var totalMs int64
		minMs := captureTimesMs[0]
		maxMs := captureTimesMs[0]

		for _, t := range captureTimesMs {
			totalMs += t
			if t < minMs {
				minMs = t
			}
			if t > maxMs {
				maxMs = t
			}
		}

		avgMs := totalMs / int64(len(captureTimesMs))
		fmt.Printf("  Average capture time: %dms\n", avgMs)
		fmt.Printf("  Min: %dms, Max: %dms\n", minMs, maxMs)
	}

	fmt.Println()
	fmt.Printf("Camera %d is working correctly!\n", cameraID)
	fmt.Println("\nTo use this camera, update config/tracking_config.yaml:")
	fmt.Printf("  cameras:\n")
	fmt.Printf("    - camera_id: %d\n", cameraID)
	fmt.Printf("      name: \"usb_camera\"\n")
	fmt.Printf("      width: %.0f\n", width)
	fmt.Printf("      height: %.0f\n", height)
	fmt.Printf("      fps: %.0f\n", fps)
	fmt.Println()
}
