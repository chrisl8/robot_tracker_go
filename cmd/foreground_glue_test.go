//go:build gocv

package main

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/config"
	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/position"
)

const (
	fgW = 640
	fgH = 360
)

// fgRig is a RobotSystem with just enough wiring to run processForeground: a
// calibrated 100 px/m floor mapping, a planner, and a foreground glue.
type fgRig struct {
	rs  *RobotSystem
	rng *rand.Rand
	now time.Time
	obj *[4]int  // x0,y0,x1,y1 of an axis-aligned bright object, or nil
	rot *rotRect // a rotated bright object, or nil
}

// rotRect is a rotated rectangle (pixel centre, half-length, half-width,
// degrees) painted into the synthetic frame.
type rotRect struct {
	cx, cy, halfLen, halfWidth, angleDeg float64
}

func newFGRig(t *testing.T, apply bool) *fgRig {
	t.Helper()
	est, err := position.NewPositionEstimator("", "", false, 0)
	if err != nil {
		t.Fatalf("NewPositionEstimator: %v", err)
	}
	est.GetHomography().SetFromValues(0.01, 0, 0, 0, 0.01, 0, 0, 0, 1) // 100 px per metre

	on := true
	cfg := &config.Config{Foreground: config.ForegroundConfig{
		WarmupSec: 1, AppearMs: 200, VanishMs: 400, ApplyToPlanner: &apply, Enabled: &on,
	}}
	rs := &RobotSystem{cfg: cfg}
	rs.planning.planner = planning.NewPlanner(nil)
	rs.position.positionEst = est
	rs.detection.fg = newForegroundGlue(cfg.EffectiveForeground())
	t.Cleanup(rs.detection.fg.det.Close)
	return &fgRig{rs: rs, rng: rand.New(rand.NewSource(1)), now: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
}

func (r *fgRig) frame() []byte {
	f := make([]byte, fgW*fgH*3)
	for i := 0; i < fgW*fgH; i++ {
		v := byte(120 + r.rng.Float64()*8 - 4)
		f[i*3], f[i*3+1], f[i*3+2] = v, v, v
	}
	if r.obj != nil {
		for y := r.obj[1]; y < r.obj[3]; y++ {
			for x := r.obj[0]; x < r.obj[2]; x++ {
				i := (y*fgW + x) * 3
				f[i], f[i+1], f[i+2] = 210, 210, 210
			}
		}
	}
	if r.rot != nil {
		rad := r.rot.angleDeg * math.Pi / 180
		c, s := math.Cos(rad), math.Sin(rad)
		for y := 0; y < fgH; y++ {
			for x := 0; x < fgW; x++ {
				dx, dy := float64(x)-r.rot.cx, float64(y)-r.rot.cy
				lx := dx*c + dy*s
				ly := -dx*s + dy*c
				if math.Abs(lx) <= r.rot.halfLen && math.Abs(ly) <= r.rot.halfWidth {
					i := (y*fgW + x) * 3
					f[i], f[i+1], f[i+2] = 210, 210, 210
				}
			}
		}
	}
	return f
}

func (r *fgRig) step(tags ...detection.AprilTag) {
	r.now = r.now.Add(70 * time.Millisecond)
	r.rs.processForeground(r.frame(), fgW, fgH, r.now, &detection.DetectionResult{Tags: tags})
}

func (r *fgRig) run(n int, tags ...detection.AprilTag) {
	for i := 0; i < n; i++ {
		r.step(tags...)
	}
}

func TestProcessForeground_PublishesAfterItPersistsAndClearsWhenRemoved(t *testing.T) {
	r := newFGRig(t, false)
	r.run(30) // warm up on an empty floor
	if got := r.rs.detection.fg.snapshotState(); got.Warming || got.Count != 0 {
		t.Fatalf("empty floor: %+v", got)
	}

	r.obj = &[4]int{300, 150, 360, 210} // 60x60 px = 0.6 x 0.6 m
	r.step()
	if got := r.rs.detection.fg.snapshotState().Count; got != 0 {
		t.Errorf("an object seen for one frame published %d obstacles; it must persist first", got)
	}
	r.run(8)
	st := r.rs.detection.fg.snapshotState()
	if st.Count != 1 {
		t.Fatalf("persisting object: %d obstacles, want 1", st.Count)
	}
	box := r.rs.detection.fg.published[0].Box
	if w, h := box.Width(), box.Height(); w < 0.5 || w > 0.8 || h < 0.5 || h > 0.8 {
		t.Errorf("obstacle is %.2f x %.2f m, want about 0.6 x 0.6", w, h)
	}

	r.obj = nil
	r.run(3)
	if got := r.rs.detection.fg.snapshotState().Count; got != 1 {
		t.Errorf("obstacle dropped after a moment (%d); it should linger for vanish_ms", got)
	}
	r.run(10)
	if got := r.rs.detection.fg.snapshotState().Count; got != 0 {
		t.Errorf("obstacle still present after it left (%d)", got)
	}
}

func TestProcessForeground_ShadowModeDoesNotSteerUntilApplied(t *testing.T) {
	r := newFGRig(t, false)
	r.rs.planning.planner.AddRobot(1, [2]float64{3.0, 1.8}, 0.30)
	r.run(30)
	r.obj = &[4]int{300, 150, 360, 210}
	r.run(10)
	if r.rs.detection.fg.snapshotState().Count != 1 {
		t.Fatal("expected the obstacle to be detected")
	}
	if c := r.rs.planning.planner.GetClearance(1); c < 1e9 {
		t.Errorf("shadow mode leaked an obstacle into the planner (clearance %.2f)", c)
	}

	r.rs.detection.fg.apply.Store(true)
	r.step()
	if c := r.rs.planning.planner.GetClearance(1); c > 1e9 {
		t.Error("with steering on, the planner should know about the obstacle")
	}

	r.rs.detection.fg.apply.Store(false)
	r.step()
	if c := r.rs.planning.planner.GetClearance(1); c < 1e9 {
		t.Errorf("turning steering off should clear the planner's obstacles (clearance %.2f)", c)
	}
}

func TestProcessForeground_RobotTagMaskHidesTheRobot(t *testing.T) {
	r := newFGRig(t, false)
	r.run(30)

	// A bright "robot" appears with its tag detected on top of it.
	r.obj = &[4]int{300, 150, 340, 190}
	tag := detection.AprilTag{TagID: 1, CenterX: 320, CenterY: 170}
	r.run(10, tag)
	if got := r.rs.detection.fg.snapshotState().Count; got != 0 {
		t.Errorf("a robot with a detected tag became %d obstacles", got)
	}

	// Its tag drops out for a moment: the last position stays masked.
	r.run(10)
	if got := r.rs.detection.fg.snapshotState().Count; got != 0 {
		t.Errorf("a robot whose tag was just lost became %d obstacles", got)
	}
}

func TestProcessForeground_CalibrationTagsAreNotRobots(t *testing.T) {
	r := newFGRig(t, false)
	r.run(30)
	r.obj = &[4]int{300, 150, 360, 210}
	// Tag 100 is a calibration tag lying on the floor, not a robot to mask.
	r.run(10, detection.AprilTag{TagID: 100, CenterX: 330, CenterY: 180})
	if got := r.rs.detection.fg.snapshotState().Count; got != 1 {
		t.Errorf("an object next to a calibration tag: %d obstacles, want 1", got)
	}
}

func TestProcessForeground_DisabledOrUncalibratedPublishesNothing(t *testing.T) {
	r := newFGRig(t, true)
	r.run(30)
	r.obj = &[4]int{300, 150, 360, 210}
	r.run(10)
	if r.rs.detection.fg.snapshotState().Count != 1 {
		t.Fatal("expected an obstacle")
	}

	r.rs.detection.fg.enabled.Store(false)
	r.step()
	if got := r.rs.detection.fg.snapshotState(); got.Count != 0 || got.Enabled {
		t.Errorf("disabled detector state: %+v", got)
	}
}

func TestProcessForeground_ResetRelearnsAndAbsorbRemoves(t *testing.T) {
	r := newFGRig(t, false)
	r.run(30)
	r.obj = &[4]int{300, 150, 360, 210}
	r.run(10)
	if r.rs.detection.fg.snapshotState().Count != 1 {
		t.Fatal("expected an obstacle")
	}

	r.rs.detection.fg.requestAbsorb(330, 180)
	r.run(6)
	if got := r.rs.detection.fg.snapshotState().Count; got != 0 {
		t.Errorf("absorbed object still an obstacle (%d)", got)
	}

	r.obj = &[4]int{100, 60, 160, 120}
	r.run(10)
	if r.rs.detection.fg.snapshotState().Count != 1 {
		t.Fatal("expected a second obstacle")
	}
	r.rs.detection.fg.requestReset()
	r.step()
	if got := r.rs.detection.fg.snapshotState(); got.Count != 0 {
		t.Errorf("after reset: %+v", got)
	}
}

// TestProcessForeground_TightOrientedObstacleReachesThePlanner is the
// end-to-end proof for phase 6: a rotated object's exact footprint (not its
// inflated AABB) is what actually blocks the planner. A point clearly outside
// the real stick, but inside its old bounding box, must stay clear once the
// obstacle's Quad is populated and steering is on.
func TestProcessForeground_TightOrientedObstacleReachesThePlanner(t *testing.T) {
	r := newFGRig(t, true) // apply = true: obstacles steer the planner
	r.run(30)              // warm up on an empty floor

	// A ~50 x 10 px stick at 45 degrees, centred at (320, 180) in pixel space,
	// i.e. (3.2, 1.8) m at the rig's 100 px/m calibration.
	r.rot = &rotRect{cx: 320, cy: 180, halfLen: 25, halfWidth: 5, angleDeg: 45}
	r.run(10)

	got := r.rs.detection.fg.snapshotState()
	if got.Count != 1 {
		t.Fatalf("expected 1 tracked obstacle, got %+v", got)
	}
	tracked := r.rs.detection.fg.published[0]

	// The published Quad must be genuinely oriented, not a degenerate
	// axis-aligned rectangle: an axis-aligned quad has only two distinct X
	// values and two distinct Y values among its four corners.
	xs, ys := map[float64]bool{}, map[float64]bool{}
	for _, p := range tracked.Quad {
		xs[math.Round(p[0]*1000)] = true
		ys[math.Round(p[1]*1000)] = true
	}
	if len(xs) <= 2 && len(ys) <= 2 {
		t.Fatalf("published Quad looks axis-aligned, want an oriented (45 degree) box: %v", tracked.Quad)
	}

	r.rs.planning.planner.AddRobot(1, [2]float64{0, 0}, 0.1)

	// A point on the stick's true diagonal axis, just past its tip: clearly
	// clear of the real 10 px-wide stick, but well inside the AABB a
	// rectangle-only fix would have used (the stick's world half-length is
	// 25 px / 100 px/m = 0.25 m from centre (3.2, 1.8), so its AABB spans
	// roughly +-0.18m on each axis around the centre once rotated 45deg).
	clearPoint := [2]float64{3.2 + 0.05, 1.8 - 0.30} // near the AABB edge, off the stick's axis
	r.rs.planning.planner.UpdateRobotState(1, clearPoint, [2]float64{0, 0})
	if c := r.rs.planning.planner.GetClearance(1); c < 0.02 {
		t.Errorf("point %v should be clear of the real stick shape, clearance = %.3fm", clearPoint, c)
	}

	// Sanity: a point actually on the stick's axis is correctly blocked.
	onStick := [2]float64{3.2, 1.8}
	r.rs.planning.planner.UpdateRobotState(1, onStick, [2]float64{0, 0})
	if c := r.rs.planning.planner.GetClearance(1); c > 0 {
		t.Errorf("point %v is on the real stick, want clearance <= 0, got %.3fm", onStick, c)
	}
}
