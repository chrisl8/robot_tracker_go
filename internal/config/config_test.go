package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	content := `
robots:
  - tag_id: 1
    name: "robot_1"
    diameter: 0.18
    speed: 0.15
  - tag_id: 2
    name: "robot_2"
    diameter: 0.15
    speed: 0.2

planning:
  step_size: 0.05
  replan_interval: 0.5
  avoidance:
    safety_margin: 0.05

yolo:
  model: assets/yolov8n.onnx
  conf_thres: 0.5
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if len(cfg.Robots) != 2 {
		t.Errorf("Expected 2 robots, got %d", len(cfg.Robots))
	}

	robot := cfg.GetRobotByTagID(1)
	if robot == nil {
		t.Error("GetRobotByTagID(1) returned nil")
		return
	}
	if robot.Name != "robot_1" {
		t.Errorf("robot.Name = %s, want robot_1", robot.Name)
	}

	if cfg.YOLO.ConfThres != 0.5 {
		t.Errorf("YOLO.ConfThres = %f, want 0.5", cfg.YOLO.ConfThres)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("nonexistent.yaml")
	if err == nil {
		t.Error("Load() should return error for nonexistent file")
	}
}

func TestGetRobotByTagID_NotFound(t *testing.T) {
	cfg := &Config{
		Robots: []RobotConfig{
			{TagID: 1, Name: "robot_1"},
		},
	}

	robot := cfg.GetRobotByTagID(999)
	if robot != nil {
		t.Error("GetRobotByTagID(999) should return nil")
	}
}

func TestGetRobotByName_NotFound(t *testing.T) {
	cfg := &Config{
		Robots: []RobotConfig{
			{TagID: 1, Name: "robot_1"},
		},
	}

	robot := cfg.GetRobotByName("nonexistent")
	if robot != nil {
		t.Error("GetRobotByName() should return nil for nonexistent robot")
	}
}

func TestLoad_ReservedCalibrationTagIDs(t *testing.T) {
	tests := []struct {
		name    string
		tagID   int
		wantErr bool
	}{
		{"ordinary robot id", 1, false},
		{"just below reserved range", 99, false},
		{"first reserved id", 100, true},
		{"last reserved id", 104, true},
		{"just above reserved range", 105, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			content := fmt.Sprintf("robots:\n  - tag_id: %d\n    name: \"r\"\n    diameter: 0.18\n", tt.tagID)
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatalf("writing config: %v", err)
			}

			_, err := Load(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
