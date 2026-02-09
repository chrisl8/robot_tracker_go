//go:build gocv

package main

import (
	"testing"

	"robot_tracker_go/internal/detection"
	"robot_tracker_go/internal/planning"
)

func TestConvertDemoObstacles_ToDetection(t *testing.T) {
	demoObs := []DemoObstacle{
		{
			name:       "Person 1",
			x:          400,
			y:          300,
			width:      80,
			height:     120,
			confidence: 0.92,
			moving:     true,
			vx:         2,
			vy:         1,
		},
		{
			name:       "Cup",
			x:          800,
			y:          200,
			width:      40,
			height:     40,
			confidence: 0.88,
			moving:     false,
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
			name:   "Laptop",
			x:      600,
			y:      400,
			width:  70,
			height: 50,
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
	pipeline := detection.NewDetectionPipeline(nil, detection.AprilTagConfig{Family: "tag36h11"})

	demoObs := []DemoObstacle{
		{
			name:   "Chair",
			x:      200,
			y:      500,
			width:  100,
			height: 100,
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
	pipeline := detection.NewDetectionPipeline(nil, detection.AprilTagConfig{Family: "tag36h11"})

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
