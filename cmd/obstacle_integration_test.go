//go:build !gocv

package main

import (
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/position"
)

func TestPlanningToDetectionConversion(t *testing.T) {
	obs := []planning.Obstacle{
		{
			Name:              "TestWall",
			WorldTopLeft:      [2]float64{0.0, 0.0},
			WorldBottomRight:  [2]float64{1.0, 0.5},
			PixelsTopLeft:     [2]int{100, 100},
			PixelsBottomRight: [2]int{300, 200},
		},
	}

	conv := convertPlanningObstaclesToDetection(obs)

	if len(conv) != 1 {
		t.Fatalf("expected 1, got %d", len(conv))
	}
	if conv[0].ID != "TestWall" {
		t.Errorf("got %s", conv[0].ID)
	}
	if conv[0].PixelTopLeft != [2]int{100, 100} {
		t.Errorf("got %v", conv[0].PixelTopLeft)
	}
	if conv[0].Clearance != 0.05 {
		t.Errorf("clearance = %f", conv[0].Clearance)
	}
}

func TestPipelineStoresObstacles(t *testing.T) {
	pipeline := detection.NewDetectionPipeline(detection.AprilTagConfig{Family: "tag36h11"})

	stored := pipeline.GetObstacles()
	if len(stored) != 0 {
		t.Fatalf("expected 0 initially, got %d", len(stored))
	}

	obs := []detection.Obstacle{
		{ID: "TestObs", PixelTopLeft: [2]int{50, 50}, PixelBottomRight: [2]int{100, 100}},
	}
	pipeline.SetObstacles(obs)

	stored = pipeline.GetObstacles()
	if len(stored) != 1 {
		t.Fatalf("expected 1, got %d", len(stored))
	}
	if stored[0].ID != "TestObs" {
		t.Errorf("got %s", stored[0].ID)
	}
}

func TestMultipleObstacles(t *testing.T) {
	obs := []planning.Obstacle{
		{Name: "Obs1", PixelsTopLeft: [2]int{10, 10}, PixelsBottomRight: [2]int{50, 50}},
		{Name: "Obs2", PixelsTopLeft: [2]int{100, 100}, PixelsBottomRight: [2]int{150, 150}},
		{Name: "Obs3", PixelsTopLeft: [2]int{200, 200}, PixelsBottomRight: [2]int{250, 250}},
	}

	conv := convertPlanningObstaclesToDetection(obs)

	if len(conv) != 3 {
		t.Fatalf("expected 3, got %d", len(conv))
	}
	for i, name := range []string{"Obs1", "Obs2", "Obs3"} {
		if conv[i].ID != name {
			t.Errorf("[%d] got %s", i, conv[i].ID)
		}
	}
}

func TestClearObstacles(t *testing.T) {
	pipeline := detection.NewDetectionPipeline(detection.AprilTagConfig{Family: "tag36h11"})

	pipeline.SetObstacles([]detection.Obstacle{
		{ID: "Test", PixelTopLeft: [2]int{100, 100}, PixelBottomRight: [2]int{200, 200}},
	})

	stored := pipeline.GetObstacles()
	if len(stored) != 1 {
		t.Fatalf("expected 1, got %d", len(stored))
	}

	pipeline.SetObstacles([]detection.Obstacle{})

	stored = pipeline.GetObstacles()
	if len(stored) != 0 {
		t.Fatalf("expected 0, got %d", len(stored))
	}
}

func TestDrawResultsWithObstacles(t *testing.T) {
	pipeline := detection.NewDetectionPipeline(detection.AprilTagConfig{Family: "tag36h11"})

	pipeline.SetObstacles([]detection.Obstacle{
		{ID: "VisualTest", PixelTopLeft: [2]int{100, 100}, PixelBottomRight: [2]int{200, 200}},
	})

	result := &detection.DetectionResult{
		Tags:            []detection.AprilTag{},
		FusedDetections: []detection.FusedDetection{}, Timestamp: 0, FrameIdx: 0,
	}

	imgData := make([]byte, 640*480*3)
	output := pipeline.DrawResults(imgData, 640, 480, result)

	if output == nil {
		t.Error("DrawResults returned nil")
	}
	if len(output) != len(imgData) {
		t.Errorf("length %d != %d", len(output), len(imgData))
	}
}

func TestEmptyObstacles(t *testing.T) {
	conv := convertPlanningObstaclesToDetection([]planning.Obstacle{})
	if len(conv) != 0 {
		t.Errorf("expected 0, got %d", len(conv))
	}
}

func TestDemoToDetection(t *testing.T) {
	demo := []DemoObstacle{
		{Name: "Person", X: 320, Y: 240, Width: 80, Height: 120},
		{Name: "Cup", X: 500, Y: 400, Width: 30, Height: 30},
	}

	conv := convertDemoObstaclesToDetection(demo)

	if len(conv) != 2 {
		t.Fatalf("expected 2, got %d", len(conv))
	}
	if conv[0].ID != "Person" {
		t.Errorf("got %s", conv[0].ID)
	}
	if conv[0].PixelTopLeft != [2]int{280, 180} {
		t.Errorf("got %v", conv[0].PixelTopLeft)
	}
	if conv[1].ID != "Cup" {
		t.Errorf("got %s", conv[1].ID)
	}
}

func TestObstaclesWithWorldCoords(t *testing.T) {
	obs := []planning.Obstacle{
		{
			Name:              "WorldTest",
			WorldTopLeft:      [2]float64{0.5, 0.5},
			WorldBottomRight:  [2]float64{1.5, 1.5},
			PixelsTopLeft:     [2]int{320, 240},
			PixelsBottomRight: [2]int{480, 360},
		},
	}

	conv := convertPlanningObstaclesToDetection(obs)

	if len(conv) != 1 {
		t.Fatalf("expected 1, got %d", len(conv))
	}
	if conv[0].WorldTopLeft[0] != 0.5 {
		t.Errorf("WorldTopLeft[0] = %f", conv[0].WorldTopLeft[0])
	}
	if conv[0].WorldBottomRight[1] != 1.5 {
		t.Errorf("WorldBottomRight[1] = %f", conv[0].WorldBottomRight[1])
	}
}

func TestFullObstacleAddFlow(t *testing.T) {
	pipeline := detection.NewDetectionPipeline(detection.AprilTagConfig{Family: "tag36h11"})

	input := []planning.Obstacle{
		{
			Name:              "UserAdded",
			WorldTopLeft:      [2]float64{0.0, 0.0},
			WorldBottomRight:  [2]float64{1.0, 1.0},
			PixelsTopLeft:     [2]int{100, 100},
			PixelsBottomRight: [2]int{300, 300},
		},
	}

	converted := convertPlanningObstaclesToDetection(input)
	pipeline.SetObstacles(converted)

	stored := pipeline.GetObstacles()
	if len(stored) != 1 {
		t.Fatalf("expected 1, got %d", len(stored))
	}

	if stored[0].ID != "UserAdded" {
		t.Errorf("expected 'UserAdded', got '%s'", stored[0].ID)
	}

	result := &detection.DetectionResult{
		Tags:            []detection.AprilTag{},
		FusedDetections: []detection.FusedDetection{}, Timestamp: 0, FrameIdx: 0,
	}

	imgData := make([]byte, 640*480*3)
	output := pipeline.DrawResults(imgData, 640, 480, result)

	if output == nil {
		t.Error("DrawResults returned nil")
	}
}

// TestDemoTargetCapturesFitCleanly guards the demo-mode calibration target:
// it must be a layout the real fit rates "good", otherwise the wizard demo
// (and its Playwright coverage) would show a misleading failure.
func TestDemoTargetCapturesFitCleanly(t *testing.T) {
	for _, size := range [][2]int{{1280, 720}, {1920, 1080}} {
		fit, err := position.FitTarget(demoTargetCaptures(size[0], size[1]))
		if err != nil {
			t.Fatalf("%dx%d: FitTarget failed: %v", size[0], size[1], err)
		}
		if fit.Rating != position.RatingGood {
			t.Errorf("%dx%d: rating = %s (quality %.2f cm), want good", size[0], size[1], fit.Rating, fit.QualityCm)
		}
	}
}
