package position

import (
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
