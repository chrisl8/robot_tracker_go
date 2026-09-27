//go:build gocv

package ui

import "testing"

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
