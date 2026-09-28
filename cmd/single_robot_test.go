//go:build gocv

package main

import (
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
)

// startRigQueue replaces the rig's never-started queue with a running one
// (over an unconnected controller, so writes just fail harmlessly). The
// per-robot clear used to be guarded by IsRunning(), so it only misbehaved
// against a running queue.
func startRigQueue(t *testing.T, rs *RobotSystem) {
	t.Helper()
	rs.io.commandQueue = controller.NewCommandQueue(controller.NewArduinoController("none", 0), 100, 0)
	rs.io.commandQueue.Start()
	t.Cleanup(rs.io.commandQueue.Stop)
}

func confirmedTrack(id, tagID int) tracking.Track {
	tag := tagID
	return tracking.Track{
		TrackID:  id,
		TagID:    &tag,
		State:    tracking.TrackStateConfirmed,
		WorldPos: [2]float64{3.2, 2.4},
	}
}

// The controller line, command queue and path executor are shared, so a
// tracked robot that has no path must not clear the command just issued for
// the robot that does. It used to, depending on track order.
func TestExecuteAutonomousControl_PathlessRobotDoesNotClearAnotherRobotsCommand(t *testing.T) {
	for _, tc := range []struct {
		name   string
		tracks func() []tracking.Track
	}{
		{"pathless robot listed after", func() []tracking.Track {
			return []tracking.Track{confirmedTrack(1, demoAutonomyTagID), confirmedTrack(2, 8)}
		}},
		{"pathless robot listed before", func() []tracking.Track {
			return []tracking.Track{confirmedTrack(2, 8), confirmedTrack(1, demoAutonomyTagID)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rs := newDemoAutonomyRig(t)
			startRigQueue(t, rs)
			rs.planning.planner.SetGoal(demoAutonomyTagID, [2]float64{5.2, 5.4})
			rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{3.2, 2.4}, 0.3)
			rs.planning.planner.AddRobot(8, [2]float64{1, 1}, 0.3) // tracked, but no goal/path

			rs.executeAutonomousControl(tc.tracks())

			if !rs.io.commandQueue.HasActiveCommand() {
				t.Error("the path-following robot's command was cleared by a robot with no path")
			}
		})
	}
}

func TestExecuteAutonomousControl_ClearsCommandWhenNoRobotHasAPath(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	startRigQueue(t, rs)
	rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{3.2, 2.4}, 0.3)
	rs.io.commandQueue.Enqueue(controller.CommandForward) // stale command left over from earlier

	rs.executeAutonomousControl([]tracking.Track{confirmedTrack(1, demoAutonomyTagID)})

	if rs.io.commandQueue.HasActiveCommand() {
		t.Error("stale command should be cleared when no robot has a path")
	}
}

// Only one robot may be the controlled robot: giving another robot a goal
// releases the previous one's.
func TestOnDestinationSet_ReleasesOtherRobotsGoals(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	rs.registerWebServerCallbacks()
	rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{1, 1}, 0.3)
	rs.planning.planner.AddRobot(8, [2]float64{2, 2}, 0.3)

	set := rs.web.webServer.Callbacks.OnDestinationSet
	set(demoAutonomyTagID, [2]float64{300, 300})
	set(8, [2]float64{400, 400})

	got := rs.planning.planner.RobotsWithGoals()
	if len(got) != 1 || got[0] != 8 {
		t.Errorf("RobotsWithGoals = %v, want only robot 8", got)
	}
}
