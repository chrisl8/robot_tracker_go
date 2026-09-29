package detection

import (
	"testing"
	"time"
)

func TestNewDetectionPipeline(t *testing.T) {
	pipeline := NewDetectionPipeline(AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})
	if pipeline == nil {
		t.Fatal("NewDetectionPipeline returned nil")
	}
	if pipeline.tagDetector == nil {
		t.Error("tagDetector is nil")
	}
}

func TestDetectionPipeline_Detect(t *testing.T) {
	pipeline := NewDetectionPipeline(AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})

	timestamp := float64(time.Now().UnixNano()) / 1e9
	result := pipeline.Detect([]byte{}, 640, 480, timestamp, 1)

	if result == nil {
		t.Fatal("Detect returned nil")
	}
	if result.Timestamp != timestamp {
		t.Errorf("Timestamp = %f, want %f", result.Timestamp, timestamp)
	}
	if result.FrameIdx != 1 {
		t.Errorf("FrameIdx = %d, want 1", result.FrameIdx)
	}
	if len(result.Tags) != 0 {
		t.Errorf("Expected 0 tags (empty image input), got %d", len(result.Tags))
	}
	if len(result.FusedDetections) != 0 {
		t.Errorf("Expected 0 fused detections, got %d", len(result.FusedDetections))
	}
}

func TestDetectionPipeline_Detect_NilTagDetectorDoesNotPanic(t *testing.T) {
	// Simulates NewAprilTagDetector returning an error: tagDetector stays nil.
	// Detect must degrade to "no tags" instead of nil-pointer-dereferencing.
	pipeline := &DetectionPipeline{obstacleDrawer: NewObstacleDrawer()}

	timestamp := float64(time.Now().UnixNano()) / 1e9
	result := pipeline.Detect([]byte{1, 2, 3}, 640, 480, timestamp, 1)

	if result == nil {
		t.Fatal("Detect returned nil")
	}
	if len(result.Tags) != 0 {
		t.Errorf("Expected 0 tags with nil tagDetector, got %d", len(result.Tags))
	}
	if len(result.FusedDetections) != 0 {
		t.Errorf("Expected 0 fused detections with nil tagDetector, got %d", len(result.FusedDetections))
	}
}

func TestDetectionPipeline_fuseDetections(t *testing.T) {
	pipeline := NewDetectionPipeline(AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})

	t.Run("no detections", func(t *testing.T) {
		fused := pipeline.fuseDetections([]AprilTag{})
		if len(fused) != 0 {
			t.Errorf("Expected 0 fused detections, got %d", len(fused))
		}
	})

	t.Run("only tags", func(t *testing.T) {
		tags := []AprilTag{
			{
				TagID:   1,
				Family:  "tag36h11",
				CenterX: 100,
				CenterY: 100,
				Corners: [4][2]float64{{0, 0}, {50, 0}, {50, 50}, {0, 50}},
			},
		}
		fused := pipeline.fuseDetections(tags)

		if len(fused) != 1 {
			t.Fatalf("Expected 1 fused detection, got %d", len(fused))
		}
		if fused[0].DetectionType != DetectionTypeAprilTag {
			t.Errorf("DetectionType = %v, want %v", fused[0].DetectionType, DetectionTypeAprilTag)
		}
		if fused[0].TagID == nil || *fused[0].TagID != 1 {
			t.Error("TagID should be 1")
		}
		if fused[0].Source != "april_tag" {
			t.Errorf("Source = %s, want april_tag", fused[0].Source)
		}
	})

	t.Run("multiple tags", func(t *testing.T) {
		tags := []AprilTag{
			{TagID: 1, CenterX: 100, CenterY: 100, Corners: [4][2]float64{{0, 0}, {50, 0}, {50, 50}, {0, 50}}},
			{TagID: 2, CenterX: 200, CenterY: 200, Corners: [4][2]float64{{200, 200}, {250, 200}, {250, 250}, {200, 250}}},
		}
		fused := pipeline.fuseDetections(tags)

		if len(fused) != 2 {
			t.Fatalf("Expected 2 fused detections, got %d", len(fused))
		}
	})
}

func TestDetectionPipeline_tagToBbox(t *testing.T) {
	pipeline := NewDetectionPipeline(AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})

	tests := []struct {
		name       string
		tag        AprilTag
		expectedX1 int
		expectedY1 int
		expectedX2 int
		expectedY2 int
	}{
		{
			name:       "axis aligned",
			tag:        AprilTag{Corners: [4][2]float64{{10, 10}, {60, 10}, {60, 60}, {10, 60}}},
			expectedX1: 10,
			expectedY1: 10,
			expectedX2: 60,
			expectedY2: 60,
		},
		{
			name:       "rotated corners",
			tag:        AprilTag{Corners: [4][2]float64{{50, 0}, {100, 50}, {50, 100}, {0, 50}}},
			expectedX1: 0,
			expectedY1: 0,
			expectedX2: 100,
			expectedY2: 100,
		},
		{
			name:       "single point",
			tag:        AprilTag{Corners: [4][2]float64{{100, 100}, {100, 100}, {100, 100}, {100, 100}}},
			expectedX1: 100,
			expectedY1: 100,
			expectedX2: 100,
			expectedY2: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bbox := pipeline.tagToBbox(tt.tag)
			if bbox.X1 != tt.expectedX1 || bbox.Y1 != tt.expectedY1 ||
				bbox.X2 != tt.expectedX2 || bbox.Y2 != tt.expectedY2 {
				t.Errorf("tagToBbox() = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
					bbox.X1, bbox.Y1, bbox.X2, bbox.Y2,
					tt.expectedX1, tt.expectedY1, tt.expectedX2, tt.expectedY2)
			}
		})
	}
}

func TestDetectionResult_Empty(t *testing.T) {
	result := &DetectionResult{
		Tags:            []AprilTag{},
		FusedDetections: []FusedDetection{},
		Timestamp:       1234.5,
		FrameIdx:        10,
	}

	if len(result.Tags) != 0 {
		t.Error("Tags should be empty")
	}
	if len(result.FusedDetections) != 0 {
		t.Error("FusedDetections should be empty")
	}
	if result.Timestamp != 1234.5 {
		t.Errorf("Timestamp = %f, want 1234.5", result.Timestamp)
	}
	if result.FrameIdx != 10 {
		t.Errorf("FrameIdx = %d, want 10", result.FrameIdx)
	}
}
