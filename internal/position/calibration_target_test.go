package position

import (
	"errors"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// pinholeWorldToPixel builds the floor-to-pixel homography of a pinhole camera
// (focal length f px, 1280x720 frame) at camY metres along +y from the floor
// origin and camZ metres up, looking at the origin.
func pinholeWorldToPixel(camY, camZ, f float64) [3][3]float64 {
	c := [3]float64{0, camY, camZ}
	z := normalize3([3]float64{-c[0], -c[1], -c[2]})
	x := normalize3(cross3(z, [3]float64{0, -1, 0}))
	y := cross3(z, x)
	r := [3][3]float64{x, y, z}
	tvec := [3]float64{}
	for i := 0; i < 3; i++ {
		tvec[i] = -(r[i][0]*c[0] + r[i][1]*c[1] + r[i][2]*c[2])
	}
	cols := [3][3]float64{{r[0][0], r[0][1], tvec[0]}, {r[1][0], r[1][1], tvec[1]}, {r[2][0], r[2][1], tvec[2]}}
	k := [3][3]float64{{f, 0, 640}, {0, f, 360}, {0, 0, 1}}
	return mul3(k, cols)
}

func cross3(a, b [3]float64) [3]float64 {
	return [3]float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

func normalize3(v [3]float64) [3]float64 {
	n := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	return [3]float64{v[0] / n, v[1] / n, v[2] / n}
}

// scatteredTag is where a tag REALLY lies on the floor, in the Center tag's frame.
type scatteredTag struct {
	id             int
	x, y, thetaRad float64
	sizeScale      float64 // 1 = printed at the right size
}

// dropTags simulates someone placing the tags roughly near the nominal spots
// (+-1 stumble radius in metres, any rotation up to rotDeg), then the camera
// seeing them with pixel noise.
func dropTags(rng *rand.Rand, hwp [3][3]float64, stumbleM, rotDeg, noisePx float64) ([]TargetCapture, []scatteredTag) {
	nominal := map[int][2]float64{100: {0, 0}, 101: {-0.55, -0.32}, 102: {0.55, -0.32}, 103: {0.55, 0.32}, 104: {-0.55, 0.32}}
	caps := make([]TargetCapture, 0, 5)
	truth := make([]scatteredTag, 0, 5)
	for _, id := range []int{100, 101, 102, 103, 104} {
		t := scatteredTag{id: id, sizeScale: 1}
		if id != 100 {
			t.x = nominal[id][0] + rng.NormFloat64()*stumbleM
			t.y = nominal[id][1] + rng.NormFloat64()*stumbleM
			t.thetaRad = (rng.Float64()*2 - 1) * rotDeg * math.Pi / 180
		}
		truth = append(truth, t)
		caps = append(caps, captureOf(rng, hwp, t, noisePx))
	}
	return caps, truth
}

func captureOf(rng *rand.Rand, hwp [3][3]float64, t scatteredTag, noisePx float64) TargetCapture {
	local := localTagCorners()
	for i := range local {
		local[i].X *= t.sizeScale
		local[i].Y *= t.sizeScale
	}
	var c TargetCapture
	c.ID = t.id
	for i, w := range poseCorners(tagPose{X: t.x, Y: t.y, Theta: t.thetaRad}, local) {
		p := applyH(hwp, w)
		c.Corners[i] = Point2D{X: p.X + rng.NormFloat64()*noisePx, Y: p.Y + rng.NormFloat64()*noisePx}
	}
	return c
}

// floorErrorCm measures, over a grid of floor points inside the tag spread,
// how far the fitted pixel->floor mapping is from the truth.
func floorErrorCm(h *Homography, hwp [3][3]float64) (rms, maxErr float64) {
	sumSq, n := 0.0, 0
	for gx := -0.55; gx <= 0.551; gx += 0.1 {
		for gy := -0.32; gy <= 0.321; gy += 0.08 {
			pix := applyH(hwp, Point2D{X: gx, Y: gy})
			got := h.PixelToWorld(Point2D{X: pix.X, Y: pix.Y})
			e := math.Hypot(got.X-gx, got.Y-gy) * 100
			sumSq += e * e
			maxErr = math.Max(maxErr, e)
			n++
		}
	}
	return math.Sqrt(sumSq / float64(n)), maxErr
}

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
		if tag.GuideX <= 0 || tag.GuideX >= 1 || tag.GuideY <= 0 || tag.GuideY >= 1 {
			t.Errorf("tag %d guide (%.2f, %.2f) must be inside the frame", tag.ID, tag.GuideX, tag.GuideY)
		}
	}
	if IsTargetTagID(1) || IsTargetTagID(105) || IsTargetTagID(99) {
		t.Error("IDs outside 100-104 must not be reserved")
	}

	tags[0].ID = 999
	if TargetTags()[0].ID != TargetCenterID {
		t.Error("TargetTags must return a copy")
	}
}

// TestFitTarget_ToleratesSloppyPlacement is the point of the design: nobody
// measures, so the tags are dropped roughly near the boxes at any rotation and
// the fit must still recover the floor from their printed size alone.
func TestFitTarget_ToleratesSloppyPlacement(t *testing.T) {
	cameras := []struct {
		name          string
		camY, camZ, f float64
	}{
		{"oblique wall-mounted view", 1.3, 1.2, 850},
		{"steeper view", 0.6, 1.6, 850},
		{"nearly overhead", 0.05, 1.8, 850},
	}
	placements := []struct {
		name               string
		stumbleM, rotDeg   float64
		maxRMSCm, maxMaxCm float64
	}{
		{"careful (3 cm, 8 deg)", 0.03, 8, 0.5, 1.5},
		{"sloppy (10 cm, 45 deg)", 0.10, 45, 0.5, 1.5},
		{"stumbling drunk (25 cm, any angle)", 0.25, 180, 0.6, 1.8},
	}

	for _, cam := range cameras {
		for _, pl := range placements {
			t.Run(cam.name+"/"+pl.name, func(t *testing.T) {
				hwp := pinholeWorldToPixel(cam.camY, cam.camZ, cam.f)
				worstRMS, worstMax := 0.0, 0.0
				for seed := int64(1); seed <= 12; seed++ {
					rng := rand.New(rand.NewSource(seed))
					caps, _ := dropTags(rng, hwp, pl.stumbleM, pl.rotDeg, 0.3)
					res, err := FitTarget(caps)
					if err != nil {
						t.Fatalf("seed %d: FitTarget failed: %v", seed, err)
					}
					if res.Rating != RatingGood {
						t.Errorf("seed %d: rating %s (rms %.2f cm), want good", seed, res.Rating, res.RMSCm)
					}
					rms, maxErr := floorErrorCm(res.Homography, hwp)
					worstRMS = math.Max(worstRMS, rms)
					worstMax = math.Max(worstMax, maxErr)
				}
				if worstRMS > pl.maxRMSCm || worstMax > pl.maxMaxCm {
					t.Errorf("floor error rms %.2f cm (limit %.2f), max %.2f cm (limit %.2f)",
						worstRMS, pl.maxRMSCm, worstMax, pl.maxMaxCm)
				}
			})
		}
	}
}

func TestFitTarget_Rejects(t *testing.T) {
	hwp := pinholeWorldToPixel(1.3, 1.2, 850)
	full := func() []TargetCapture {
		caps, _ := dropTags(rand.New(rand.NewSource(1)), hwp, 0.05, 20, 0)
		return caps
	}
	without := func(ids ...int) []TargetCapture {
		var out []TargetCapture
	outer:
		for _, c := range full() {
			for _, id := range ids {
				if c.ID == id {
					continue outer
				}
			}
			out = append(out, c)
		}
		return out
	}

	tests := []struct {
		name    string
		caps    []TargetCapture
		wantErr error
	}{
		{"missing center", without(100), ErrMissingTags},
		{"only two corners", without(103, 104), ErrMissingTags},
		{"unknown tag", func() []TargetCapture { c := full(); c[1].ID = 7; return c }(), ErrUnknownTargetT},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := FitTarget(tt.caps); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}

	t.Run("duplicate tag", func(t *testing.T) {
		c := full()
		if _, err := FitTarget(append(c, c[0])); err == nil {
			t.Error("expected an error for a duplicated tag")
		}
	})

	t.Run("center plus three corners is enough", func(t *testing.T) {
		if _, err := FitTarget(without(104)); err != nil {
			t.Errorf("center + 3 corners should fit: %v", err)
		}
	})
}

// A tag printed at the wrong size cannot be explained by any floor mapping,
// so it must show up as the worst-fitting tag.
func TestFitTarget_FlagsWrongSizeTag(t *testing.T) {
	hwp := pinholeWorldToPixel(1.3, 1.2, 850)
	rng := rand.New(rand.NewSource(5))
	_, truth := dropTags(rng, hwp, 0.05, 30, 0)

	caps := make([]TargetCapture, 0, len(truth))
	for _, tg := range truth {
		if tg.id == 103 {
			tg.sizeScale = 0.75 // printed at ~11 cm instead of 15 cm
		}
		caps = append(caps, captureOf(rng, hwp, tg, 0.2))
	}

	res, err := FitTarget(caps)
	if err != nil && !errors.Is(err, ErrPoorFit) {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("a result should be returned so the UI can name the bad tag")
	}
	if res.WorstTagID != 103 {
		t.Errorf("worst tag = %d, want 103 (per tag: %+v)", res.WorstTagID, res.PerTag)
	}
	if res.Rating == RatingGood {
		t.Errorf("a badly scaled tag should not rate good (rms %.2f cm)", res.RMSCm)
	}
}

func TestFitTarget_DistanceChecksMatchTruth(t *testing.T) {
	hwp := pinholeWorldToPixel(1.3, 1.2, 850)
	rng := rand.New(rand.NewSource(9))
	caps, truth := dropTags(rng, hwp, 0.15, 90, 0.3)
	res, err := FitTarget(caps)
	if err != nil {
		t.Fatalf("FitTarget failed: %v", err)
	}
	pos := map[int]scatteredTag{}
	for _, tg := range truth {
		pos[tg.id] = tg
	}
	if len(res.Checks) != len(checkPairs) {
		t.Fatalf("got %d checks, want %d", len(res.Checks), len(checkPairs))
	}
	for _, c := range res.Checks {
		a, b := pos[c.FromID], pos[c.ToID]
		want := math.Hypot(a.x-b.x, a.y-b.y)
		if math.Abs(c.Meters-want) > 0.01 {
			t.Errorf("%d->%d = %.3f m, truth %.3f m", c.FromID, c.ToID, c.Meters, want)
		}
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
	hwp := pinholeWorldToPixel(1.3, 1.2, 850)
	caps, _ := dropTags(rand.New(rand.NewSource(2)), hwp, 0.1, 40, 0.2)
	res, err := FitTarget(caps)
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
	pix := applyH(hwp, w)
	got := est.PixelToWorld(int(math.Round(pix.X)), int(math.Round(pix.Y)))
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

func TestSaveCalibration_BacksUpTheReplacedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "calibration_test.yaml")
	base := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	mk := func(version int) *CalibrationConfig {
		return &CalibrationConfig{Version: version, Camera: CameraInfo{Name: "Camera 0", Resolution: [2]int{1280, 720}}}
	}
	readVersion := func(p string) int {
		t.Helper()
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("reading %s: %v", p, err)
		}
		var c CalibrationConfig
		if err := yaml.Unmarshal(data, &c); err != nil {
			t.Fatalf("parsing %s: %v", p, err)
		}
		return c.Version
	}

	// First save: nothing to back up.
	if err := saveCalibrationAt(path, mk(1), base); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if matches, _ := filepath.Glob(path + ".bak-*"); len(matches) != 0 {
		t.Fatalf("first save should not create a backup, got %v", matches)
	}

	// Second save: the first version is preserved.
	if err := saveCalibrationAt(path, mk(2), base.Add(time.Minute)); err != nil {
		t.Fatalf("second save: %v", err)
	}
	backup := path + ".bak-20260926-120100"
	if got := readVersion(backup); got != 1 {
		t.Errorf("backup holds version %d, want 1", got)
	}
	if got := readVersion(path); got != 2 {
		t.Errorf("current file holds version %d, want 2", got)
	}

	// Many more saves: only the newest calibrationBackupsToKeep backups remain.
	for i := 3; i <= 12; i++ {
		if err := saveCalibrationAt(path, mk(i), base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}
	matches, _ := filepath.Glob(path + ".bak-*")
	if len(matches) != calibrationBackupsToKeep {
		t.Fatalf("kept %d backups, want %d: %v", len(matches), calibrationBackupsToKeep, matches)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Error("the oldest backup should have been pruned")
	}
	// The newest backup is the version just before the last save (11).
	if got := readVersion(path + ".bak-20260926-121200"); got != 11 {
		t.Errorf("newest backup holds version %d, want 11", got)
	}
}
