package planning

import (
	"math"
	"math/rand"
	"testing"
)

func testAStar() *AStar {
	return NewAStar(&AStarConfig{GridWidthMeters: 20, GridHeightMeters: 20, Resolution: 0.05, MaxIterations: 200000})
}

// Start/goal cells used int() (truncation toward zero) while obstacles are
// rasterized with floor, so for negative coordinates the start and goal landed
// one cell off from the grid the obstacles live on.
func TestAStar_NegativeCoordinatesMapToTheirOwnCell(t *testing.T) {
	a := testAStar()
	start := [2]float64{-0.07, -0.07} // floor cell is [-0.10,-0.05); int() truncation picked [-0.05,0)
	goal := [2]float64{-1.02, -0.53}

	path, ok := a.Plan(start, goal, nil, 0)
	if !ok {
		t.Fatal("no path")
	}

	// The first waypoint is the centre of the cell containing start.
	if d := math.Hypot(path[0][0]-start[0], path[0][1]-start[1]); d > 0.05*math.Sqrt2/2+1e-9 {
		t.Errorf("first waypoint %v is %.3fm from start %v, want within half a cell diagonal", path[0], d, start)
	}
	// The last waypoint is exactly where the operator asked to go.
	if last := path[len(path)-1]; last != goal {
		t.Errorf("last waypoint = %v, want the exact goal %v", last, goal)
	}
}

// Waypoints are cell centres (that is where obstacles are tested), not corners.
func TestAStar_WaypointsAreCellCentres(t *testing.T) {
	a := testAStar()
	path, ok := a.Plan([2]float64{0.025, 0.025}, [2]float64{0.525, 0.025}, nil, 0)
	if !ok {
		t.Fatal("no path")
	}
	for i, p := range path[:len(path)-1] { // the last is the exact goal
		fx := math.Mod(math.Abs(p[0])/0.05, 1)
		fy := math.Mod(math.Abs(p[1])/0.05, 1)
		if math.Abs(fx-0.5) > 1e-6 || math.Abs(fy-0.5) > 1e-6 {
			t.Errorf("waypoint %d = %v is not on a cell centre (cell fractions %.2f, %.2f)", i, p, fx, fy)
		}
	}
}

func TestAStar_StartInGoalCellIsASuccessNotAFailure(t *testing.T) {
	a := testAStar()

	path, ok := a.Plan([2]float64{1.01, 1.01}, [2]float64{1.03, 1.02}, nil, 0)

	if !ok || len(path) != 1 || path[0] != [2]float64{1.03, 1.02} {
		t.Errorf("path = %v ok=%v, want the single exact goal waypoint", path, ok)
	}
}

// Two obstacles touching only at a corner leave no gap. A diagonal move between
// the two blocked orthogonal cells used to squeeze straight through.
func TestAStar_DoesNotCutCornersBetweenDiagonalObstacles(t *testing.T) {
	a := testAStar()
	// With margin 0 these block exactly cells (0,0) and (1,1).
	obstacles := []Obstacle{
		NewRectObstacle("a", [2]float64{0.0, 0.0}, [2]float64{0.05, 0.05}),
		NewRectObstacle("b", [2]float64{0.05, 0.05}, [2]float64{0.10, 0.10}),
	}
	start := [2]float64{0.075, 0.025} // cell (1,0)
	goal := [2]float64{0.025, 0.075}  // cell (0,1)

	path, ok := a.Plan(start, goal, obstacles, 0)

	if !ok {
		t.Fatal("expected a detour path around the pair")
	}
	if len(path) <= 2 {
		t.Errorf("path %v goes straight through the corner gap between the obstacles", path)
	}
}

// Every segment of a raw A* path must keep the margin clear of every obstacle.
// (Sampled along the segment; consecutive cell centres can dip a hair inside the
// margin between them, hence the small tolerance.)
func TestAStar_PathSegmentsRespectTheMargin(t *testing.T) {
	const margin = 0.15
	const tolerance = 0.01
	a := testAStar()
	rng := rand.New(rand.NewSource(7))

	checked := 0
	for trial := 0; trial < 400; trial++ {
		var obstacles []Obstacle
		for i := 0; i < 1+rng.Intn(6); i++ {
			x, y := rng.Float64()*4-2, rng.Float64()*4-2
			w, h := 0.1+rng.Float64()*0.7, 0.1+rng.Float64()*0.7
			obstacles = append(obstacles, NewRectObstacle("o", [2]float64{x, y}, [2]float64{x + w, y + h}))
		}
		start := [2]float64{rng.Float64()*4 - 2, rng.Float64()*4 - 2}
		goal := [2]float64{rng.Float64()*4 - 2, rng.Float64()*4 - 2}
		if clearance(RobotState{Position: start}, obstacles) < margin+0.1 ||
			clearance(RobotState{Position: goal}, obstacles) < margin+0.1 {
			continue // start/goal too close to an obstacle for a clean comparison
		}

		path, ok := a.Plan(start, goal, obstacles, margin)
		if !ok || len(path) < 2 {
			continue
		}
		checked++

		for i := 0; i+1 < len(path); i++ {
			for k := 0; k <= 20; k++ {
				f := float64(k) / 20
				pt := [2]float64{path[i][0] + f*(path[i+1][0]-path[i][0]), path[i][1] + f*(path[i+1][1]-path[i][1])}
				if c := clearance(RobotState{Position: pt}, obstacles); c < margin-tolerance {
					t.Fatalf("trial %d: point %v on segment %v->%v is %.3fm from an obstacle, margin %.2f",
						trial, pt, path[i], path[i+1], c, margin)
				}
			}
		}
	}
	if checked < 100 {
		t.Fatalf("only %d of 400 trials were checkable; the test is not exercising anything", checked)
	}
}
