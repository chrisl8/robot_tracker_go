package position

import (
	"testing"
)

func TestNewPositionEstimator(t *testing.T) {
	est, err := NewPositionEstimator("", "", false, 0.0)
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
	if len(est.obstacles) != 0 {
		t.Error("obstacles should be empty")
	}
	if est.smoothing {
		t.Error("smoothing should be false")
	}
}

func TestPositionEstimator_PixelToWorld_Invalid(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)
	result := est.PixelToWorld(100, 200)

	if result.X != 100 || result.Y != 200 {
		t.Errorf("Invalid estimator should return original pixel coordinates, got (%f, %f)", result.X, result.Y)
	}
}

func TestPositionEstimator_PixelToWorld_Valid(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)
	est.homography.SetFromValues(1, 0, 0, 0, 1, 0, 0, 0, 1)

	result := est.PixelToWorld(100, 200)

	if result.X != 100 || result.Y != 200 {
		t.Errorf("Identity transform should preserve coordinates, got (%f, %f)", result.X, result.Y)
	}
}

func TestPositionEstimator_WorldToPixel_Invalid(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)
	x, y := est.WorldToPixel(Point2D{X: 1.5, Y: 2.5})

	if x != 1 || y != 2 {
		t.Errorf("Invalid estimator should return floored world coordinates, got (%d, %d)", x, y)
	}
}

func TestPositionEstimator_WorldToPixel_Valid(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)
	est.homography.SetFromValues(1, 0, 0, 0, 1, 0, 0, 0, 1)

	x, y := est.WorldToPixel(Point2D{X: 100.7, Y: 200.9})

	if x != 100 || y != 200 {
		t.Errorf("Identity transform should floor coordinates, got (%d, %d)", x, y)
	}
}

func TestPositionEstimator_UpdatePosition_NoSmoothing(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)

	est.UpdatePosition(1, 100, 200)
	pos := est.GetPosition(1)

	if pos == nil {
		t.Fatal("GetPosition returned nil")
	}
	if pos.X != 100 || pos.Y != 200 {
		t.Errorf("Position = (%f, %f), want (100, 200)", pos.X, pos.Y)
	}
}

func TestPositionEstimator_UpdatePosition_WithSmoothing(t *testing.T) {
	est, _ := NewPositionEstimator("", "", true, 0.5)

	est.UpdatePosition(1, 100, 200)
	pos1 := est.GetPosition(1)

	est.UpdatePosition(1, 100, 200)
	pos2 := est.GetPosition(1)

	if pos1 == nil || pos2 == nil {
		t.Fatal("GetPosition returned nil")
	}
	if pos1.X != 100 || pos1.Y != 200 {
		t.Errorf("First position = (%f, %f), want (100, 200)", pos1.X, pos1.Y)
	}
	if pos2.X != 100 || pos2.Y != 200 {
		t.Errorf("Second position = (%f, %f), want (100, 200)", pos2.X, pos2.Y)
	}
}

func TestPositionEstimator_UpdatePosition_SmoothingEffect(t *testing.T) {
	est, _ := NewPositionEstimator("", "", true, 0.5)

	est.UpdatePosition(1, 0, 0)
	pos := est.GetPosition(1)
	if pos == nil {
		t.Fatal("GetPosition returned nil")
	}
	if pos.X != 0 || pos.Y != 0 {
		t.Errorf("Initial position = (%f, %f), want (0, 0)", pos.X, pos.Y)
	}

	est.UpdatePosition(1, 100, 100)
	pos = est.GetPosition(1)
	if pos == nil {
		t.Fatal("GetPosition returned nil")
	}
	if pos.X != 50 || pos.Y != 50 {
		t.Errorf("After smoothing position = (%f, %f), want (50, 50)", pos.X, pos.Y)
	}
}

func TestPositionEstimator_GetPosition_UnknownTrackID(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)

	pos := est.GetPosition(999)
	if pos != nil {
		t.Errorf("GetPosition for unknown track should return nil, got (%f, %f)", pos.X, pos.Y)
	}
}

func TestPositionEstimator_GetObstacles_Empty(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)

	obstacles := est.GetObstacles()
	if len(obstacles) != 0 {
		t.Errorf("GetObstacles on empty estimator = %d, want 0", len(obstacles))
	}
}

func TestPositionEstimator_AddObstacle(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)

	obstacle := Obstacle{
		Name:             "test_obstacle",
		WorldTopLeft:     Point2D{X: 0, Y: 0},
		WorldBottomRight: Point2D{X: 1, Y: 1},
	}

	est.AddObstacle(obstacle)
	obstacles := est.GetObstacles()

	if len(obstacles) != 1 {
		t.Errorf("After AddObstacle, GetObstacles = %d, want 1", len(obstacles))
	}
	if obstacles[0].Name != "test_obstacle" {
		t.Errorf("Obstacle name = %s, want test_obstacle", obstacles[0].Name)
	}
}

func TestPositionEstimator_ClearObstacles(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)

	est.AddObstacle(Obstacle{Name: "obs1"})
	est.AddObstacle(Obstacle{Name: "obs2"})
	if len(est.GetObstacles()) != 2 {
		t.Error("Expected 2 obstacles after adding")
	}

	est.ClearObstacles()
	if len(est.GetObstacles()) != 0 {
		t.Errorf("After ClearObstacles, GetObstacles = %d, want 0", len(est.GetObstacles()))
	}
}

func TestPositionEstimator_GetHomography(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)
	h := est.GetHomography()

	if h == nil {
		t.Error("GetHomography should not return nil")
	}
}

func TestPositionEstimator_IsCalibrated_NotCalibrated(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)

	if est.IsCalibrated() {
		t.Error("Estimator without valid homography should not be calibrated")
	}
}

func TestPositionEstimator_IsCalibrated_Calibrated(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)
	est.homography.SetFromValues(1, 0, 0, 0, 1, 0, 0, 0, 1)

	if !est.IsCalibrated() {
		t.Error("Estimator with valid homography should be calibrated")
	}
}

func TestPositionEstimator_MultipleTrackIDs(t *testing.T) {
	est, _ := NewPositionEstimator("", "", false, 0.0)

	est.UpdatePosition(1, 100, 200)
	est.UpdatePosition(2, 300, 400)
	est.UpdatePosition(3, 500, 600)

	pos1 := est.GetPosition(1)
	pos2 := est.GetPosition(2)
	pos3 := est.GetPosition(3)

	if pos1 == nil {
		t.Errorf("Track 1 position is nil")
	} else if pos1.X != 100 || pos1.Y != 200 {
		t.Errorf("Track 1 position = (%f, %f), want (100, 200)", pos1.X, pos1.Y)
	}
	if pos2 == nil {
		t.Errorf("Track 2 position is nil")
	} else if pos2.X != 300 || pos2.Y != 400 {
		t.Errorf("Track 2 position = (%f, %f), want (300, 400)", pos2.X, pos2.Y)
	}
	if pos3 == nil {
		t.Errorf("Track 3 position is nil")
	} else if pos3.X != 500 || pos3.Y != 600 {
		t.Errorf("Track 3 position = (%f, %f), want (500, 600)", pos3.X, pos3.Y)
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

func TestSmoothedPosition_Struct(t *testing.T) {
	sp := SmoothedPosition{
		X:       100.5,
		Y:       200.7,
		Updated: true,
	}

	if sp.X != 100.5 || sp.Y != 200.7 {
		t.Errorf("SmoothedPosition = (%f, %f), want (100.5, 200.7)", sp.X, sp.Y)
	}
	if !sp.Updated {
		t.Error("Updated should be true")
	}
}
