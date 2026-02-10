package detection

import (
	"testing"
)

func TestYOLODetectionsToDynamicObstacles_Empty(t *testing.T) {
	result := YOLODetectionsToDynamicObstacles(nil, nil, nil, 0.5)
	if len(result) != 0 {
		t.Errorf("Expected empty slice for empty input, got %v", result)
	}
}

func TestYOLODetectionsToDynamicObstacles_FiltersLowConfidence(t *testing.T) {
	detections := []YOLODetection{
		{Bbox: &BoundingBox{X1: 0, Y1: 0, X2: 100, Y2: 100}, Confidence: 0.3, ClassName: "person"},
		{Bbox: &BoundingBox{X1: 0, Y1: 0, X2: 100, Y2: 100}, Confidence: 0.9, ClassName: "person"},
	}

	result := YOLODetectionsToDynamicObstacles(detections, nil, nil, 0.5)

	if len(result) != 1 {
		t.Errorf("Expected 1 obstacle, got %d", len(result))
	}
	if result[0].Confidence != 0.9 {
		t.Errorf("Expected confidence 0.9, got %v", result[0].Confidence)
	}
}

func TestYOLODetectionsToDynamicObstacles_FiltersByClass(t *testing.T) {
	detections := []YOLODetection{
		{Bbox: &BoundingBox{X1: 0, Y1: 0, X2: 100, Y2: 100}, Confidence: 0.9, ClassName: "person"},
		{Bbox: &BoundingBox{X1: 0, Y1: 0, X2: 100, Y2: 100}, Confidence: 0.9, ClassName: "cup"},
		{Bbox: &BoundingBox{X1: 0, Y1: 0, X2: 100, Y2: 100}, Confidence: 0.9, ClassName: "cat"},
	}

	relevantClasses := map[string]bool{"person": true, "cup": true}
	result := YOLODetectionsToDynamicObstacles(detections, nil, relevantClasses, 0.5)

	if len(result) != 2 {
		t.Errorf("Expected 2 obstacles, got %d", len(result))
	}
}

func TestYOLODetectionsToDynamicObstacles_CalculatesRadius(t *testing.T) {
	detections := []YOLODetection{
		{Bbox: &BoundingBox{X1: 0, Y1: 0, X2: 100, Y2: 60}, Confidence: 0.9, ClassName: "person"},
	}

	result := YOLODetectionsToDynamicObstacles(detections, nil, nil, 0.5)

	if len(result) != 1 {
		t.Fatalf("Expected 1 obstacle, got %d", len(result))
	}

	expectedRadius := 50.0
	if result[0].Radius != expectedRadius {
		t.Errorf("Expected radius %v, got %v", expectedRadius, result[0].Radius)
	}
}

func TestYOLODetectionsToDynamicObstacles_NilBbox(t *testing.T) {
	detections := []YOLODetection{
		{Bbox: nil, Confidence: 0.9, ClassName: "person"},
		{Bbox: &BoundingBox{X1: 0, Y1: 0, X2: 100, Y2: 100}, Confidence: 0.9, ClassName: "cup"},
	}

	result := YOLODetectionsToDynamicObstacles(detections, nil, nil, 0.5)

	if len(result) != 1 {
		t.Errorf("Expected 1 obstacle, got %d", len(result))
	}
}

func TestYOLODetectionsToDynamicObstacles_UsesPixelCoordsWhenNotCalibrated(t *testing.T) {
	detections := []YOLODetection{
		{Bbox: &BoundingBox{X1: 100, Y1: 200, X2: 200, Y2: 300}, Confidence: 0.9, ClassName: "person"},
	}

	result := YOLODetectionsToDynamicObstacles(detections, nil, nil, 0.5)

	if len(result) != 1 {
		t.Fatalf("Expected 1 obstacle, got %d", len(result))
	}

	expectedX := 150.0
	expectedY := 250.0
	if result[0].X != expectedX {
		t.Errorf("Expected X %v, got %v", expectedX, result[0].X)
	}
	if result[0].Y != expectedY {
		t.Errorf("Expected Y %v, got %v", expectedY, result[0].Y)
	}
}
