//go:build gocv

package main

import (
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
)

func TestConvertDemoObstacles_ToDetection(t *testing.T) {
	demoObs := []DemoObstacle{
		{
			Name:       "Person 1",
			X:          400,
			Y:          300,
			Width:      80,
			Height:     120,
			Confidence: 0.92,
			Moving:     true,
			VX:         2,
			VY:         1,
		},
		{
			Name:       "Cup",
			X:          800,
			Y:          200,
			Width:      40,
			Height:     40,
			Confidence: 0.88,
			Moving:     false,
		},
	}

	converted := convertDemoObstaclesToDetection(demoObs)

	if len(converted) != 2 {
		t.Fatalf("expected 2 detection obstacles, got %d", len(converted))
	}

	if converted[0].ID != "Person 1" {
		t.Errorf("expected first obstacle ID 'Person 1', got '%s'", converted[0].ID)
	}

	if converted[0].PixelTopLeft[0] != 360 {
		t.Errorf("expected PixelTopLeft[0] = 360 (400-40), got %d", converted[0].PixelTopLeft[0])
	}
	if converted[0].PixelTopLeft[1] != 240 {
		t.Errorf("expected PixelTopLeft[1] = 240 (300-60), got %d", converted[0].PixelTopLeft[1])
	}
	if converted[0].PixelBottomRight[0] != 440 {
		t.Errorf("expected PixelBottomRight[0] = 440 (400+40), got %d", converted[0].PixelBottomRight[0])
	}
	if converted[0].PixelBottomRight[1] != 360 {
		t.Errorf("expected PixelBottomRight[1] = 360 (300+60), got %d", converted[0].PixelBottomRight[1])
	}

	if converted[1].ID != "Cup" {
		t.Errorf("expected second obstacle ID 'Cup', got '%s'", converted[1].ID)
	}
}

func TestConvertDemoObstacles_Empty(t *testing.T) {
	converted := convertDemoObstaclesToDetection([]DemoObstacle{})
	if len(converted) != 0 {
		t.Errorf("expected 0 converted obstacles, got %d", len(converted))
	}
}

func TestConvertDemoObstacles_SingleMoving(t *testing.T) {
	demoObs := []DemoObstacle{
		{
			Name:   "Laptop",
			X:      600,
			Y:      400,
			Width:  70,
			Height: 50,
		},
	}

	converted := convertDemoObstaclesToDetection(demoObs)

	if len(converted) != 1 {
		t.Fatalf("expected 1 detection obstacle, got %d", len(converted))
	}

	if converted[0].PixelTopLeft[0] != 565 {
		t.Errorf("expected PixelTopLeft[0] = 565, got %d", converted[0].PixelTopLeft[0])
	}
}

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

func TestDetectionPipeline_SetObstacles_FromDemo(t *testing.T) {
	pipeline := detection.NewDetectionPipeline(detection.AprilTagConfig{Family: "tag36h11"})

	demoObs := []DemoObstacle{
		{
			Name:   "Chair",
			X:      200,
			Y:      500,
			Width:  100,
			Height: 100,
		},
	}

	converted := convertDemoObstaclesToDetection(demoObs)
	pipeline.SetObstacles(converted)

	stored := pipeline.GetObstacles()
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored obstacle, got %d", len(stored))
	}

	if stored[0].ID != "Chair" {
		t.Errorf("expected stored obstacle ID 'Chair', got '%s'", stored[0].ID)
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
