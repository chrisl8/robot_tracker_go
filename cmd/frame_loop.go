//go:build gocv

package main

import (
	"image"
	"image/draw"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/camera"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// idleFramePoll is how long the camera loop waits before asking again when the
// camera has not produced a new frame yet.
const idleFramePoll = 5 * time.Millisecond

// isRepeatFrame reports whether f is the frame the loop already processed.
// GetFrame hands back the latest frame without waiting, so it can return the
// same one again. Processing it again would advance the tracker and the
// steering logic on an unchanged picture, and, worse, would make a camera that
// has hung look alive to the stall watchdog (a hung read produces no error).
// Frames without a sequence number (Seq 0) are never treated as repeats.
func isRepeatFrame(lastSeq uint64, f *camera.Frame) bool {
	return f != nil && f.Seq != 0 && f.Seq == lastSeq
}

// resolveDemoMode decides whether to run demo mode: it is used when requested,
// or as the fallback when no camera can be opened. If the camera is not
// available yet (e.g. the macOS permission dialog is pending) it retries with
// backoff before giving up.
func (rs *RobotSystem) resolveDemoMode(demo bool) bool {
	if rs.capture.cam == nil && !demo && rs.capture.cameraConfig != nil {
		utils.Log("Camera not available at startup, retrying (waiting for permission?)...")
		if err := rs.tryOpenCamera(2 * time.Minute); err != nil {
			utils.Logf("Camera unavailable after retries: %v, falling back to demo mode", err)
			demo = true
		}
	}
	// If camera still isn't available and not in demo mode, fall back
	if rs.capture.cam == nil && !demo {
		utils.Log("No camera available, falling back to demo mode")
		demo = true
	}
	return demo
}

// runCameraLoop starts the real camera and processes frames until the system is
// stopped. It returns false if the camera failed to start, so the caller can fall
// back to demo mode; otherwise it returns true once the loop has ended.
func (rs *RobotSystem) runCameraLoop() bool {
	utils.Log("Starting real camera capture...")
	if err := rs.StartCamera(); err != nil {
		utils.Logf("Failed to start camera: %v, falling back to demo mode", err)
		return false
	}

	frameFailures := 0
	var lastSeq uint64
	minFrameInterval := time.Second / time.Duration(rs.cfg.EffectiveMaxFPS())
	utils.Logf("Processing capped at %d fps", rs.cfg.EffectiveMaxFPS())
	for rs.capture.cameraRunning.Load() {
		startTime := time.Now()
		frame, err := rs.capture.cam.GetFrame()
		if err != nil {
			frameFailures++
			rs.stats.perf.RecordCameraFailure()
			if frameFailures == 1 || frameFailures%50 == 0 {
				utils.Logf("Failed to get frame (%d in a row): %v", frameFailures, err)
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if frameFailures > 0 {
			utils.Logf("Camera frames resumed after %d failed reads", frameFailures)
			frameFailures = 0
		}
		if frame == nil || len(frame.Data) == 0 {
			utils.Logf("Empty frame received")
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if isRepeatFrame(lastSeq, frame) {
			time.Sleep(idleFramePoll)
			continue
		}
		lastSeq = frame.Seq
		img := cameraFrameToImage(frame)
		if img == nil {
			utils.Logf("Failed to convert frame to image")
			time.Sleep(100 * time.Millisecond)
			continue
		}
		rs.ProcessFrame(img, frame.Data)
		if elapsed := time.Since(startTime); elapsed < minFrameInterval {
			time.Sleep(minFrameInterval - elapsed)
		}
	}
	return true
}

// enterDemoMode switches the system to demo mode. It also covers falling back to
// demo because the camera never opened: calibrating on synthetic frames must not
// overwrite the real calibration file.
func (rs *RobotSystem) enterDemoMode() {
	rs.demoMode = true
	rs.registerDemoRobots()
	if rs.web.webServer != nil {
		rs.web.webServer.SetCameraName(demoCameraName)
	}
}

// runDemoLoop feeds synthetic frames (a test pattern with moving AprilTags)
// through the demo pipeline. It never returns.
func (rs *RobotSystem) runDemoLoop() {
	utils.Log("Demo mode: Generating test pattern with AprilTag visualization...")
	_ = rs.StartCamera()
	frameNum := 0
	for {
		frame := generateTestPattern(640, 480, frameNum)
		demoTags := generateDemoTags(640, 480, frameNum)
		rgbaImg, ok := frame.(*image.RGBA)
		if !ok {
			rgbaImg = image.NewRGBA(frame.Bounds())
			draw.Draw(rgbaImg, frame.Bounds(), frame, frame.Bounds().Min, draw.Src)
		}
		rgbaWithTags := drawDemoTagsOnImage(rgbaImg, demoTags)
		rs.ProcessDemoFrame(rgbaWithTags, frameNum, demoTags)
		frameNum++
		time.Sleep(33 * time.Millisecond)
	}
}
