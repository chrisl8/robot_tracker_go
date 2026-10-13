package followsim

import (
	"math"

	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
)

// Observation is everything a follower is told each frame: what the camera
// reports, and when. No speeds, no commands' effects, nothing about the plant.
type Observation struct {
	Pos          [2]float64 // measured position, metres
	Heading      float64    // measured heading, radians
	HeadingDelta float64    // change in measured heading since the last frame
	Seconds      float64    // simulated time
}

// Follower turns an observation into the next command. Done reports that it
// considers the goal reached.
type Follower interface {
	Step(obs Observation) (cmd controller.Command, done bool)
	// Replans is how many times it re-planned (a sign it got too close to something).
	Replans() int
}

// FollowerEnv is what a follower is built from.
type FollowerEnv struct {
	Planner  *planning.Planner
	RobotID  int
	Exec     ExecutorSettings
	Diameter float64
}

// ExecutorSettings mirror path_execution in config/tracking_config.yaml (a
// test checks they still do).
type ExecutorSettings struct {
	WaypointThreshold   float64
	SpinThresholdDeg    float64
	BurstFrames         int
	MaxWaitFrames       int
	ForwardThresholdDeg float64
}

// DefaultExecutorSettings are the tuned values from the config.
func DefaultExecutorSettings() ExecutorSettings {
	return ExecutorSettings{
		WaypointThreshold: 0.1, SpinThresholdDeg: 20, BurstFrames: 2, MaxWaitFrames: 2, ForwardThresholdDeg: 20,
	}
}

func (s ExecutorSettings) newExecutor() *controller.PathExecutor {
	e := controller.NewPathExecutor()
	e.SpinThresholdDeg = s.SpinThresholdDeg
	e.BurstFrames = s.BurstFrames
	e.MaxWaitFrames = s.MaxWaitFrames
	e.ForwardThresholdDeg = s.ForwardThresholdDeg
	return e
}

// Constants copied from cmd/autonomy.go, which lives in package main behind
// the OpenCV build tag and so cannot be imported. Keep them in step.
const (
	proximityClearance   = 0.04 // proximityClearance
	proximityProbeStep   = 0.05 // proximityProbeStep
	proximityReplanEvery = 3.0  // seconds: replans are rate-limited in executeAutonomousControl
)

// loop is the part of the per-frame logic that does not depend on how the
// target is chosen: the proximity replan, the goal-arrival check, and the
// proximity backstop. It is a copy of the matching parts of
// executeAutonomousControl in cmd/autonomy.go for one robot; if one changes,
// change the other.
type loop struct {
	env        FollowerEnv
	exec       *controller.PathExecutor
	lastReplan float64
	replans    int
	hasReplan  bool
}

func newLoop(env FollowerEnv) loop { return loop{env: env, exec: env.Exec.newExecutor()} }

// Replans returns how many times the follower re-planned because it got too close to something.
func (l *loop) Replans() int { return l.replans }

// guard runs the replan and goal checks. havePath is false when the robot has
// no path; done is true when the goal is reached.
func (l *loop) guard(obs Observation) (clearance float64, tooClose, havePath, done bool) {
	pl, id := l.env.Planner, l.env.RobotID
	if _, ok := pl.GetNextWaypoint(id); !ok {
		return 0, false, false, false
	}

	clearance = pl.GetClearance(id)
	tooClose = clearance < proximityClearance
	if tooClose && (!l.hasReplan || obs.Seconds-l.lastReplan > proximityReplanEvery) {
		if goal, ok := pl.GetGoal(id); ok {
			pl.ClearPathOnly(id)
			if path, ok := pl.PlanPath(id, obs.Pos, goal); ok {
				pl.SetPath(id, path)
				l.replans++
			}
			l.lastReplan, l.hasReplan = obs.Seconds, true
		}
	}

	if goal, ok := pl.GetGoal(id); ok {
		if math.Hypot(obs.Pos[0]-goal[0], obs.Pos[1]-goal[1]) < l.env.Exec.WaypointThreshold {
			pl.CompletePath(id)
			l.exec.Reset()
			return clearance, tooClose, true, true
		}
	}
	return clearance, tooClose, true, false
}

// steer turns the target into a command and applies the proximity backstop.
func (l *loop) steer(target [2]float64, obs Observation, clearance float64, tooClose bool) controller.Command {
	bearing := math.Atan2(target[1]-obs.Pos[1], target[0]-obs.Pos[0])
	cmd := l.exec.BearingToCommand(obs.Heading, bearing, obs.HeadingDelta)
	if tooClose && l.movesTowardObstacle(cmd, obs, clearance) {
		cmd = controller.CommandStop
	}
	return cmd
}

func (l *loop) movesTowardObstacle(cmd controller.Command, obs Observation, clearance float64) bool {
	dir := obs.Heading
	switch cmd {
	case controller.CommandForward:
	case controller.CommandBackward:
		dir += math.Pi
	case controller.CommandLeft, controller.CommandRight, controller.CommandStop:
		return false
	}
	probe := [2]float64{obs.Pos[0] + proximityProbeStep*math.Cos(dir), obs.Pos[1] + proximityProbeStep*math.Sin(dir)}
	return l.env.Planner.ClearanceAt(l.env.RobotID, probe) < clearance
}

// Baseline is the follower the robot runs today: hop from waypoint to
// waypoint, arriving within the waypoint threshold, and steer at the next one.
type Baseline struct{ loop }

// NewBaseline builds the old waypoint-hopping follower, the per-frame logic of cmd/autonomy.go, as the comparison point for the carrot.
func NewBaseline(env FollowerEnv) Follower { return &Baseline{newLoop(env)} }

// Step returns the next command for the observation, and true once the goal is reached. With no path it stops and keeps waiting.
func (b *Baseline) Step(obs Observation) (controller.Command, bool) {
	clearance, tooClose, havePath, done := b.guard(obs)
	if !havePath {
		return controller.CommandStop, false
	}
	if done {
		return controller.CommandStop, true
	}
	pl, id := b.env.Planner, b.env.RobotID
	if !pl.AdvancePastWaypoints(id, obs.Pos, b.env.Exec.WaypointThreshold) {
		b.exec.Reset()
		return controller.CommandStop, true
	}
	wp, ok := pl.GetNextWaypoint(id)
	if !ok {
		return controller.CommandStop, false
	}
	return b.steer(wp, obs, clearance, tooClose), false
}
