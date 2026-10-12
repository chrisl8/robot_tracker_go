package followsim

import (
	"math"
	"math/rand"

	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
)

// MaxSeconds is how long a run may take before it is called a failure.
const MaxSeconds = 240.0

// FollowerFactory builds the follower under test for one run.
type FollowerFactory func(FollowerEnv) Follower

// Result is what one run measured, from the TRUE state of the robot (not what
// the camera reported).
type Result struct {
	Seed           int64
	Params         PlantParams
	PlanFailed     bool    // the planner found no path to begin with
	Reached        bool    // follower declared arrival AND the robot really is near the goal
	FinalDist      float64 // true distance to the goal at the end, metres
	Seconds        float64
	Travelled      float64 // metres actually walked
	MinClearance   float64 // smallest true gap between robot body and any obstacle
	Collided       bool    // body overlapped an obstacle at some point
	MinFrameMargin float64 // smallest distance of the robot's centre inside the camera's view
	LeftFrame      bool    // centre went outside the view (a lost robot, in life)
	MaxCrossTrack  float64 // greatest distance from the first planned path
	Replans        int
	CmdSwitches    int // changes of command between frames
}

// arrivedWithin is how close to the goal the robot must truly be for a
// follower's "done" to count as having arrived: the goal marker's radius.
const arrivedWithin = 0.30

// Run drives one trip of scene with a plant drawn from seed and the follower
// from factory, and measures what really happened.
func Run(scene Scene, factory FollowerFactory, exec ExecutorSettings, seed int64) Result {
	rng := rand.New(rand.NewSource(seed)) // #nosec G404 -- simulation, not security
	params := SampleParams(rng)
	return RunWith(scene, factory, exec, params, rng, seed)
}

// RunWith is Run with the plant given (the simulator's own tests use it).
func RunWith(scene Scene, factory FollowerFactory, exec ExecutorSettings, params PlantParams, rng *rand.Rand, seed int64) Result {
	res := Result{Seed: seed, Params: params, MinClearance: math.Inf(1), MinFrameMargin: math.Inf(1)}

	planner := planning.NewPlanner(nil)
	area, ok := planning.NewArea(scene.Area)
	if !ok {
		res.PlanFailed = true
		return res
	}
	planner.SetArea(area)
	planner.SetObstacles(scene.Obstacles)
	planner.AddRobot(RobotID, scene.Start, Diameter)
	planner.SetGoal(RobotID, scene.Goal)
	first := planner.GetPathsWithGoals()[RobotID]
	if len(first) == 0 {
		res.PlanFailed = true
		return res
	}
	first = append([][2]float64{scene.Start}, first...)

	theta := (rng.Float64()*2 - 1) * math.Pi // #nosec G404 -- simulation, not security
	plant := NewPlant(params, rng, scene.Start[0], scene.Start[1], theta)
	sensor := NewSensor(params, rng)
	follower := factory(FollowerEnv{Planner: planner, RobotID: RobotID, Exec: exec, Diameter: Diameter})

	prev := controller.CommandStop
	frames := int(MaxSeconds * FrameRate)
	done := false
	for f := 0; f < frames && !done; f++ {
		pos, heading, delta := sensor.Measure(plant)
		planner.AddRobot(RobotID, pos, Diameter)

		var cmd controller.Command
		cmd, done = follower.Step(Observation{Pos: pos, Heading: heading, HeadingDelta: delta, Seconds: float64(f) * dt})
		if cmd != prev {
			res.CmdSwitches++
		}
		prev = cmd
		plant.Step(cmd)

		truePos := [2]float64{plant.X, plant.Y}
		if c := planner.ClearanceAt(RobotID, truePos); c < res.MinClearance {
			res.MinClearance = c
		}
		if m := scene.FrameMargin(truePos); m < res.MinFrameMargin {
			res.MinFrameMargin = m
		}
		if x := distToPolyline(truePos, first); x > res.MaxCrossTrack {
			res.MaxCrossTrack = x
		}
		res.Seconds = float64(f+1) * dt
	}

	res.FinalDist = math.Hypot(plant.X-scene.Goal[0], plant.Y-scene.Goal[1])
	res.Travelled = plant.Travelled
	res.Reached = done && res.FinalDist <= arrivedWithin
	res.Collided = res.MinClearance < 0
	res.LeftFrame = res.MinFrameMargin < 0
	res.Replans = follower.Replans()
	return res
}

// RunMany runs n trips with seeds baseSeed, baseSeed+1, ... so two followers
// can be compared on exactly the same sequence of robots and floors.
func RunMany(scene Scene, factory FollowerFactory, exec ExecutorSettings, n int, baseSeed int64) []Result {
	out := make([]Result, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, Run(scene, factory, exec, baseSeed+int64(i)))
	}
	return out
}

func distToPolyline(p [2]float64, path [][2]float64) float64 {
	best := math.Inf(1)
	for i := 0; i+1 < len(path); i++ {
		a, b := path[i], path[i+1]
		abx, aby := b[0]-a[0], b[1]-a[1]
		t := 0.0
		if l2 := abx*abx + aby*aby; l2 > 0 {
			t = math.Max(0, math.Min(1, ((p[0]-a[0])*abx+(p[1]-a[1])*aby)/l2))
		}
		if d := math.Hypot(p[0]-(a[0]+t*abx), p[1]-(a[1]+t*aby)); d < best {
			best = d
		}
	}
	return best
}
