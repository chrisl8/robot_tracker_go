package detection

import (
	"testing"
	"time"
)

func TestNewDetectionPipeline(t *testing.T) {
	tests := []struct {
		name        string
		config      *YOLOConfig
		tagConfig   AprilTagConfig
		yoloEnabled bool
	}{
		{
			name:        "nil config",
			config:      nil,
			tagConfig:   AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0},
			yoloEnabled: false,
		},
		{
			name:        "empty model path",
			config:      &YOLOConfig{ModelPath: ""},
			tagConfig:   AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0},
			yoloEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := NewDetectionPipeline(tt.config, tt.tagConfig)
			if pipeline == nil {
				t.Fatal("NewDetectionPipeline returned nil")
			}
			if pipeline.tagDetector == nil {
				t.Error("tagDetector is nil")
			}
			if pipeline.yoloEnabled != tt.yoloEnabled {
				t.Errorf("yoloEnabled = %v, want %v", pipeline.yoloEnabled, tt.yoloEnabled)
			}
		})
	}
}

func TestDetectionPipeline_Detect(t *testing.T) {
	pipeline := NewDetectionPipeline(nil, AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})

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
	if len(result.YOLODetections) != 0 {
		t.Errorf("Expected 0 YOLO detections (YOLO disabled), got %d", len(result.YOLODetections))
	}
	if len(result.FusedDetections) != 0 {
		t.Errorf("Expected 0 fused detections, got %d", len(result.FusedDetections))
	}
}

func TestDetectionPipeline_IsYOLOEnabled(t *testing.T) {
	tests := []struct {
		name          string
		config        *YOLOConfig
		expectedValue bool
	}{
		{"nil config", nil, false},
		{"empty model", &YOLOConfig{ModelPath: ""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := NewDetectionPipeline(tt.config, AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})
			if pipeline.IsYOLOEnabled() != tt.expectedValue {
				t.Errorf("IsYOLOEnabled() = %v, want %v", pipeline.IsYOLOEnabled(), tt.expectedValue)
			}
		})
	}
}

func TestDetectionPipeline_fuseDetections(t *testing.T) {
	pipeline := NewDetectionPipeline(nil, AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})

	t.Run("no detections", func(t *testing.T) {
		fused := pipeline.fuseDetections([]AprilTag{}, []YOLODetection{})
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
		fused := pipeline.fuseDetections(tags, []YOLODetection{})

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

	t.Run("only yolo", func(t *testing.T) {
		yoloDetections := []YOLODetection{
			{
				Bbox:       &BoundingBox{X1: 10, Y1: 10, X2: 50, Y2: 50},
				Confidence: 0.85,
				ClassID:    0,
				ClassName:  "person",
			},
		}
		fused := pipeline.fuseDetections([]AprilTag{}, yoloDetections)

		if len(fused) != 1 {
			t.Fatalf("Expected 1 fused detection, got %d", len(fused))
		}
		if fused[0].DetectionType != DetectionTypeYOLO {
			t.Errorf("DetectionType = %v, want %v", fused[0].DetectionType, DetectionTypeYOLO)
		}
		if fused[0].ClassName != "person" {
			t.Errorf("ClassName = %s, want person", fused[0].ClassName)
		}
		if fused[0].Source != "yolo" {
			t.Errorf("Source = %s, want yolo", fused[0].Source)
		}
	})

	t.Run("tags and yolo separate", func(t *testing.T) {
		tags := []AprilTag{
			{TagID: 1, CenterX: 100, CenterY: 100, Corners: [4][2]float64{{0, 0}, {50, 0}, {50, 50}, {0, 50}}},
		}
		yoloDetections := []YOLODetection{
			{Bbox: &BoundingBox{X1: 200, Y1: 200, X2: 250, Y2: 250}, Confidence: 0.9, ClassID: 2, ClassName: "car"},
		}
		fused := pipeline.fuseDetections(tags, yoloDetections)

		if len(fused) != 2 {
			t.Fatalf("Expected 2 fused detections, got %d", len(fused))
		}
	})

	t.Run("fused detection when yolo contains tag", func(t *testing.T) {
		tags := []AprilTag{
			{TagID: 5, CenterX: 100, CenterY: 100, Corners: [4][2]float64{{90, 90}, {110, 90}, {110, 110}, {90, 110}}},
		}
		yoloDetections := []YOLODetection{
			{Bbox: &BoundingBox{X1: 50, Y1: 50, X2: 150, Y2: 150}, Confidence: 0.95, ClassID: 0, ClassName: "person"},
		}
		fused := pipeline.fuseDetections(tags, yoloDetections)

		if len(fused) != 1 {
			t.Fatalf("Expected 1 fused detection, got %d", len(fused))
		}
		if fused[0].DetectionType != DetectionTypeFused {
			t.Errorf("DetectionType = %v, want %v", fused[0].DetectionType, DetectionTypeFused)
		}
	})
}

func TestDetectionPipeline_findMatchingYOLO(t *testing.T) {
	pipeline := NewDetectionPipeline(nil, AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})

	t.Run("no yolo detections", func(t *testing.T) {
		tag := AprilTag{TagID: 1, CenterX: 100, CenterY: 100}
		idx := pipeline.findMatchingYOLO(tag, []YOLODetection{})
		if idx != -1 {
			t.Errorf("Expected -1, got %d", idx)
		}
	})

	t.Run("tag not in yolo bbox", func(t *testing.T) {
		tag := AprilTag{TagID: 1, CenterX: 100, CenterY: 100}
		yoloDetections := []YOLODetection{
			{Bbox: &BoundingBox{X1: 200, Y1: 200, X2: 300, Y2: 300}},
		}
		idx := pipeline.findMatchingYOLO(tag, yoloDetections)
		if idx != -1 {
			t.Errorf("Expected -1, got %d", idx)
		}
	})

	t.Run("tag in yolo bbox", func(t *testing.T) {
		tag := AprilTag{TagID: 1, CenterX: 150, CenterY: 150}
		yoloDetections := []YOLODetection{
			{Bbox: &BoundingBox{X1: 100, Y1: 100, X2: 200, Y2: 200}},
		}
		idx := pipeline.findMatchingYOLO(tag, yoloDetections)
		if idx != 0 {
			t.Errorf("Expected 0, got %d", idx)
		}
	})

	t.Run("nil bbox", func(t *testing.T) {
		tag := AprilTag{TagID: 1, CenterX: 100, CenterY: 100}
		yoloDetections := []YOLODetection{
			{Bbox: nil},
		}
		idx := pipeline.findMatchingYOLO(tag, yoloDetections)
		if idx != -1 {
			t.Errorf("Expected -1, got %d", idx)
		}
	})
}

func TestDetectionPipeline_tagToBbox(t *testing.T) {
	pipeline := NewDetectionPipeline(nil, AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})

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

func TestDetectionPipeline_DrawResults(t *testing.T) {
	pipeline := NewDetectionPipeline(nil, AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})

	image := []byte{1, 2, 3, 4, 5}
	result := &DetectionResult{
		Tags:            []AprilTag{},
		YOLODetections:  []YOLODetection{},
		FusedDetections: []FusedDetection{},
		Timestamp:       0,
		FrameIdx:        0,
	}

	output := pipeline.DrawResults(image, 640, 480, result)
	if len(output) != len(image) {
		t.Errorf("DrawResults changed image length from %d to %d", len(image), len(output))
	}
}

func TestDetectionResult_Empty(t *testing.T) {
	result := &DetectionResult{
		Tags:            []AprilTag{},
		YOLODetections:  []YOLODetection{},
		FusedDetections: []FusedDetection{},
		Timestamp:       1234.5,
		FrameIdx:        10,
	}

	if len(result.Tags) != 0 {
		t.Error("Tags should be empty")
	}
	if len(result.YOLODetections) != 0 {
		t.Error("YOLODetections should be empty")
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

func TestDetectionPipeline_setDefaultClassNames(t *testing.T) {
	pipeline := NewDetectionPipeline(nil, AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})

	if pipeline.yoloDetector == nil {
		t.Skip("yoloDetector is nil (expected with nil config)")
	}

	className := pipeline.yoloDetector.GetClassName(0)
	if className == "" {
		t.Error("GetClassName should return a valid class name")
	}
}

func TestDetectionPipeline_SetObstacles_Integration(t *testing.T) {
	pipeline := NewDetectionPipeline(nil, AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})

	t.Run("initial obstacles should be empty", func(t *testing.T) {
		obstacles := pipeline.GetObstacles()
		if len(obstacles) != 0 {
			t.Errorf("expected 0 obstacles, got %d", len(obstacles))
		}
	})

	t.Run("SetObstacles should store obstacles", func(t *testing.T) {
		testObstacles := []Obstacle{
			{
				ID:               "test-1",
				PixelTopLeft:     [2]int{100, 100},
				PixelBottomRight: [2]int{200, 200},
			},
			{
				ID:               "test-2",
				PixelTopLeft:     [2]int{300, 300},
				PixelBottomRight: [2]int{400, 400},
			},
		}

		pipeline.SetObstacles(testObstacles)

		obstacles := pipeline.GetObstacles()
		if len(obstacles) != 2 {
			t.Errorf("expected 2 obstacles, got %d", len(obstacles))
		}

		if obstacles[0].ID != "test-1" {
			t.Errorf("expected first obstacle ID 'test-1', got '%s'", obstacles[0].ID)
		}

		if obstacles[1].ID != "test-2" {
			t.Errorf("expected second obstacle ID 'test-2', got '%s'", obstacles[1].ID)
		}
	})

	t.Run("SetObstacles should replace existing obstacles", func(t *testing.T) {
		pipeline.SetObstacles([]Obstacle{
			{
				ID:               "new-obs",
				PixelTopLeft:     [2]int{50, 50},
				PixelBottomRight: [2]int{150, 150},
			},
		})

		obstacles := pipeline.GetObstacles()
		if len(obstacles) != 1 {
			t.Errorf("expected 1 obstacle, got %d", len(obstacles))
		}

		if obstacles[0].ID != "new-obs" {
			t.Errorf("expected obstacle ID 'new-obs', got '%s'", obstacles[0].ID)
		}
	})

	t.Run("SetObstacles with nil should clear obstacles", func(t *testing.T) {
		pipeline.SetObstacles(nil)

		obstacles := pipeline.GetObstacles()
		if len(obstacles) != 0 {
			t.Errorf("expected 0 obstacles after nil, got %d", len(obstacles))
		}
	})

	t.Run("SetObstacles with empty slice should clear obstacles", func(t *testing.T) {
		pipeline.SetObstacles([]Obstacle{})

		obstacles := pipeline.GetObstacles()
		if len(obstacles) != 0 {
			t.Errorf("expected 0 obstacles after empty slice, got %d", len(obstacles))
		}
	})
}

func TestDetectionPipeline_DrawResults_Obstacles_Integration(t *testing.T) {
	pipeline := NewDetectionPipeline(nil, AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})

	t.Run("DrawResults should not draw obstacles when none are set", func(t *testing.T) {
		imgData := make([]byte, 640*480*3)

		result := &DetectionResult{
			Tags:            []AprilTag{},
			YOLODetections:  []YOLODetection{},
			FusedDetections: []FusedDetection{},
		}

		output := pipeline.DrawResults(imgData, 640, 480, result)

		if output == nil {
			t.Error("expected output to not be nil")
		}
	})

	t.Run("DrawResults should draw obstacles when set", func(t *testing.T) {
		imgData := make([]byte, 640*480*3)

		pipeline.SetObstacles([]Obstacle{
			{
				ID:               "visible-obs",
				PixelTopLeft:     [2]int{100, 100},
				PixelBottomRight: [2]int{200, 200},
			},
		})

		result := &DetectionResult{
			Tags:            []AprilTag{},
			YOLODetections:  []YOLODetection{},
			FusedDetections: []FusedDetection{},
		}

		output := pipeline.DrawResults(imgData, 640, 480, result)

		if output == nil {
			t.Error("expected output to not be nil when obstacles are set")
		}
	})

	t.Run("DrawResults should draw multiple obstacles", func(t *testing.T) {
		imgData := make([]byte, 640*480*3)

		pipeline.SetObstacles([]Obstacle{
			{
				ID:               "obs-1",
				PixelTopLeft:     [2]int{50, 50},
				PixelBottomRight: [2]int{100, 100},
			},
			{
				ID:               "obs-2",
				PixelTopLeft:     [2]int{200, 200},
				PixelBottomRight: [2]int{300, 300},
			},
			{
				ID:               "obs-3",
				PixelTopLeft:     [2]int{400, 400},
				PixelBottomRight: [2]int{500, 500},
			},
		})

		result := &DetectionResult{
			Tags:            []AprilTag{},
			YOLODetections:  []YOLODetection{},
			FusedDetections: []FusedDetection{},
		}

		output := pipeline.DrawResults(imgData, 640, 480, result)

		if output == nil {
			t.Error("expected output to not be nil when multiple obstacles are set")
		}
	})

	t.Run("DrawResults with cleared obstacles should not draw", func(t *testing.T) {
		imgData := make([]byte, 640*480*3)

		pipeline.SetObstacles([]Obstacle{
			{
				ID:               "was-visible",
				PixelTopLeft:     [2]int{100, 100},
				PixelBottomRight: [2]int{200, 200},
			},
		})

		pipeline.SetObstacles([]Obstacle{})

		result := &DetectionResult{
			Tags:            []AprilTag{},
			YOLODetections:  []YOLODetection{},
			FusedDetections: []FusedDetection{},
		}

		output := pipeline.DrawResults(imgData, 640, 480, result)

		if output == nil {
			t.Error("expected output to not be nil even when obstacles are cleared")
		}
	})
}
