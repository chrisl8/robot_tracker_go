//go:build gocv

package main

import (
	"github.com/chrisl8/robot_tracker_go/internal/pursuit"
)

// carrotState is one robot's carrot tracker and the path it is following.
type carrotState struct {
	tracker *pursuit.Tracker
	path    [][2]float64
	target  [2]float64 // the last carrot, for the UI
}

// carrotTarget is the point robotID should steer at, with the carrot follower
// (path_execution.follower: carrot): a point ahead of its progress along the
// planned path, held at bends until it is close enough to pivot there. ok is
// false when the robot has no path. It also tells the planner which waypoint is
// next, so the path drawn to the operator starts there. The matching code in
// the simulator is Carrot.Step in internal/followsim/carrot.go; if one
// changes, change the other.
func (rs *RobotSystem) carrotTarget(robotID int, pos [2]float64) (target [2]float64, ok bool) {
	pl := rs.planning.planner
	path := pl.FullPath(robotID)
	if len(path) == 0 {
		return pos, false
	}
	if rs.io.carrots == nil {
		rs.io.carrots = make(map[int]*carrotState)
	}
	st := rs.io.carrots[robotID]
	if st == nil {
		diameter, known := pl.RobotDiameter(robotID)
		if !known {
			diameter = defaultRobotDiameter
		}
		st = &carrotState{tracker: pursuit.New(pursuit.DefaultParams(diameter), pl.World(robotID))}
		rs.io.carrots[robotID] = st
	}
	// A replan gives a new path: follow it from its start.
	if !pursuit.SamePath(path, st.path) {
		st.path = path
		st.tracker.SetPath(path)
	}
	target, ok = st.tracker.Target(pos)
	if ok {
		st.target = target
		pl.SetWaypointIndex(robotID, st.tracker.NextVertex())
	}
	return target, ok
}

// pruneCarrots forgets the carrot state of robots that no longer have a path,
// so a later goal (even one with an identical path) starts fresh.
func (rs *RobotSystem) pruneCarrots() {
	for id := range rs.io.carrots {
		if _, hasPath := rs.planning.planner.GetNextWaypoint(id); !hasPath {
			delete(rs.io.carrots, id)
		}
	}
}

// carrotPoints returns each robot's current carrot, for the UI to draw. Empty
// unless the carrot follower is in use.
func (rs *RobotSystem) carrotPoints() map[int][2]float64 {
	out := make(map[int][2]float64, len(rs.io.carrots))
	for id, st := range rs.io.carrots {
		out[id] = st.target
	}
	return out
}
