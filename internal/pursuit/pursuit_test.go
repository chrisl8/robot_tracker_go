package pursuit

import (
	"math"
	"testing"
)

// fakeWorld: straight lines are allowed unless they would end beyond blockX.
type fakeWorld struct{ blockX float64 }

func (f fakeWorld) CanDriveStraight(_, b [2]float64) bool {
	return f.blockX == 0 || b[0] <= f.blockX
}

func newTracker(path [][2]float64) *Tracker {
	t := New(Params{Lookahead: 0.3, ArrivalRadius: 0.06, BendDeg: 15}, fakeWorld{})
	t.SetPath(path)
	return t
}

// following is a tracker for a robot that has already walked progress metres
// along path (a tracker only searches near where it was, so a test cannot just
// teleport the robot into the middle of a path).
func following(path [][2]float64, progress float64) *Tracker {
	t := newTracker(path)
	t.s = progress
	return t
}

func near(a, b [2]float64) bool { return math.Hypot(a[0]-b[0], a[1]-b[1]) < 1e-6 }

var lPath = [][2]float64{{0, 0}, {2, 0}, {2, 2}}

func TestTarget(t *testing.T) {
	tests := []struct {
		name string
		path [][2]float64
		prog float64 // metres the robot has already followed the path
		pos  [2]float64
		want [2]float64
	}{
		{"straight leg: the carrot leads by the look-ahead", lPath, 1, [2]float64{1, 0}, [2]float64{1.3, 0}},
		{"a robot off the line is aimed at the line ahead of it", lPath, 1, [2]float64{1, 0.1}, [2]float64{1.3, 0}},
		{"the carrot waits at a bend rather than rounding it", lPath, 1.8, [2]float64{1.8, 0}, [2]float64{2, 0}},
		{"arriving within the radius (0.06 m) releases the carrot round the bend", lPath, 1.95, [2]float64{1.96, 0}, [2]float64{2, 0.26}},
		{"close but not within the radius, the carrot still waits", lPath, 1.92, [2]float64{1.92, 0}, [2]float64{2, 0}},
		{"a robot that cut the corner is past the bend, so the carrot goes on", lPath, 1.85, [2]float64{1.85, 0.3}, [2]float64{2, 0.6}},
		{"the carrot stops at the goal", lPath, 3.9, [2]float64{2, 1.9}, [2]float64{2, 2}},
		{"a straight path has no bends to wait at", [][2]float64{{0, 0}, {1, 0}, {2, 0}}, 0.9, [2]float64{0.9, 0}, [2]float64{1.2, 0}},
		{"a gentle turn (under the bend angle) is not waited at", [][2]float64{{0, 0}, {1, 0}, {2, 0.1}}, 0.9, [2]float64{0.9, 0}, [2]float64{1.2, 0.02}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := following(tt.path, tt.prog)
			got, ok := tr.Target(tt.pos)
			if !ok || math.Hypot(got[0]-tt.want[0], got[1]-tt.want[1]) > 0.02 {
				t.Errorf("Target(%v) = %v (ok=%v), want %v", tt.pos, got, ok, tt.want)
			}
		})
	}
}

// A detour round a block passes close to itself. Progress must not jump to the
// return leg just because the robot is nearer to it.
func TestProgressDoesNotSnapToAnotherLeg(t *testing.T) {
	u := [][2]float64{{0, 0}, {2, 0}, {2, 0.3}, {0, 0.3}}
	tr := following(u, 1.0)
	tr.Target([2]float64{1.0, 0})
	tr.Target([2]float64{1.0, 0.2}) // 0.1 from the return leg, 0.2 from the one it is on
	if p := tr.Progress(); math.Abs(p-1.0) > 0.25 {
		t.Errorf("progress jumped to %.2f m: the robot was snapped to the return leg", p)
	}
}

func TestProgressMayStepBackOnlyALittle(t *testing.T) {
	tr := following([][2]float64{{0, 0}, {3, 0}}, 2)
	tr.Target([2]float64{2, 0})
	tr.Target([2]float64{1.9, 0})
	if p := tr.Progress(); math.Abs(p-1.9) > 1e-6 {
		t.Errorf("a robot that backed up 10 cm should read 1.9, got %.3f", p)
	}
	tr.Target([2]float64{0.5, 0})
	if p := tr.Progress(); p < 1.9-searchBack-1e-6 {
		t.Errorf("progress fell to %.3f in one frame, more than the %.2f m allowed", p, searchBack)
	}
}

func TestCarrotIsPulledBackWhenTheWayIsNotClear(t *testing.T) {
	tr := New(Params{Lookahead: 0.3, ArrivalRadius: 0.06, BendDeg: 15}, fakeWorld{blockX: 1.15})
	tr.SetPath([][2]float64{{0, 0}, {3, 0}})
	tr.s = 1
	got, _ := tr.Target([2]float64{1, 0})
	if got[0] > 1.15+1e-9 || got[0] < 1.0 {
		t.Errorf("carrot at x=%.3f, want it pulled back to at most 1.15", got[0])
	}
}

func TestSetPathStartsOver(t *testing.T) {
	tr := following(lPath, 1.9)
	tr.Target([2]float64{1.99, 0}) // clears the bend, progress ~2
	tr.SetPath(lPath)
	if tr.Progress() != 0 {
		t.Errorf("progress %.2f after SetPath, want 0", tr.Progress())
	}
	tr.s = 1.8
	if got, _ := tr.Target([2]float64{1.8, 0}); !near(got, [2]float64{2, 0}) {
		t.Errorf("the new path's bend was not waited at: carrot %v", got)
	}
}

func TestNoPath(t *testing.T) {
	tr := New(Params{Lookahead: 0.3}, fakeWorld{})
	if _, ok := tr.Target([2]float64{0, 0}); ok {
		t.Error("ok with no path")
	}
	tr.SetPath([][2]float64{{1, 1}})
	if got, ok := tr.Target([2]float64{0, 0}); !ok || !near(got, [2]float64{1, 1}) {
		t.Errorf("single-point path: %v ok=%v", got, ok)
	}
	if tr.Length() != 0 {
		t.Errorf("length %v", tr.Length())
	}
}

func TestDefaultParamsScaleWithTheRobot(t *testing.T) {
	small, big := DefaultParams(0.15), DefaultParams(0.30)
	if big.Lookahead != 2*small.Lookahead || math.Abs(big.ArrivalRadius-2*small.ArrivalRadius) > 1e-9 {
		t.Errorf("look-ahead and arrival radius should scale with diameter: %+v vs %+v", small, big)
	}
}

func TestNextVertex(t *testing.T) {
	tests := []struct {
		progress float64
		want     int
	}{{0, 1}, {1.99, 1}, {2.0, 2}, {2.5, 2}, {4.0, 2}}
	for _, tt := range tests {
		if got := following(lPath, tt.progress).NextVertex(); got != tt.want {
			t.Errorf("progress %.2f: next vertex %d, want %d", tt.progress, got, tt.want)
		}
	}
	if got := New(Params{}, fakeWorld{}).NextVertex(); got != 0 {
		t.Errorf("no path: %d", got)
	}
}

func TestSamePath(t *testing.T) {
	a := [][2]float64{{0, 0}, {1, 1}}
	for _, tt := range []struct {
		name string
		b    [][2]float64
		want bool
	}{
		{"identical", [][2]float64{{0, 0}, {1, 1}}, true},
		{"a point differs", [][2]float64{{0, 0}, {1, 1.0001}}, false},
		{"longer", [][2]float64{{0, 0}, {1, 1}, {2, 2}}, false},
		{"empty", nil, false},
	} {
		if got := SamePath(a, tt.b); got != tt.want {
			t.Errorf("%s: SamePath = %v, want %v", tt.name, got, tt.want)
		}
	}
	if !SamePath(nil, [][2]float64{}) {
		t.Error("two empty paths differ")
	}
}
