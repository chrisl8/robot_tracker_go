package planning

import (
	"math"
	"testing"
)

// A trapezoid like a camera looking down a room at an angle: narrow far edge.
var trapezoid = [4][2]float64{{1, 0}, {3, 0}, {4, 3}, {0, 3}}

func TestNewArea(t *testing.T) {
	tests := []struct {
		name    string
		corners [4][2]float64
		ok      bool
	}{
		{"convex trapezoid", trapezoid, true},
		{"clockwise winding", [4][2]float64{trapezoid[3], trapezoid[2], trapezoid[1], trapezoid[0]}, true},
		{"bowtie", [4][2]float64{{0, 0}, {4, 3}, {4, 0}, {0, 3}}, false},
		{"concave", [4][2]float64{{0, 0}, {4, 0}, {1, 1}, {0, 4}}, false},
		{"degenerate", [4][2]float64{{0, 0}, {1, 1}, {2, 2}, {3, 3}}, false},
		{"NaN corner", [4][2]float64{{0, 0}, {4, 0}, {math.NaN(), 3}, {0, 3}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := NewArea(tt.corners); ok != tt.ok {
				t.Errorf("NewArea ok = %v, want %v", ok, tt.ok)
			}
		})
	}
}

func TestArea_Contains(t *testing.T) {
	for _, corners := range [][4][2]float64{trapezoid, {trapezoid[3], trapezoid[2], trapezoid[1], trapezoid[0]}} {
		a := mustArea(t, corners)
		tests := []struct {
			name  string
			p     [2]float64
			inset float64
			want  bool
		}{
			{"centre", [2]float64{2, 1.5}, 0, true},
			{"outside below", [2]float64{2, -0.1}, 0, false},
			{"outside the slanted side", [2]float64{0.4, 0.1}, 0, false}, // inside the bounding box only
			{"inside the slanted side", [2]float64{1.2, 0.5}, 0, true},
			{"near the bottom edge, too close", [2]float64{2, 0.1}, 0.2, false},
			{"near the bottom edge, clear", [2]float64{2, 0.3}, 0.2, true},
		}
		for _, tt := range tests {
			if got := a.Contains(tt.p, tt.inset); got != tt.want {
				t.Errorf("%s: Contains(%v, %v) = %v, want %v", tt.name, tt.p, tt.inset, got, tt.want)
			}
		}
	}
}

func mustArea(t *testing.T, corners [4][2]float64) *Area {
	t.Helper()
	a, ok := NewArea(corners)
	if !ok || a == nil {
		t.Fatalf("NewArea(%v) failed", corners)
	}
	return a
}

func areaAStar() *AStar {
	return NewAStar(&AStarConfig{GridWidthMeters: 20, GridHeightMeters: 20, Resolution: 0.05, MaxIterations: 200000})
}

// A wall whose only way round is outside the area: unrestricted A* goes round
// it, restricted A* must report no path rather than leave the camera's view.
func TestAStar_PlanWithinNeverLeavesArea(t *testing.T) {
	area := mustArea(t, [4][2]float64{{0, 0}, {6, 0}, {6, 4}, {0, 4}})
	wall := Obstacle{Quad: Quad{{2.9, -1}, {3.1, -1}, {3.1, 5}, {2.9, 5}}} // spans the whole view and beyond
	start, goal := [2]float64{1, 2}, [2]float64{5, 2}

	if _, ok := areaAStar().Plan(start, goal, []Obstacle{wall}, 0); !ok {
		t.Fatal("control: unrestricted plan should route round the wall")
	}
	if path, ok := areaAStar().PlanWithin(start, goal, []Obstacle{wall}, 0, area); ok {
		t.Errorf("path found only by leaving the area: %v", path)
	}
}

// With room to spare, every waypoint keeps the margin from every edge, even
// though the straight line (a diagonal along the slanted side) hugs the edge.
func TestAStar_PlanWithinKeepsMarginFromEdges(t *testing.T) {
	area := mustArea(t, trapezoid)
	const margin = 0.3
	path, ok := areaAStar().PlanWithin([2]float64{1.8, 0.6}, [2]float64{0.9, 2.5}, nil, margin, area)
	if !ok {
		t.Fatal("no path")
	}
	for _, p := range path {
		if !area.Contains(p, margin-0.05) { // cell centres are within half a cell of the limit
			t.Errorf("waypoint %v is closer than the margin to an edge", p)
		}
	}
}

func TestAStar_PlanWithinRejectsGoalOutsideOrTooCloseToEdge(t *testing.T) {
	area := mustArea(t, [4][2]float64{{0, 0}, {6, 0}, {6, 4}, {0, 4}})
	tests := []struct {
		name   string
		goal   [2]float64
		margin float64
		ok     bool
	}{
		{"inside", [2]float64{3, 2}, 0.3, true},
		{"beyond the edge", [2]float64{7, 2}, 0.3, false},
		{"in the margin", [2]float64{5.9, 2}, 0.3, false},
		{"in the margin but margin 0", [2]float64{5.9, 2}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := areaAStar().PlanWithin([2]float64{1, 1}, tt.goal, nil, tt.margin, area); ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
		})
	}
}

// A robot standing in the edge margin (it was driven or set down there) must
// still be able to plan a way back in, and must not be led further out.
func TestAStar_PlanWithinStartInEdgeMargin(t *testing.T) {
	area := mustArea(t, [4][2]float64{{0, 0}, {6, 0}, {6, 4}, {0, 4}})
	start := [2]float64{0.1, 2} // 0.1m from the left edge, margin is 0.3
	path, ok := areaAStar().PlanWithin(start, [2]float64{3, 2}, nil, 0.3, area)
	if !ok {
		t.Fatal("robot in the margin is trapped")
	}
	for _, p := range path {
		if p[0] < start[0]-0.05 {
			t.Errorf("waypoint %v moves towards the edge from %v", p, start)
		}
	}
}

func TestPlanner_SetAreaConfinesAndReplans(t *testing.T) {
	p := NewPlanner(nil)
	p.AddRobot(1, [2]float64{1, 2}, 0.3)
	p.SetGoal(1, [2]float64{3, 2})
	if _, ok := p.GetPathsWithGoals()[1]; !ok {
		t.Fatal("no path before area set")
	}

	// Shrink the world so the goal is outside: the stale path must be dropped
	// and must not be replanned while the goal stays outside.
	area := mustArea(t, [4][2]float64{{0, 0}, {2, 0}, {2, 4}, {0, 4}})
	p.SetArea(area)
	if path, ok := p.GetPathsWithGoals()[1]; ok {
		t.Errorf("path to a goal outside the area was kept: %v", path)
	}

	// Identical area again does not replan.
	before := p.replans
	p.SetArea(area)
	if p.replans != before {
		t.Error("unchanged area triggered a replan")
	}

	// Lifting the restriction restores planning.
	p.SetArea(nil)
	if _, ok := p.GetPathsWithGoals()[1]; !ok {
		t.Error("no path after lifting the area")
	}
}
