//go:build gocv

package main

import (
	"image"
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/camera"
	"github.com/chrisl8/robot_tracker_go/internal/controller"
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

// orderCamera records what state the command queue was in when Stop reached
// the camera.
type orderCamera struct{ onStop func() }

func (c *orderCamera) Start() error                     { return nil }
func (c *orderCamera) Stop()                            { c.onStop() }
func (c *orderCamera) GetFrame() (*camera.Frame, error) { return nil, nil }
func (c *orderCamera) GetName() string                  { return "order" }

// The robot must be told to stop before the slow parts of shutdown (waiting
// for the frame loop, the final background save, stopping the camera), not
// after them.
func TestStop_StopsTheRobotBeforeStoppingTheCamera(t *testing.T) {
	rs := &RobotSystem{}
	rs.io.commandQueue = controller.NewCommandQueue(controller.NewArduinoController("none", 0), 100, 0)
	rs.io.commandQueue.Start()
	queueRunningAtCameraStop := true
	rs.capture.cam = &orderCamera{onStop: func() {
		queueRunningAtCameraStop = rs.io.commandQueue.IsRunning()
	}}

	rs.Stop()

	if queueRunningAtCameraStop {
		t.Error("the command queue was still running when the camera was stopped")
	}
}
