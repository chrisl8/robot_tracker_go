package planning

import (
	"math"
	"testing"
)

func pathLength(path [][2]float64) float64 {
	l := 0.0
	for i := 1; i < len(path); i++ {
		l += math.Hypot(path[i][0]-path[i-1][0], path[i][1]-path[i-1][1])
	}
	return l
}

// The real scene that exposed the problem: the camera's floor, the bar and
// two edge obstacles, with the goal beside the bar's far end so the only way
// is round the near end. Coordinates are from config/obstacles_Camera_0.yaml
// and calibration_Camera_0.yaml.
//
// A path that has to bend round obstacles used to come back from planning
// almost unsimplified for some start positions (19 to 62 waypoints, a point
// every 5 cm, for a route that needs about 8): one simplified segment that cut
// a corner was rejected, and the old fallback then restored every raw point it
// covered. The robot turns, pauses and bursts at each waypoint, so a staircase
// of waypoints is a slow, jerky drive.
func TestPlanPath_StaysSimpleWhereverTheRobotStarts(t *testing.T) {
	p := NewPlanner(nil)
	p.AddRobot(1, [2]float64{0.4, 0.6}, 0.30)
	p.SetArea(mustArea(t, [4][2]float64{{-1.3596, -0.8617}, {1.4743, -0.6852}, {1.5082, 1.0144}, {-1.5909, 0.8065}}))
	obstacles := []Obstacle{
		NewRectObstacle("edge", [2]float64{1.4169, 0.3333}, [2]float64{1.5082, 1.0144}),
		NewRectObstacle("corner", [2]float64{-1.5399, 0.4386}, [2]float64{-1.2377, 0.8302}),
		NewRectObstacle("bar", [2]float64{-0.9140, -0.2415}, [2]float64{1.0318, 0.0545}),
	}
	p.SetObstacles(obstacles)
	goal := [2]float64{1.2843, 0.0009}
	opts := p.planOptionsLocked(1)

	const maxWaypoints = 15 // the route needs 7 to 11; the old code gave 19 to 62
	checked := 0
	for _, sx := range []float64{-1.2, -0.9, -0.5, 0, 0.4, 0.8, 1.2} {
		for _, sy := range []float64{0.3, 0.6, 0.9} {
			start := [2]float64{sx, sy}
			raw, ok := p.globalPlanner.PlanWithOptions(start, goal, obstacles, opts, p.area)
			if !ok {
				continue // e.g. a start inside the corner obstacle's own margin
			}
			final, ok := p.PlanPath(1, start, goal)
			if !ok {
				t.Fatalf("start %v: PlanPath failed where A* succeeded", start)
			}
			checked++

			if len(final) > maxWaypoints {
				t.Errorf("start %v: %d waypoints (raw A* path has %d), want at most %d: a corner-cutting segment must cost a waypoint or two, not the whole stretch",
					start, len(final), len(raw), maxWaypoints)
			}
			// Simplifying must not trade safety for fewer points, nor lengthen the route.
			if rc, fc := minClearance(raw, obstacles), minClearance(final, obstacles); fc < rc-0.02 {
				t.Errorf("start %v: clearance %.3f m is below the raw path's %.3f m", start, fc, rc)
			}
			if rl, fl := pathLength(raw), pathLength(final); fl > rl+0.01 {
				t.Errorf("start %v: simplified path is longer (%.2f m) than the raw one (%.2f m)", start, fl, rl)
			}
		}
	}
	if checked < 15 {
		t.Fatalf("only %d of 21 starts produced a path; the test is not exercising the scene", checked)
	}
}
