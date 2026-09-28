//go:build gocv

package main

import (
	"image"
	"testing"
)

// Once Stop() has begun, frames still arriving must be dropped: Stop closes the
// detector's OpenCV resources and must never race a frame in flight.
func TestProcessFrame_DroppedAfterStop(t *testing.T) {
	rs := &RobotSystem{}
	rs.Stop()

	rs.ProcessFrame(image.NewRGBA(image.Rect(0, 0, 4, 4)), nil)
	rs.ProcessDemoFrame(image.NewRGBA(image.Rect(0, 0, 4, 4)), 0, nil)

	if rs.stats.frameNum != 0 {
		t.Errorf("frameNum = %d, want 0: frames after Stop must be ignored", rs.stats.frameNum)
	}
}

func TestStop_MarksFrameLoopStopped(t *testing.T) {
	rs := &RobotSystem{}
	rs.capture.cameraRunning.Store(true)

	rs.Stop()

	if rs.capture.cameraRunning.Load() {
		t.Error("cameraRunning should be false after Stop")
	}
	rs.capture.frameMu.Lock()
	stopped := rs.capture.stopped
	rs.capture.frameMu.Unlock()
	if !stopped {
		t.Error("stopped should be set after Stop")
	}
}
