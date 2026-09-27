//go:build gocv

package main

import (
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
	obj *[4]int // x0,y0,x1,y1 of a bright object, or nil
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
	rs := &RobotSystem{cfg: cfg, planner: planning.NewPlanner(nil), positionEst: est}
	rs.fg = newForegroundGlue(cfg.EffectiveForeground())
	t.Cleanup(rs.fg.det.Close)
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
	if got := r.rs.fg.snapshotState(); got.Warming || got.Count != 0 {
		t.Fatalf("empty floor: %+v", got)
	}

	r.obj = &[4]int{300, 150, 360, 210} // 60x60 px = 0.6 x 0.6 m
	r.step()
	if got := r.rs.fg.snapshotState().Count; got != 0 {
		t.Errorf("an object seen for one frame published %d obstacles; it must persist first", got)
	}
	r.run(8)
	st := r.rs.fg.snapshotState()
	if st.Count != 1 {
		t.Fatalf("persisting object: %d obstacles, want 1", st.Count)
	}
	box := r.rs.fg.published[0].Box
	if w, h := box.Width(), box.Height(); w < 0.5 || w > 0.8 || h < 0.5 || h > 0.8 {
		t.Errorf("obstacle is %.2f x %.2f m, want about 0.6 x 0.6", w, h)
	}

	r.obj = nil
	r.run(3)
	if got := r.rs.fg.snapshotState().Count; got != 1 {
		t.Errorf("obstacle dropped after a moment (%d); it should linger for vanish_ms", got)
	}
	r.run(10)
	if got := r.rs.fg.snapshotState().Count; got != 0 {
		t.Errorf("obstacle still present after it left (%d)", got)
	}
}

func TestProcessForeground_ShadowModeDoesNotSteerUntilApplied(t *testing.T) {
	r := newFGRig(t, false)
	r.rs.planner.AddRobot(1, [2]float64{3.0, 1.8}, 0.30)
	r.run(30)
	r.obj = &[4]int{300, 150, 360, 210}
	r.run(10)
	if r.rs.fg.snapshotState().Count != 1 {
		t.Fatal("expected the obstacle to be detected")
	}
	if c := r.rs.planner.GetClearance(1); c < 1e9 {
		t.Errorf("shadow mode leaked an obstacle into the planner (clearance %.2f)", c)
	}

	r.rs.fg.apply.Store(true)
	r.step()
	if c := r.rs.planner.GetClearance(1); c > 1e9 {
		t.Error("with steering on, the planner should know about the obstacle")
	}

	r.rs.fg.apply.Store(false)
	r.step()
	if c := r.rs.planner.GetClearance(1); c < 1e9 {
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
	if got := r.rs.fg.snapshotState().Count; got != 0 {
		t.Errorf("a robot with a detected tag became %d obstacles", got)
	}

	// Its tag drops out for a moment: the last position stays masked.
	r.run(10)
	if got := r.rs.fg.snapshotState().Count; got != 0 {
		t.Errorf("a robot whose tag was just lost became %d obstacles", got)
	}
}

func TestProcessForeground_CalibrationTagsAreNotRobots(t *testing.T) {
	r := newFGRig(t, false)
	r.run(30)
	r.obj = &[4]int{300, 150, 360, 210}
	// Tag 100 is a calibration tag lying on the floor, not a robot to mask.
	r.run(10, detection.AprilTag{TagID: 100, CenterX: 330, CenterY: 180})
	if got := r.rs.fg.snapshotState().Count; got != 1 {
		t.Errorf("an object next to a calibration tag: %d obstacles, want 1", got)
	}
}

func TestProcessForeground_DisabledOrUncalibratedPublishesNothing(t *testing.T) {
	r := newFGRig(t, true)
	r.run(30)
	r.obj = &[4]int{300, 150, 360, 210}
	r.run(10)
	if r.rs.fg.snapshotState().Count != 1 {
		t.Fatal("expected an obstacle")
	}

	r.rs.fg.enabled.Store(false)
	r.step()
	if got := r.rs.fg.snapshotState(); got.Count != 0 || got.Enabled {
		t.Errorf("disabled detector state: %+v", got)
	}
}

func TestProcessForeground_ResetRelearnsAndAbsorbRemoves(t *testing.T) {
	r := newFGRig(t, false)
	r.run(30)
	r.obj = &[4]int{300, 150, 360, 210}
	r.run(10)
	if r.rs.fg.snapshotState().Count != 1 {
		t.Fatal("expected an obstacle")
	}

	r.rs.fg.requestAbsorb(330, 180)
	r.run(6)
	if got := r.rs.fg.snapshotState().Count; got != 0 {
		t.Errorf("absorbed object still an obstacle (%d)", got)
	}

	r.obj = &[4]int{100, 60, 160, 120}
	r.run(10)
	if r.rs.fg.snapshotState().Count != 1 {
		t.Fatal("expected a second obstacle")
	}
	r.rs.fg.requestReset()
	r.step()
	if got := r.rs.fg.snapshotState(); got.Count != 0 {
		t.Errorf("after reset: %+v", got)
	}
}
