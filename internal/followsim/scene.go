package followsim

import (
	"math"

	"github.com/chrisl8/robot_tracker_go/internal/planning"
)

// RobotID is the tag the simulated robot carries.
const RobotID = 1

// Diameter is the simulated robot's footprint, as configured for the Vorpal.
const Diameter = 0.30

// Scene is a floor, its obstacles, and one trip across it.
type Scene struct {
	Name      string
	Area      [4][2]float64 // what the camera sees, as the planner is told
	Obstacles []planning.Obstacle
	Start     [2]float64
	Goal      [2]float64
}

// FrameMargin is how far p is inside the camera's view: the smallest distance
// to any edge, negative if outside. A robot outside is a robot lost.
func (s Scene) FrameMargin(p [2]float64) float64 {
	area, ok := planning.NewArea(s.Area)
	if !ok {
		return math.Inf(1)
	}
	return area.Margin(p)
}

// Scenes are the trips the followers are judged on. The first two are the real
// floor that exposed the early-arrival and dense-waypoint problems (obstacles
// and calibration from config/); the third is plain "go round a block", the
// walk-a-square case, in the open.
func Scenes() []Scene {
	realArea := [4][2]float64{{-1.3596, -0.8617}, {1.4743, -0.6852}, {1.5082, 1.0144}, {-1.5909, 0.8065}}
	realObstacles := func() []planning.Obstacle {
		return []planning.Obstacle{
			planning.NewRectObstacle("edge", [2]float64{1.4169, 0.3333}, [2]float64{1.5082, 1.0144}),
			planning.NewRectObstacle("corner", [2]float64{-1.5399, 0.4386}, [2]float64{-1.2377, 0.8302}),
			planning.NewRectObstacle("bar", [2]float64{-0.9140, -0.2415}, [2]float64{1.0318, 0.0545}),
		}
	}
	return []Scene{
		{Name: "bar detour", Area: realArea, Obstacles: realObstacles(), Start: [2]float64{0.4, 0.6}, Goal: [2]float64{1.2843, 0.0009}},
		{Name: "bar detour, reversed", Area: realArea, Obstacles: realObstacles(), Start: [2]float64{1.2843, 0.0009}, Goal: [2]float64{0.4, 0.6}},
		{
			Name:      "round a block",
			Area:      [4][2]float64{{-1.5, -0.9}, {1.5, -0.9}, {1.5, 0.9}, {-1.5, 0.9}},
			Obstacles: []planning.Obstacle{planning.NewRectObstacle("block", [2]float64{-0.3, -0.35}, [2]float64{0.3, 0.35})},
			Start:     [2]float64{-1.0, 0}, Goal: [2]float64{1.0, 0},
		},
	}
}
