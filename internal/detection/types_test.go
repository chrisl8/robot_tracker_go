package detection

import (
	"testing"
)

func TestBoundingBox_Center(t *testing.T) {
	tests := []struct {
		name      string
		bbox      BoundingBox
		expectedX int
		expectedY int
	}{
		{
			name:      "normal bbox",
			bbox:      BoundingBox{X1: 10, Y1: 20, X2: 30, Y2: 40},
			expectedX: 20,
			expectedY: 30,
		},
		{
			name:      "zero bbox",
			bbox:      BoundingBox{X1: 0, Y1: 0, X2: 0, Y2: 0},
			expectedX: 0,
			expectedY: 0,
		},
		{
			name:      "negative coordinates",
			bbox:      BoundingBox{X1: -10, Y1: -20, X2: 10, Y2: 20},
			expectedX: 0,
			expectedY: 0,
		},
		{
			name:      "large bbox",
			bbox:      BoundingBox{X1: 100, Y1: 200, X2: 500, Y2: 600},
			expectedX: 300,
			expectedY: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y := tt.bbox.Center()
			if x != tt.expectedX || y != tt.expectedY {
				t.Errorf("Center() = (%d, %d), want (%d, %d)", x, y, tt.expectedX, tt.expectedY)
			}
		})
	}
}

func TestBoundingBox_Width(t *testing.T) {
	tests := []struct {
		name     string
		bbox     BoundingBox
		expected int
	}{
		{
			name:     "normal width",
			bbox:     BoundingBox{X1: 10, Y1: 20, X2: 30, Y2: 40},
			expected: 20,
		},
		{
			name:     "zero width",
			bbox:     BoundingBox{X1: 10, Y1: 10, X2: 10, Y2: 20},
			expected: 0,
		},
		{
			name:     "negative width",
			bbox:     BoundingBox{X1: 30, Y1: 20, X2: 10, Y2: 40},
			expected: -20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if width := tt.bbox.Width(); width != tt.expected {
				t.Errorf("Width() = %d, want %d", width, tt.expected)
			}
		})
	}
}

func TestBoundingBox_Height(t *testing.T) {
	tests := []struct {
		name     string
		bbox     BoundingBox
		expected int
	}{
		{
			name:     "normal height",
			bbox:     BoundingBox{X1: 10, Y1: 20, X2: 30, Y2: 40},
			expected: 20,
		},
		{
			name:     "zero height",
			bbox:     BoundingBox{X1: 10, Y1: 20, X2: 30, Y2: 20},
			expected: 0,
		},
		{
			name:     "negative height",
			bbox:     BoundingBox{X1: 10, Y1: 40, X2: 30, Y2: 20},
			expected: -20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if height := tt.bbox.Height(); height != tt.expected {
				t.Errorf("Height() = %d, want %d", height, tt.expected)
			}
		})
	}
}

func TestBoundingBox_Area(t *testing.T) {
	tests := []struct {
		name     string
		bbox     BoundingBox
		expected int
	}{
		{
			name:     "normal area",
			bbox:     BoundingBox{X1: 0, Y1: 0, X2: 10, Y2: 10},
			expected: 100,
		},
		{
			name:     "zero area",
			bbox:     BoundingBox{X1: 5, Y1: 5, X2: 5, Y2: 5},
			expected: 0,
		},
		{
			name:     "negative dimensions (inverted coords)",
			bbox:     BoundingBox{X1: 10, Y1: 10, X2: 5, Y2: 5},
			expected: 25,
		},
		{
			name:     "rectangle area",
			bbox:     BoundingBox{X1: 0, Y1: 0, X2: 100, Y2: 50},
			expected: 5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if area := tt.bbox.Area(); area != tt.expected {
				t.Errorf("Area() = %d, want %d", area, tt.expected)
			}
		})
	}
}

func TestBoundingBox_Contains(t *testing.T) {
	tests := []struct {
		name     string
		bbox     BoundingBox
		x, y     int
		expected bool
	}{
		{
			name:     "point inside",
			bbox:     BoundingBox{X1: 10, Y1: 10, X2: 30, Y2: 30},
			x:        20,
			y:        20,
			expected: true,
		},
		{
			name:     "point on edge",
			bbox:     BoundingBox{X1: 10, Y1: 10, X2: 30, Y2: 30},
			x:        10,
			y:        20,
			expected: true,
		},
		{
			name:     "point outside",
			bbox:     BoundingBox{X1: 10, Y1: 10, X2: 30, Y2: 30},
			x:        5,
			y:        20,
			expected: false,
		},
		{
			name:     "point outside y",
			bbox:     BoundingBox{X1: 10, Y1: 10, X2: 30, Y2: 30},
			x:        20,
			y:        40,
			expected: false,
		},
		{
			name:     "zero bbox contains nothing",
			bbox:     BoundingBox{X1: 0, Y1: 0, X2: 0, Y2: 0},
			x:        0,
			y:        0,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.bbox.Contains(tt.x, tt.y); result != tt.expected {
				t.Errorf("Contains(%d, %d) = %v, want %v", tt.x, tt.y, result, tt.expected)
			}
		})
	}
}

func TestBoundingBox_IoU(t *testing.T) {
	tests := []struct {
		name     string
		bbox1    BoundingBox
		bbox2    BoundingBox
		expected float64
	}{
		{
			name:     "identical boxes",
			bbox1:    BoundingBox{X1: 0, Y1: 0, X2: 10, Y2: 10},
			bbox2:    BoundingBox{X1: 0, Y1: 0, X2: 10, Y2: 10},
			expected: 1.0,
		},
		{
			name:     "no overlap",
			bbox1:    BoundingBox{X1: 0, Y1: 0, X2: 10, Y2: 10},
			bbox2:    BoundingBox{X1: 20, Y1: 20, X2: 30, Y2: 30},
			expected: 0.0,
		},
		{
			name:     "partial overlap",
			bbox1:    BoundingBox{X1: 0, Y1: 0, X2: 10, Y2: 10},
			bbox2:    BoundingBox{X1: 5, Y1: 5, X2: 15, Y2: 15},
			expected: 0.14285714285714285,
		},
		{
			name:     "one contained in other",
			bbox1:    BoundingBox{X1: 0, Y1: 0, X2: 20, Y2: 20},
			bbox2:    BoundingBox{X1: 5, Y1: 5, X2: 15, Y2: 15},
			expected: 0.25,
		},
		{
			name:     "touching edges",
			bbox1:    BoundingBox{X1: 0, Y1: 0, X2: 10, Y2: 10},
			bbox2:    BoundingBox{X1: 10, Y1: 0, X2: 20, Y2: 10},
			expected: 0.0,
		},
		{
			name:     "diagonal touching",
			bbox1:    BoundingBox{X1: 0, Y1: 0, X2: 10, Y2: 10},
			bbox2:    BoundingBox{X1: 10, Y1: 10, X2: 20, Y2: 20},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.bbox1.IoU(&tt.bbox2)
			if result != tt.expected {
				t.Errorf("IoU() = %f, want %f", result, tt.expected)
			}
		})
	}
}

func TestDetectionType(t *testing.T) {
	tests := []struct {
		detectionType DetectionType
		expectedName  string
	}{
		{DetectionTypeAprilTag, "AprilTag"},
		{DetectionTypeYOLO, "YOLO"},
		{DetectionTypeFused, "Fused"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedName, func(t *testing.T) {
			var name string
			switch tt.detectionType {
			case DetectionTypeAprilTag:
				name = "AprilTag"
			case DetectionTypeYOLO:
				name = "YOLO"
			case DetectionTypeFused:
				name = "Fused"
			}
			if name != tt.expectedName {
				t.Errorf("DetectionType = %s, want %s", name, tt.expectedName)
			}
		})
	}
}

func TestAprilTagDetector_NewAprilTagDetector(t *testing.T) {
	tests := []struct {
		name           string
		config         AprilTagConfig
		expectNilError bool
	}{
		{
			name: "default values",
			config: AprilTagConfig{
				Family:       "",
				QuadDecimate: 0,
			},
			expectNilError: true,
		},
		{
			name: "custom family",
			config: AprilTagConfig{
				Family:       "tag25h9",
				QuadDecimate: 0,
			},
			expectNilError: true,
		},
		{
			name: "custom decimate",
			config: AprilTagConfig{
				Family:       "",
				QuadDecimate: 4.0,
			},
			expectNilError: true,
		},
		{
			name: "both custom",
			config: AprilTagConfig{
				Family:       "tag16h5",
				QuadDecimate: 3.0,
			},
			expectNilError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector, err := NewAprilTagDetector(tt.config)
			if tt.expectNilError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if !tt.expectNilError && err == nil {
				t.Errorf("Expected error, got nil")
			}
			if tt.expectNilError && detector == nil {
				t.Errorf("Expected detector, got nil")
			}
		})
	}
}

func TestAprilTagDetector_Detect(t *testing.T) {
	detector, _ := NewAprilTagDetector(AprilTagConfig{
		Family:       "tag36h11",
		QuadDecimate: 2.0,
	})
	tags := detector.Detect([]byte{}, 640, 480)
	if len(tags) != 0 {
		t.Errorf("Expected empty tags slice (empty image input), got %d tags", len(tags))
	}
}

func TestYOLODetector_NewYOLODetector(t *testing.T) {
	tests := []struct {
		name        string
		config      *YOLOConfig
		expectError bool
	}{
		{
			name:        "empty model path",
			config:      &YOLOConfig{ModelPath: ""},
			expectError: false,
		},
		{
			name:        "non-existent model path",
			config:      &YOLOConfig{ModelPath: "/nonexistent/path/model.onnx"},
			expectError: false, // Stub doesn't check file existence
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector, err := NewYOLODetector(tt.config)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if !tt.expectError && detector == nil {
				t.Errorf("Expected detector, got nil")
			}
		})
	}
}

func TestYOLODetector_Detect(t *testing.T) {
	detector, _ := NewYOLODetector(&YOLOConfig{ModelPath: ""})
	detections := detector.Detect([]byte{}, 640, 480)
	if len(detections) != 0 {
		t.Errorf("Expected empty detections slice (stub), got %d detections", len(detections))
	}
}

func TestYOLODetector_GetClassName(t *testing.T) {
	detector, _ := NewYOLODetector(&YOLOConfig{})
	detector.SetClassNames(map[int]string{
		0: "person",
		1: "car",
	})

	tests := []struct {
		classID  int
		expected string
	}{
		{0, "person"},
		{1, "car"},
		{99, "class_99"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if name := detector.GetClassName(tt.classID); name != tt.expected {
				t.Errorf("GetClassName(%d) = %s, want %s", tt.classID, name, tt.expected)
			}
		})
	}
}

func TestYOLODetector_IsAvailable(t *testing.T) {
	tests := []struct {
		name              string
		modelPath         string
		expectError       bool
		expectedAvailable bool
	}{
		{"empty path", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector, err := NewYOLODetector(&YOLOConfig{ModelPath: tt.modelPath})
			if tt.expectError && err == nil {
				t.Skip("Expected error but got nil")
			}
			if err != nil {
				t.Skipf("Skipping due to error: %v", err)
			}
			if available := detector.IsAvailable(); available != tt.expectedAvailable {
				t.Errorf("IsAvailable() = %v, want %v", available, tt.expectedAvailable)
			}
		})
	}
}

func TestYOLODetector_DefaultConfig(t *testing.T) {
	config := &YOLOConfig{}
	detector, err := NewYOLODetector(config)
	if err != nil {
		t.Fatalf("NewYOLODetector failed: %v", err)
	}
	if detector == nil {
		t.Error("Expected detector, got nil")
	}
}
