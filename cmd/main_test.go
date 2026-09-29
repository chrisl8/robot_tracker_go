//go:build gocv

package main

import (
	"sync"
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
)

func TestConvertPlanningObstacles_ToDetection(t *testing.T) {
	planningObs := []planning.Obstacle{
		{
			Name:              "TestObstacle",
			WorldTopLeft:      [2]float64{0.5, 0.5},
			WorldBottomRight:  [2]float64{1.0, 1.0},
			PixelsTopLeft:     [2]int{100, 100},
			PixelsBottomRight: [2]int{200, 200},
		},
	}

	detectionObs := convertPlanningObstaclesToDetection(planningObs)

	if len(detectionObs) != 1 {
		t.Fatalf("expected 1 detection obstacle, got %d", len(detectionObs))
	}

	if detectionObs[0].ID != "TestObstacle" {
		t.Errorf("expected obstacle ID 'TestObstacle', got '%s'", detectionObs[0].ID)
	}

	if detectionObs[0].WorldTopLeft[0] != 0.5 {
		t.Errorf("expected WorldTopLeft[0] = 0.5, got %f", detectionObs[0].WorldTopLeft[0])
	}
}

func TestDetectionPipeline_SetObstacles_FromPlanning(t *testing.T) {
	pipeline := detection.NewDetectionPipeline(detection.AprilTagConfig{Family: "tag36h11"})

	planningObs := []planning.Obstacle{
		{
			Name:              "Wall",
			WorldTopLeft:      [2]float64{0.0, 0.0},
			WorldBottomRight:  [2]float64{2.0, 0.5},
			PixelsTopLeft:     [2]int{0, 0},
			PixelsBottomRight: [2]int{640, 120},
		},
	}

	converted := convertPlanningObstaclesToDetection(planningObs)
	pipeline.SetObstacles(converted)

	stored := pipeline.GetObstacles()
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored obstacle, got %d", len(stored))
	}

	if stored[0].Clearance != 0.05 {
		t.Errorf("expected clearance 0.05, got %f", stored[0].Clearance)
	}
}

// TestDemoModeNeverUsesTheRealCalibrationFile guards against demo runs (and
// Playwright, which starts one) overwriting the real camera's calibration.
func TestDemoModeNeverUsesTheRealCalibrationFile(t *testing.T) {
	demo := &RobotSystem{demoMode: true}
	if got := demo.cameraDisplayName(); got != demoCameraName {
		t.Fatalf("demo camera name = %q, want %q", got, demoCameraName)
	}
	real := &RobotSystem{}
	if real.cameraDisplayName() != "" {
		t.Errorf("no camera configured should give an empty name, got %q", real.cameraDisplayName())
	}
}

// TestRobotSystem_Stop_IsIdempotent guards against the double-Stop() panic
// described in docs/archived/code-review-2026-09-27.md #15: main() both defers a call
// to Stop() and calls it from the SIGINT/SIGTERM handler goroutine, and only
// the handler's os.Exit(0) happens to keep those from overlapping today. This
// calls Stop() twice sequentially and many times concurrently, on a
// RobotSystem with no subsystems wired up (as NewRobotSystem(nil) produces),
// so any regression to an unconditional close()/Shutdown() in Stop() panics
// the test instead of only showing up under real shutdown timing.
func TestRobotSystem_Stop_IsIdempotent(t *testing.T) {
	rs := NewRobotSystem(nil)
	rs.stats.watchdogStop = make(chan struct{})

	rs.Stop()
	rs.Stop() // must not panic (e.g. double close of watchdogStop)

	rs = NewRobotSystem(nil)
	rs.stats.watchdogStop = make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rs.Stop()
		}()
	}
	wg.Wait()
}
