//go:build gocv

package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetForegroundStateFilename(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Logitech C920", "config/foreground_bg_Logitech_C920.bin"},
		{"", "config/foreground_bg_unknown.bin"},
	}
	for _, tt := range tests {
		if got := GetForegroundStateFilename(tt.name); got != tt.want {
			t.Errorf("GetForegroundStateFilename(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestGetForegroundStateFilename_DistinctFromCalibrationFilename(t *testing.T) {
	name := "USB Camera"
	if GetForegroundStateFilename(name) == GetCalibrationFilename(name) {
		t.Error("the background-state and calibration files must not collide for the same camera")
	}
}

func TestGetObstaclesFilename(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Camera 0", "config/obstacles_Camera_0.yaml"},
		{"", "config/obstacles.yaml"},
	}
	for _, tt := range tests {
		if got := GetObstaclesFilename(tt.name); got != tt.want {
			t.Errorf("GetObstaclesFilename(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestResolveObstaclesPath covers the four resolution tiers documented on
// ResolveObstaclesPath: an existing configured override wins; a configured
// override that doesn't exist on disk is ignored in favor of the per-camera
// default; and the per-camera default itself falls back further to the
// camera-agnostic default when no camera name is known. This is a
// regression test for docs/archived/code-review-2026-09-27.md's "obstacle-file path
// resolution logic spread across three places" tech-debt finding.
func TestResolveObstaclesPath(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "custom_obstacles.yaml")
	if err := os.WriteFile(existing, []byte("obstacles: []\n"), 0600); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}
	missing := filepath.Join(dir, "does_not_exist.yaml")

	tests := []struct {
		testName       string
		configuredPath string
		cameraName     string
		want           string
	}{
		{"configured path exists: wins over camera default", existing, "Camera 0", existing},
		{"configured path missing: falls back to per-camera default", missing, "Camera 0", "config/obstacles_Camera_0.yaml"},
		{"unconfigured, camera known: per-camera default", "", "Camera 0", "config/obstacles_Camera_0.yaml"},
		{"unconfigured, no camera: generic default", "", "", "config/obstacles.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			if got := ResolveObstaclesPath(tt.configuredPath, tt.cameraName); got != tt.want {
				t.Errorf("ResolveObstaclesPath(%q, %q) = %q, want %q", tt.configuredPath, tt.cameraName, got, tt.want)
			}
		})
	}
}
