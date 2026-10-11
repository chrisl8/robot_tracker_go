package planning

import (
	"math"
	"testing"
)

// tightRoom is the camera view that motivated soft clearances: a 3.0 x 1.7 m
// floor with a long bar across it leaving ~0.5 m gaps at both ends. A 0.30 m
// robot fits easily, but the full radius+0.12 m margin on the bar plus the same
// margin on the frame edge left no way past.
func tightRoom(t *testing.T) (*Planner, [2]float64, [2]float64, Obstacle) {
	t.Helper()
	p := NewPlanner(nil)
	start, goal := [2]float64{1.4, 1.5}, [2]float64{1.4, 0.2}
	p.AddRobot(1, start, 0.30)
	p.SetArea(mustArea(t, [4][2]float64{{0, 0}, {3, 0}, {3, 1.7}, {0, 1.7}}))
	bar := NewRectObstacle("bar", [2]float64{0.36, 0.6}, [2]float64{2.66, 0.78})
	p.SetObstacles([]Obstacle{bar})
	return p, start, goal, bar
}

func TestPlanPath_SqueezesThroughGapTighterThanSoftMargin(t *testing.T) {
	p, start, goal, bar := tightRoom(t)

	// Under the old single margin there is no way through.
	if _, ok := p.globalPlanner.PlanWithin(start, goal, []Obstacle{bar}, p.marginLocked(1), p.area); ok {
		t.Fatal("test setup: expected no path with one hard radius+0.12 margin")
	}

	path, ok := p.PlanPath(1, start, goal)
	if !ok {
		t.Fatal("PlanPath found no way past the bar, though the gap fits the robot")
	}
	opts := p.planOptionsLocked(1)
	if c := minClearance(path, []Obstacle{bar}); c < opts.Hard-0.02 {
		t.Errorf("path clearance %.3f m is below the hard limit %.3f m: %v", c, opts.Hard, path)
	}
	for _, wp := range path {
		if !p.area.Contains(wp, areaEdgeMargin-0.02) {
			t.Errorf("waypoint %v leaves the camera area", wp)
		}
	}
}

// Only the left gap (0.36 m) is wide enough; the right one (0.34 m) is not,
// so the path has to go round the left end of the bar.
func TestPlanPath_TakesTheGapThatFits(t *testing.T) {
	p, start, goal, _ := tightRoom(t)
	path, ok := p.PlanPath(1, start, goal)
	if !ok {
		t.Fatal("no path")
	}
	passedLeft := false
	for _, wp := range path {
		if wp[1] > 0.55 && wp[1] < 0.85 && wp[0] < 0.36 {
			passedLeft = true
		}
	}
	if !passedLeft {
		t.Errorf("expected the path to pass the left end of the bar: %v", path)
	}
}

// With plenty of room the soft margin must still be honoured: the cost
// penalty exists to keep paths away from obstacles, not just to allow squeezes.
func TestPlanPath_KeepsSoftMarginWhenThereIsRoom(t *testing.T) {
	p := NewPlanner(nil)
	start, goal := [2]float64{-1, 0}, [2]float64{1, 0}
	p.AddRobot(1, start, 0.30)
	box := NewRectObstacle("box", [2]float64{-0.2, -0.2}, [2]float64{0.2, 0.05})
	p.SetObstacles([]Obstacle{box})
	path, ok := p.PlanPath(1, start, goal)
	if !ok {
		t.Fatal("no path")
	}
	soft := p.planOptionsLocked(1).Soft
	if c := minClearance(path, []Obstacle{box}); c < soft-0.03 {
		t.Errorf("clearance %.3f m, want at least the soft margin %.3f m: %v", c, soft, path)
	}
}

func TestAStar_PlanWithOptions_HardLimitStillBlocks(t *testing.T) {
	a := areaAStar()
	area := mustArea(t, [4][2]float64{{-5, -5}, {5, -5}, {5, 5}, {-5, 5}})
	// A wall across the whole area with a 0.30 m slot in it.
	wall := []Obstacle{
		NewRectObstacle("lower", [2]float64{-0.2, -9}, [2]float64{0.2, -0.15}),
		NewRectObstacle("upper", [2]float64{-0.2, 0.15}, [2]float64{0.2, 9}),
	}
	start, goal := [2]float64{-1, 0}, [2]float64{1, 0}

	// A robot needing 0.22 m each side cannot use a 0.15 m half-slot...
	if _, ok := a.PlanWithOptions(start, goal, wall, PlanOptions{Hard: 0.22, Soft: 0.27, Edge: 0.1}, area); ok {
		t.Error("a 0.30 m slot must stay blocked at a 0.22 m hard limit")
	}
	// ...but one needing 0.10 m can.
	if _, ok := a.PlanWithOptions(start, goal, wall, PlanOptions{Hard: 0.10, Soft: 0.27, Edge: 0.1}, area); !ok {
		t.Error("a 0.30 m slot should be passable at a 0.10 m hard limit")
	}
}

func TestAStar_PlanWithOptions_EdgeMarginIsSeparate(t *testing.T) {
	a := areaAStar()
	area := mustArea(t, [4][2]float64{{0, 0}, {3, 0}, {3, 2}, {0, 2}})
	// Goal 0.12 m from the edge: fine for a 0.10 m edge margin, too close for 0.27 m.
	goal := [2]float64{1.5, 0.12}
	if _, ok := a.PlanWithOptions([2]float64{1.5, 1.5}, goal, nil, PlanOptions{Hard: 0.22, Soft: 0.27, Edge: 0.10}, area); !ok {
		t.Error("goal 0.12 m from the edge should be allowed with a 0.10 m edge margin")
	}
	if _, ok := a.PlanWithOptions([2]float64{1.5, 1.5}, goal, nil, PlanOptions{Hard: 0.22, Soft: 0.27, Edge: 0.27}, area); ok {
		t.Error("goal 0.12 m from the edge should be refused with a 0.27 m edge margin")
	}
}

func TestSegmentMargin(t *testing.T) {
	opts := PlanOptions{Hard: 0.22, Soft: 0.27}
	tests := []struct {
		name      string
		endpoints float64
		want      float64
	}{
		{"open ground keeps the soft margin", math.Inf(1), 0.27},
		{"squeeze relaxes to what the path had", 0.26, 0.26 - clearanceTolerance},
		{"never below the hard limit", 0.10, 0.22},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := segmentMargin(opts, tt.endpoints); math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("segmentMargin = %v, want %v", got, tt.want)
			}
		})
	}
}
