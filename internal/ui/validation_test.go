package ui

import "testing"

func TestIsValidCommand(t *testing.T) {
	for _, ok := range []string{"F", "B", "L", "R", "S"} {
		if !isValidCommand(ok) {
			t.Errorf("isValidCommand(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "f", "X", "FF", "stop", " S", "W"} {
		if isValidCommand(bad) {
			t.Errorf("isValidCommand(%q) = true, want false", bad)
		}
	}
}

func TestDestinationProblem(t *testing.T) {
	tests := []struct {
		name                          string
		robotID, x, y, frameW, frameH int
		wantProblem                   bool
	}{
		{"inside frame", 1, 100, 100, 640, 480, false},
		{"origin", 1, 0, 0, 640, 480, false},
		{"last pixel", 1, 639, 479, 640, 480, false},
		{"x at width is outside", 1, 640, 10, 640, 480, true},
		{"y at height is outside", 1, 10, 480, 640, 480, true},
		{"negative x", 1, -1, 10, 640, 480, true},
		{"negative y", 1, 10, -1, 640, 480, true},
		{"negative robot id", -1, 10, 10, 640, 480, true},
		{"robot id 0 is valid", 0, 10, 10, 640, 480, false},
		{"frame size unknown skips the upper bound", 1, 5000, 5000, 0, 0, false},
		{"frame size unknown still rejects negatives", 1, -5, 10, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := destinationProblem(tt.robotID, tt.x, tt.y, tt.frameW, tt.frameH)
			if (got != "") != tt.wantProblem {
				t.Errorf("destinationProblem = %q, wantProblem %v", got, tt.wantProblem)
			}
		})
	}
}

func TestCalibrationStateAfterCancel(t *testing.T) {
	state, _, filename, tagSize := calibrationStateAfterCancel(true, "config/calibration_x.yaml")
	if state != "calibrated" || filename != "config/calibration_x.yaml" || tagSize <= 0 {
		t.Errorf("calibrated: got state=%q filename=%q tagSize=%v", state, filename, tagSize)
	}

	state, _, filename, tagSize = calibrationStateAfterCancel(false, "config/calibration_x.yaml")
	if state != "not_calibrated" || filename != "" || tagSize != 0 {
		t.Errorf("uncalibrated: got state=%q filename=%q tagSize=%v", state, filename, tagSize)
	}
}

func TestPixelBoxProblem(t *testing.T) {
	tests := []struct {
		name        string
		tl, br      [2]int
		w, h        int
		wantProblem bool
	}{
		{"normal box", [2]int{10, 10}, [2]int{50, 50}, 640, 480, false},
		{"touching the far edge", [2]int{0, 0}, [2]int{640, 480}, 640, 480, false},
		{"past the right edge", [2]int{600, 10}, [2]int{641, 50}, 640, 480, true},
		{"past the bottom edge", [2]int{10, 400}, [2]int{50, 481}, 640, 480, true},
		{"negative corner", [2]int{-1, 10}, [2]int{50, 50}, 640, 480, true},
		{"zero width", [2]int{10, 10}, [2]int{10, 50}, 640, 480, true},
		{"inverted corners", [2]int{50, 50}, [2]int{10, 10}, 640, 480, true},
		{"unknown frame size skips the upper bound", [2]int{10, 10}, [2]int{5000, 5000}, 0, 0, false},
		{"unknown frame size still needs area", [2]int{10, 10}, [2]int{10, 10}, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pixelBoxProblem(tt.tl, tt.br, tt.w, tt.h)
			if (got != "") != tt.wantProblem {
				t.Errorf("pixelBoxProblem = %q, wantProblem %v", got, tt.wantProblem)
			}
		})
	}
}
