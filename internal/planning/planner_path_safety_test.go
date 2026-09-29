package planning

import (
	"math"
	"math/rand"
	"testing"
)

// A path that detours around a wall has a waypoint (the corner) that is
// farther from the robot than the goal behind the wall. The old overshoot rule
// ("closer to the next waypoint than to the current one") skipped the corner
// and steered the robot straight into the wall.
func TestAdvancePastWaypoints_DoesNotSkipDetourCornerThroughObstacle(t *testing.T) {
	p := NewPlanner(nil)
	p.SetObstacles([]Obstacle{
		NewRectObstacle("wall", [2]float64{0.45, -0.5}, [2]float64{0.55, 0.5}),
	})
	corner := [2]float64{0.5, 0.9} // over the top of the wall
	goal := [2]float64{0.7, 0}     // straight behind it: closer than the corner
	p.paths[1] = [][2]float64{corner, goal}
	p.currentWaypoint[1] = 0

	p.AdvancePastWaypoints(1, [2]float64{0, 0}, 0.05)

	if got := p.currentWaypoint[1]; got != 0 {
		t.Errorf("currentWaypoint = %d, want 0: skipped the detour corner and would drive into the wall", got)
	}
}

func TestAdvancePastWaypoints_StillSkipsOvershotWaypointWhenLineIsClear(t *testing.T) {
	p := NewPlanner(nil) // no obstacles
	p.paths[1] = [][2]float64{{0.5, 0}, {1.0, 0}, {2.0, 0}}
	p.currentWaypoint[1] = 0

	// The robot is already past the first waypoint (closer to the second).
	p.AdvancePastWaypoints(1, [2]float64{0.9, 0.3}, 0.05)

	if got := p.currentWaypoint[1]; got != 1 {
		t.Errorf("currentWaypoint = %d, want 1 (overshot waypoint skipped)", got)
	}
}

func TestAdvancePastWaypoints_ReachedWaypointAlwaysAdvances(t *testing.T) {
	p := NewPlanner(nil)
	p.SetObstacles([]Obstacle{
		NewRectObstacle("wall", [2]float64{0.45, -0.5}, [2]float64{0.55, 0.5}),
	})
	p.paths[1] = [][2]float64{{0.5, 0.9}, {0.7, 0}}
	p.currentWaypoint[1] = 0

	p.AdvancePastWaypoints(1, [2]float64{0.5, 0.9}, 0.05) // standing on the corner

	if got := p.currentWaypoint[1]; got != 1 {
		t.Errorf("currentWaypoint = %d, want 1 (waypoint reached)", got)
	}
}

// minClearance samples every segment of path and returns the smallest distance
// to any obstacle footprint.
func minClearance(path [][2]float64, obstacles []Obstacle) float64 {
	best := 1e9
	for i := 0; i+1 < len(path); i++ {
		a, b := path[i], path[i+1]
		const steps = 100
		for k := 0; k <= steps; k++ {
			f := float64(k) / steps
			pt := [2]float64{a[0] + f*(b[0]-a[0]), a[1] + f*(b[1]-a[1])}
			for _, obs := range obstacles {
				if d := distanceToQuad(pt, obs.Quad); d < best {
					best = d
				}
			}
		}
	}
	return best
}

// Simplifying a path must not move it meaningfully closer to obstacles than the
// raw A* path already was. Simplifying a second time after validation used to
// drop the waypoints validation had restored, re-introducing chords that clip
// obstacles (Douglas-Peucker epsilon is 15cm). The comparison is relative so
// it isolates simplification from A*'s own grid discretization.
func TestPlanPath_SimplificationDoesNotCutTowardObstacles(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	checked := 0
	for trial := 0; trial < 400; trial++ {
		p := NewPlanner(nil)
		var obstacles []Obstacle
		for i := 0; i < 1+rng.Intn(5); i++ {
			x, y := rng.Float64()*3-1.5, rng.Float64()*3-1.5
			w, h := 0.1+rng.Float64()*0.6, 0.1+rng.Float64()*0.6
			obstacles = append(obstacles, NewRectObstacle("o", [2]float64{x, y}, [2]float64{x + w, y + h}))
		}
		p.SetObstacles(obstacles)

		start := [2]float64{rng.Float64()*3 - 1.5, rng.Float64()*3 - 1.5}
		goal := [2]float64{rng.Float64()*3 - 1.5, rng.Float64()*3 - 1.5}

		margin := p.marginLocked(1)
		raw, ok := p.globalPlanner.Plan(start, goal, obstacles, margin)
		if !ok || len(raw) < 3 {
			continue
		}
		final, ok := p.PlanPath(1, start, goal)
		if !ok {
			t.Fatalf("trial %d: PlanPath failed where raw A* succeeded", trial)
		}
		checked++

		// Simplification may trade spare clearance for a shorter path, but it
		// must not go below the robot margin unless the raw path already did.
		rawClear, finalClear := minClearance(raw, obstacles), minClearance(final, obstacles)
		floor := math.Min(rawClear, margin)
		if finalClear < floor-0.02 {
			t.Fatalf("trial %d: simplified path clearance %.3fm is below the floor %.3fm (raw path %.3fm, margin %.3fm)\nraw:   %v\nfinal: %v",
				trial, finalClear, floor, rawClear, margin, raw, final)
		}
	}
	if checked < 100 {
		t.Fatalf("only %d of 400 trials produced a checkable path; test is not exercising anything", checked)
	}
}
