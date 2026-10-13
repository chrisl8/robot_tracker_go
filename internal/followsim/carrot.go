package followsim

import (
	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/pursuit"
)

// Carrot steers at a point ahead of its progress along the planned path (held
// at bends until the robot is close enough to pivot there) instead of hopping
// between waypoints. Everything else is the Baseline's: the goal check, the
// proximity backstop, and the PathExecutor. The path handling is a copy of
// carrotTarget in cmd/carrot.go; if one changes, change the other.
type Carrot struct {
	loop
	tracker *pursuit.Tracker
	path    [][2]float64
}

// NewCarrot builds the carrot (look-ahead) follower: the per-frame logic of cmd/carrot.go, driven by the simulator instead of the live system.
func NewCarrot(env FollowerEnv) Follower {
	return &Carrot{
		loop:    newLoop(env),
		tracker: pursuit.New(pursuit.DefaultParams(env.Diameter), env.Planner.World(env.RobotID)),
	}
}

// Step returns the next command for the observation, and true once the goal is reached. With no path it stops and keeps waiting.
func (c *Carrot) Step(obs Observation) (controller.Command, bool) {
	clearance, tooClose, havePath, done := c.guard(obs)
	if !havePath {
		return controller.CommandStop, false
	}
	if done {
		return controller.CommandStop, true
	}
	pl, id := c.env.Planner, c.env.RobotID
	// The planner's path changes when it is replanned: follow the new one from its start.
	if path := pl.FullPath(id); !pursuit.SamePath(path, c.path) {
		c.path = path
		c.tracker.SetPath(path)
	}
	target, ok := c.tracker.Target(obs.Pos)
	if !ok {
		return controller.CommandStop, false
	}
	pl.SetWaypointIndex(id, c.tracker.NextVertex())
	return c.steer(target, obs, clearance, tooClose), false
}
