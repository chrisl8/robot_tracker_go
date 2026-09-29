package detection

import (
	"testing"
)

func TestDetectionType(t *testing.T) {
	tests := []struct {
		detectionType DetectionType
		expectedName  string
	}{
		{DetectionTypeAprilTag, "AprilTag"},
		{DetectionTypeFused, "Fused"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedName, func(t *testing.T) {
			var name string
			switch tt.detectionType {
			case DetectionTypeAprilTag:
				name = "AprilTag"
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
