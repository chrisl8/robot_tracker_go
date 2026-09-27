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

func TestEffectiveMaxFPS(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want int
	}{
		{"nil config", nil, DefaultMaxFPS},
		{"unset", &Config{}, DefaultMaxFPS},
		{"negative", &Config{Processing: ProcessingConfig{MaxFPS: -3}}, DefaultMaxFPS},
		{"configured", &Config{Processing: ProcessingConfig{MaxFPS: 20}}, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.EffectiveMaxFPS(); got != tt.want {
				t.Errorf("EffectiveMaxFPS() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEffectiveForeground(t *testing.T) {
	off, on := false, true

	t.Run("defaults: detection on, shadow mode", func(t *testing.T) {
		got := (&Config{}).EffectiveForeground()
		if !got.Enabled || got.ApplyToPlanner {
			t.Errorf("enabled=%v apply=%v, want detection on and planner steering off", got.Enabled, got.ApplyToPlanner)
		}
		if got.Scale != 0.5 || got.Threshold != 22 || got.AppearMs != 400 || got.VanishMs != 1500 ||
			got.MaxBlobs != 8 || got.AbsorbAfterSec != 0 {
			t.Errorf("unexpected defaults: %+v", got)
		}
		if !got.PersistBackground || got.PersistIntervalSec != 30 {
			t.Errorf("background persistence should default to on with a 30s interval, got %+v", got)
		}
		if got.ShadowAlphaMin != 0.35 || got.ShadowAlphaMax != 0.98 || got.ShadowChromaMax != 0.12 {
			t.Errorf("unexpected shadow-suppression defaults: %+v", got)
		}
	})
	t.Run("nil config", func(t *testing.T) {
		if got := (*Config)(nil).EffectiveForeground(); !got.Enabled || got.TauSec != 60 {
			t.Errorf("nil config should give defaults, got %+v", got)
		}
	})
	t.Run("explicit values win, including switching off", func(t *testing.T) {
		cfg := &Config{Foreground: ForegroundConfig{
			Enabled: &off, ApplyToPlanner: &on, Threshold: 30, AppearMs: 250, AbsorbAfterSec: 600,
			PersistBackground: &off, PersistIntervalSec: 10,
			ShadowAlphaMin: 0.4, ShadowAlphaMax: 0.9, ShadowChromaMax: 0.2,
		}}
		got := cfg.EffectiveForeground()
		if got.Enabled || !got.ApplyToPlanner || got.Threshold != 30 || got.AppearMs != 250 || got.AbsorbAfterSec != 600 {
			t.Errorf("explicit values not honoured: %+v", got)
		}
		if got.Scale != 0.5 {
			t.Errorf("unset fields should still default, Scale=%v", got.Scale)
		}
		if got.PersistBackground || got.PersistIntervalSec != 10 {
			t.Errorf("persistence settings not honoured: %+v", got)
		}
		if got.ShadowAlphaMin != 0.4 || got.ShadowAlphaMax != 0.9 || got.ShadowChromaMax != 0.2 {
			t.Errorf("shadow-suppression settings not honoured: %+v", got)
		}
	})
}

// TestShippedConfigLoads guards the repository's own config file: it must parse,
// enable the foreground detector, and have YOLO switched off.
func TestShippedConfigLoads(t *testing.T) {
	cfg, err := Load("../../config/tracking_config.yaml")
	if err != nil {
		t.Fatalf("shipped config does not load: %v", err)
	}
	fg := cfg.EffectiveForeground()
	if !fg.Enabled || fg.Scale <= 0 || fg.AppearMs <= 0 {
		t.Errorf("shipped foreground settings look wrong: %+v", fg)
	}
	if cfg.YOLO.ModelPath != "" {
		t.Errorf("YOLO should be off in the shipped config, model = %q", cfg.YOLO.ModelPath)
	}
	if got := cfg.EffectiveMaxFPS(); got < 1 {
		t.Errorf("max fps = %d", got)
	}
}
