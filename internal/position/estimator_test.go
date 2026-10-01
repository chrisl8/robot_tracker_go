package position

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewPositionEstimator(t *testing.T) {
	est, err := NewPositionEstimator("")
	if err != nil {
		t.Fatalf("NewPositionEstimator failed: %v", err)
	}
	if est == nil {
		t.Fatal("NewPositionEstimator returned nil")
	}
	if est.homography == nil {
		t.Error("homography should not be nil")
	}
	if est.intrinsics != nil {
		t.Error("intrinsics should be nil without calibration file")
	}
}

func TestPositionEstimator_PixelToWorld_Invalid(t *testing.T) {
	est, _ := NewPositionEstimator("")
	result := est.PixelToWorld(100, 200)

	if result.X != 100 || result.Y != 200 {
		t.Errorf("Invalid estimator should return original pixel coordinates, got (%f, %f)", result.X, result.Y)
	}
}

func TestPositionEstimator_PixelToWorld_Valid(t *testing.T) {
	est, _ := NewPositionEstimator("")
	est.homography.SetFromValues(1, 0, 0, 0, 1, 0, 0, 0, 1)

	result := est.PixelToWorld(100, 200)

	if result.X != 100 || result.Y != 200 {
		t.Errorf("Identity transform should preserve coordinates, got (%f, %f)", result.X, result.Y)
	}
}

func TestPositionEstimator_WorldToPixel_Invalid(t *testing.T) {
	est, _ := NewPositionEstimator("")
	x, y := est.WorldToPixel(Point2D{X: 1.5, Y: 2.5})

	if x != 1 || y != 2 {
		t.Errorf("Invalid estimator should return floored world coordinates, got (%d, %d)", x, y)
	}
}

func TestPositionEstimator_WorldToPixel_Valid(t *testing.T) {
	est, _ := NewPositionEstimator("")
	est.homography.SetFromValues(1, 0, 0, 0, 1, 0, 0, 0, 1)

	x, y := est.WorldToPixel(Point2D{X: 100.7, Y: 200.9})

	if x != 100 || y != 200 {
		t.Errorf("Identity transform should floor coordinates, got (%d, %d)", x, y)
	}
}

func TestPositionEstimator_GetHomography(t *testing.T) {
	est, _ := NewPositionEstimator("")
	h := est.GetHomography()

	if h == nil {
		t.Error("GetHomography should not return nil")
	}
}

func TestPositionEstimator_IsCalibrated_NotCalibrated(t *testing.T) {
	est, _ := NewPositionEstimator("")

	if est.IsCalibrated() {
		t.Error("Estimator without valid homography should not be calibrated")
	}
}

func TestPositionEstimator_IsCalibrated_Calibrated(t *testing.T) {
	est, _ := NewPositionEstimator("")
	est.homography.SetFromValues(1, 0, 0, 0, 1, 0, 0, 0, 1)

	if !est.IsCalibrated() {
		t.Error("Estimator with valid homography should be calibrated")
	}
}

func TestPositionResult_Struct(t *testing.T) {
	result := PositionResult{
		Positions: []RobotPosition{
			{TrackID: 1, X: 100, Y: 200, Confidence: 0.9, TagID: 5},
		},
		Timestamp: 1234.5,
		FrameIdx:  10,
	}

	if len(result.Positions) != 1 {
		t.Errorf("Positions length = %d, want 1", len(result.Positions))
	}
	if result.Positions[0].TrackID != 1 {
		t.Errorf("Position TrackID = %d, want 1", result.Positions[0].TrackID)
	}
	if result.Timestamp != 1234.5 {
		t.Errorf("Timestamp = %f, want 1234.5", result.Timestamp)
	}
	if result.FrameIdx != 10 {
		t.Errorf("FrameIdx = %d, want 10", result.FrameIdx)
	}
}

func TestRobotPosition_Struct(t *testing.T) {
	pos := RobotPosition{
		TrackID:    2,
		X:          150.5,
		Y:          250.7,
		Confidence: 0.85,
		TagID:      10,
	}

	if pos.TrackID != 2 {
		t.Errorf("TrackID = %d, want 2", pos.TrackID)
	}
	if pos.X != 150.5 || pos.Y != 250.7 {
		t.Errorf("Position = (%f, %f), want (150.5, 250.7)", pos.X, pos.Y)
	}
	if pos.Confidence != 0.85 {
		t.Errorf("Confidence = %f, want 0.85", pos.Confidence)
	}
	if pos.TagID != 10 {
		t.Errorf("TagID = %d, want 10", pos.TagID)
	}
}

func TestCameraIntrinsics_Struct(t *testing.T) {
	intrinsics := CameraIntrinsics{
		Width:  640,
		Height: 480,
	}

	if intrinsics.Width != 640 {
		t.Errorf("Width = %d, want 640", intrinsics.Width)
	}
	if intrinsics.Height != 480 {
		t.Errorf("Height = %d, want 480", intrinsics.Height)
	}
}

func TestCalibrationData_Struct(t *testing.T) {
	cal := CalibrationData{
		Intrinsics: CameraIntrinsics{Width: 640, Height: 480},
		Homography: NewHomography(),
		WorldScale: 100.0,
	}

	if cal.Intrinsics.Width != 640 {
		t.Errorf("Intrinsics.Width = %d, want 640", cal.Intrinsics.Width)
	}
	if cal.Homography == nil {
		t.Error("Homography should not be nil")
	}
	if cal.WorldScale != 100.0 {
		t.Errorf("WorldScale = %f, want 100.0", cal.WorldScale)
	}
}

func writeCalibrationFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "calibration.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const goodCalibration = `version: 2
camera: {name: test, resolution: [640, 480]}
homography:
  - [0.01, 0, 0]
  - [0, 0.01, 0]
  - [0, 0, 1]
world_scale: 100
`

// A calibration file with no usable homography used to be replaced with an
// identity transform, so the system reported "calibrated" and treated pixels
// as 1/100 m. It must stay uncalibrated instead.
func TestLoadCalibration_RejectsUnusableHomography(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"homography missing", "version: 2\nworld_scale: 100\n"},
		{"homography incomplete", "homography:\n  - [1, 0, 0]\n  - [0, 1]\n  - [0, 0, 1]\n"},
		{"all zeros", "homography:\n  - [0, 0, 0]\n  - [0, 0, 0]\n  - [0, 0, 0]\n"},
		{"singular", "homography:\n  - [1, 2, 3]\n  - [2, 4, 6]\n  - [0, 0, 1]\n"},
		{"not a number", "homography:\n  - [.nan, 0, 0]\n  - [0, 1, 0]\n  - [0, 0, 1]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			est, _ := NewPositionEstimator("")
			err := est.LoadCalibration(writeCalibrationFile(t, tt.body))
			if !errors.Is(err, ErrInvalidCalibration) {
				t.Fatalf("LoadCalibration = %v, want ErrInvalidCalibration", err)
			}
			if est.IsCalibrated() {
				t.Error("estimator reports calibrated after loading an unusable file")
			}
		})
	}
}

// A bad file must not disturb a calibration that is already working.
func TestLoadCalibration_BadFileKeepsPreviousCalibration(t *testing.T) {
	est, _ := NewPositionEstimator("")
	if err := est.LoadCalibration(writeCalibrationFile(t, goodCalibration)); err != nil {
		t.Fatal(err)
	}
	before := est.PixelToWorld(200, 100)

	if err := est.LoadCalibration(writeCalibrationFile(t, "version: 2\ncamera: {resolution: [1, 1]}\n")); err == nil {
		t.Fatal("expected an error for a file with no homography")
	}

	if !est.IsCalibrated() {
		t.Error("previous calibration was lost")
	}
	if after := est.PixelToWorld(200, 100); *after != *before {
		t.Errorf("mapping changed from %v to %v", *before, *after)
	}
	if w, h, ok := est.CalibratedResolution(); !ok || w != 640 || h != 480 {
		t.Errorf("calibrated resolution changed to %dx%d (ok=%v)", w, h, ok)
	}
}

func TestLoadCalibration_AcceptsIntegerWorldScale(t *testing.T) {
	est, _ := NewPositionEstimator("")
	body := "homography:\n  - [1, 0, 0]\n  - [0, 1, 0]\n  - [0, 0, 1]\nworld_scale: 250\n"
	if err := est.LoadCalibration(writeCalibrationFile(t, body)); err != nil {
		t.Fatal(err)
	}
	if got := est.GetHomography().PixelsPerMeter; got != 250 {
		t.Errorf("PixelsPerMeter = %v, want 250 (an integer world_scale was ignored)", got)
	}
}

func TestPositionEstimator_VisibleWorldCorners(t *testing.T) {
	// A scale-and-shift homography: world = pixel/100 - (1, 0.5).
	newEst := func(h2 [3]float64, w, h int) *PositionEstimator {
		est := &PositionEstimator{homography: NewHomography()}
		est.homography.SetFromValues(0.01, 0, -1, 0, 0.01, -0.5, h2[0], h2[1], h2[2])
		est.homography.ComputeInverse()
		est.SetFrameSize(w, h)
		return est
	}

	t.Run("projects the frame corners", func(t *testing.T) {
		got, ok := newEst([3]float64{0, 0, 1}, 400, 300).VisibleWorldCorners()
		want := [4][2]float64{{-1, -0.5}, {3, -0.5}, {3, 2.5}, {-1, 2.5}}
		if !ok || got != want {
			t.Errorf("got %v ok=%v, want %v", got, ok, want)
		}
	})
	t.Run("no calibration", func(t *testing.T) {
		est := &PositionEstimator{homography: NewHomography()}
		est.SetFrameSize(400, 300)
		if _, ok := est.VisibleWorldCorners(); ok {
			t.Error("ok without a calibration")
		}
	})
	t.Run("no frame yet", func(t *testing.T) {
		if _, ok := newEst([3]float64{0, 0, 1}, 0, 0).VisibleWorldCorners(); ok {
			t.Error("ok before a frame size is known")
		}
	})
	t.Run("corner beyond the horizon", func(t *testing.T) {
		// w = 1 - 0.01*y goes negative for y > 100: the bottom corners are behind the camera.
		if _, ok := newEst([3]float64{0, -0.01, 1}, 400, 300).VisibleWorldCorners(); ok {
			t.Error("ok with a corner past the horizon")
		}
	})
}
