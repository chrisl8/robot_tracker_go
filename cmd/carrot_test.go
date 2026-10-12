//go:build gocv

package main

import (
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/config"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
)

// carrotRig is a rig whose robot has to go round a block to reach its goal.
func carrotRig(t *testing.T) (*RobotSystem, [][2]float64) {
	t.Helper()
	rs := newDemoAutonomyRig(t)
	startRigQueue(t, rs)
	rs.io.useCarrot = true
	rs.planning.planner.SetObstacles([]planning.Obstacle{
		planning.NewRectObstacle("block", [2]float64{-0.2, -0.4}, [2]float64{0.2, 0.4}),
	})
	rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{-1, 0}, 0.3)
	rs.planning.planner.SetGoal(demoAutonomyTagID, [2]float64{1, 0})
	path := rs.planning.planner.FullPath(demoAutonomyTagID)
	if len(path) < 3 {
		t.Fatalf("test setup: expected a path round the block, got %v", path)
	}
	return rs, path
}

func carrotFrame(rs *RobotSystem, pos [2]float64, heading float64) {
	track := confirmedTrack(1, demoAutonomyTagID)
	track.WorldPos = pos
	track.Heading = heading
	rs.executeAutonomousControl([]tracking.Track{track})
}

func TestCarrotFollower_DrivesAndKeepsThePlannerInStep(t *testing.T) {
	rs, path := carrotRig(t)

	carrotFrame(rs, [2]float64{-1, 0}, 0)
	if got := rs.io.robotCommands[demoAutonomyTagID]; got == "" || got == "stopped" {
		t.Fatalf("the carrot follower issued no drive command: %q", got)
	}
	if rs.io.carrots[demoAutonomyTagID] == nil {
		t.Fatal("no carrot tracker was created for the robot")
	}

	// The robot reaches the path's second waypoint: the planner (and so the path
	// drawn to the operator) must have moved on past the ones already behind it.
	carrotFrame(rs, path[1], 0)
	next, ok := rs.planning.planner.GetNextWaypoint(demoAutonomyTagID)
	if !ok || next == path[0] || next == path[1] {
		t.Errorf("next waypoint %v ok=%v after reaching path[1]=%v: the planner was not told the robot's progress", next, ok, path[1])
	}
	if remaining := rs.planning.planner.GetPathsWithGoals()[demoAutonomyTagID]; len(remaining) >= len(path) {
		t.Errorf("the drawn path still has %d of %d points", len(remaining), len(path))
	}
}

func TestCarrotFollower_ForgetsARobotWhoseGoalEnded(t *testing.T) {
	rs, _ := carrotRig(t)
	carrotFrame(rs, [2]float64{-1, 0}, 0)
	if rs.io.carrots[demoAutonomyTagID] == nil {
		t.Fatal("setup: no tracker")
	}
	rs.planning.planner.CompletePath(demoAutonomyTagID)
	carrotFrame(rs, [2]float64{-1, 0}, 0)
	if rs.io.carrots[demoAutonomyTagID] != nil {
		t.Error("the tracker outlived the robot's path: a later goal with an identical path would start from stale progress")
	}
}

func TestCarrotFollower_ReachesTheGoalLikeTheWaypointFollower(t *testing.T) {
	rs, path := carrotRig(t)
	carrotFrame(rs, path[len(path)-1], 0) // standing on the goal
	if _, hasGoal := rs.planning.planner.GetGoal(demoAutonomyTagID); hasGoal {
		t.Error("the goal was not completed when the robot stood on it")
	}
	if got := rs.io.robotCommands[demoAutonomyTagID]; got != "stopped" {
		t.Errorf("command at the goal = %q, want stopped", got)
	}
}

func TestInitPathExecutor_FollowerSwitch(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{{"", true}, {"waypoint", false}, {"carrot", true}, {"nonsense", true}} {
		t.Run(tc.value, func(t *testing.T) {
			rs := NewRobotSystem(&config.Config{PathExecution: config.PathExecutionConfig{Follower: tc.value}})
			rs.initPathExecutor()
			if rs.io.useCarrot != tc.want {
				t.Errorf("follower %q: useCarrot = %v, want %v", tc.value, rs.io.useCarrot, tc.want)
			}
		})
	}
}

func TestCarrotPoints_ReportsWhereTheRobotIsSteering(t *testing.T) {
	rs, path := carrotRig(t)
	if got := rs.carrotPoints(); len(got) != 0 {
		t.Fatalf("carrot reported before any frame: %v", got)
	}
	carrotFrame(rs, [2]float64{-1, 0}, 0)
	got, ok := rs.carrotPoints()[demoAutonomyTagID]
	if !ok {
		t.Fatal("no carrot reported for the driving robot")
	}
	want, _ := rs.io.carrots[demoAutonomyTagID].tracker.Target([2]float64{-1, 0})
	if got != want || got == ([2]float64{}) || got == path[len(path)-1] {
		t.Errorf("carrot %v, want the tracker's point %v ahead on the path", got, want)
	}
}
