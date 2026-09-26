package position

import (
	"errors"
	"math"
	"math/rand"
	"path/filepath"
	"testing"
	"time"
)

// captureTarget simulates the camera seeing the target laid out width x depth,
// with optional per-tag floor offsets (metres) modelling a misplaced tag and
// pixel noise on every corner. ids selects which tags are visible.
func captureTarget(ids []int, width, depth, noisePx float64, offsets map[int]Point2D) []TargetCapture {
	rng := rand.New(rand.NewSource(7))
	caps := make([]TargetCapture, 0, len(ids))
	for _, id := range ids {
		tag, _ := targetTagByID(id)
		world := tag.WorldCorners(width, depth)
		var c TargetCapture
		c.ID = id
		for i, w := range world {
			w.X += offsets[id].X
			w.Y += offsets[id].Y
			p := projectWorldToPixel(w)
			p.X += rng.NormFloat64() * noisePx
			p.Y += rng.NormFloat64() * noisePx
			c.Corners[i] = p
		}
		caps = append(caps, c)
	}
	return caps
}

var allTargetIDs = []int{100, 101, 102, 103, 104}

func TestTargetTags_Layout(t *testing.T) {
	tags := TargetTags()
	if len(tags) != 5 {
		t.Fatalf("got %d target tags, want 5", len(tags))
	}
	labels := map[string]bool{}
	for _, tag := range tags {
		if !IsTargetTagID(tag.ID) {
			t.Errorf("tag %d outside the reserved range", tag.ID)
		}
		if labels[tag.Label] {
			t.Errorf("duplicate label %q", tag.Label)
		}
		labels[tag.Label] = true
	}
	if IsTargetTagID(1) || IsTargetTagID(105) || IsTargetTagID(99) {
		t.Error("IDs outside 100-104 must not be reserved")
	}

	// Mutating the returned slice must not change the layout.
	tags[0].ID = 999
	if TargetTags()[0].ID != TargetCenterID {
		t.Error("TargetTags must return a copy")
	}
}

func TestTargetTag_WorldCorners(t *testing.T) {
	tag, _ := targetTagByID(102) // top-right: col +1, row -1
	got := tag.WorldCorners(1.0, 0.6)
	h := TargetTagSize / 2
	want := [4]Point2D{{0.5 - h, -0.3 - h}, {0.5 + h, -0.3 - h}, {0.5 + h, -0.3 + h}, {0.5 - h, -0.3 + h}}
	for i := range want {
		if math.Abs(got[i].X-want[i].X) > 1e-12 || math.Abs(got[i].Y-want[i].Y) > 1e-12 {
			t.Errorf("corner %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestFitTarget_RecoversLayout(t *testing.T) {
	caps := captureTarget(allTargetIDs, 1.0, 0.6, 0.3, nil)
	res, err := FitTarget(caps, 1.0, 0.6)
	if err != nil {
		t.Fatalf("FitTarget failed: %v", err)
	}
	if res.Rating != RatingGood {
		t.Errorf("rating = %s (rms %.2f cm), want good", res.Rating, res.RMSCm)
	}
	if len(res.PerTag) != 5 {
		t.Errorf("got %d per-tag results, want 5", len(res.PerTag))
	}

	// A floor point that was not part of the fit maps back within a centimetre.
	w := Point2D{X: 0.2, Y: 0.15}
	got := res.Homography.PixelToWorld(projectWorldToPixel(w))
	if d := math.Hypot(got.X-w.X, got.Y-w.Y); d > 0.01 {
		t.Errorf("held-out point off by %.1f mm", d*1000)
	}
}

func TestFitTarget_Rejects(t *testing.T) {
	tests := []struct {
		name          string
		ids           []int
		width, depth  float64
		wantErr       error
		mutateCapture func([]TargetCapture) []TargetCapture
	}{
		{name: "missing center", ids: []int{101, 102, 103, 104}, width: 1, depth: 0.6, wantErr: ErrMissingTags},
		{name: "only two corners", ids: []int{100, 101, 102}, width: 1, depth: 0.6, wantErr: ErrMissingTags},
		{name: "spread too small", ids: allTargetIDs, width: 0.1, depth: 0.6, wantErr: ErrTargetSpread},
		{name: "spread too large", ids: allTargetIDs, width: 1, depth: 50, wantErr: ErrTargetSpread},
		{
			name: "unknown tag", ids: allTargetIDs, width: 1, depth: 0.6, wantErr: ErrUnknownTargetT,
			mutateCapture: func(c []TargetCapture) []TargetCapture { c[1].ID = 7; return c },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := captureTarget(tt.ids, 1, 0.6, 0, nil)
			if tt.mutateCapture != nil {
				caps = tt.mutateCapture(caps)
			}
			if _, err := FitTarget(caps, tt.width, tt.depth); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}

	t.Run("duplicate tag", func(t *testing.T) {
		caps := captureTarget(allTargetIDs, 1, 0.6, 0, nil)
		caps = append(caps, caps[0])
		if _, err := FitTarget(caps, 1, 0.6); err == nil {
			t.Error("expected an error for a duplicated tag")
		}
	})

	t.Run("minimum viable set fits", func(t *testing.T) {
		caps := captureTarget([]int{100, 101, 102, 103}, 1, 0.6, 0, nil)
		if _, err := FitTarget(caps, 1, 0.6); err != nil {
			t.Errorf("center + 3 corners should fit: %v", err)
		}
	})
}

func TestFitTarget_MisplacedTagIsIdentified(t *testing.T) {
	tests := []struct {
		name       string
		offsetCm   float64
		wantRating string
		wantErr    error
	}{
		{"1 cm off still passes", 1, "", nil},
		{"6 cm off is rejected (in-sample RMS alone would hide it)", 6, RatingPoor, ErrPoorFit},
		{"25 cm off is rejected", 25, RatingPoor, ErrPoorFit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offsets := map[int]Point2D{103: {X: tt.offsetCm / 100, Y: 0}}
			caps := captureTarget(allTargetIDs, 1.0, 0.6, 0, offsets)
			res, err := FitTarget(caps, 1.0, 0.6)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if res == nil {
				t.Fatal("result should be returned even for a poor fit")
			}
			if tt.wantRating != "" && res.Rating != tt.wantRating {
				t.Errorf("rating = %s, want %s", res.Rating, tt.wantRating)
			}

			if res.WorstTagID != 103 {
				t.Errorf("worst tag = %d, want 103 (per-tag: %+v)", res.WorstTagID, res.PerTag)
			}
		})
	}
}

func TestRateFit(t *testing.T) {
	tests := []struct {
		rms  float64
		want string
	}{{0.2, RatingGood}, {0.99, RatingGood}, {1.0, RatingOK}, {1.9, RatingOK}, {2.0, RatingPoor}, {9, RatingPoor}}
	for _, tt := range tests {
		if got := RateFit(tt.rms); got != tt.want {
			t.Errorf("RateFit(%.2f) = %s, want %s", tt.rms, got, tt.want)
		}
	}
}

func TestCalibrationFile_RoundTripAndResolutionMismatch(t *testing.T) {
	caps := captureTarget(allTargetIDs, 1.0, 0.6, 0.2, nil)
	res, err := FitTarget(caps, 1.0, 0.6)
	if err != nil {
		t.Fatalf("FitTarget failed: %v", err)
	}

	path := filepath.Join(t.TempDir(), "nested", "calibration_test.yaml")
	cfg := NewCalibrationConfig("Camera 0", 1280, 720, res, time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC))
	if cfg.CalibratedAt != "2026-09-26T12:00:00Z" {
		t.Errorf("CalibratedAt = %q", cfg.CalibratedAt)
	}
	if err := SaveCalibration(path, cfg); err != nil {
		t.Fatalf("SaveCalibration failed: %v", err)
	}

	est, err := NewPositionEstimator("", "", false, 0)
	if err != nil {
		t.Fatalf("NewPositionEstimator failed: %v", err)
	}
	if err := est.LoadCalibration(path); err != nil {
		t.Fatalf("LoadCalibration failed: %v", err)
	}
	if !est.IsCalibrated() {
		t.Fatal("estimator should be calibrated after load")
	}

	w := Point2D{X: -0.25, Y: 0.2}
	got := est.PixelToWorld(int(math.Round(projectWorldToPixel(w).X)), int(math.Round(projectWorldToPixel(w).Y)))
	if d := math.Hypot(got.X-w.X, got.Y-w.Y); d > 0.02 {
		t.Errorf("loaded calibration maps a held-out point %.1f mm off", d*1000)
	}

	gotW, gotH, ok := est.CalibratedResolution()
	if !ok || gotW != 1280 || gotH != 720 {
		t.Errorf("CalibratedResolution = %d x %d (ok=%v), want 1280 x 720", gotW, gotH, ok)
	}

	if est.ResolutionMismatch() {
		t.Error("no mismatch expected before the frame size is known")
	}
	est.SetFrameSize(1280, 720)
	if est.ResolutionMismatch() {
		t.Error("no mismatch expected at the calibrated resolution")
	}
	est.SetFrameSize(1920, 1080)
	if !est.ResolutionMismatch() {
		t.Error("mismatch expected at a different resolution")
	}
}
