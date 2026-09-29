//go:build gocv

package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
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

// Goal/waypoint checks must use the track's WorldPos (which includes the
// center_offset correction), not a fresh uncorrected projection of the bbox.
// The bbox here is left at zero, which projects to world (0,0); WorldPos is at
// the goal, so the robot must be treated as having arrived.
func TestExecuteAutonomousControl_UsesTrackWorldPos(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	goal := [2]float64{5.2, 5.4}
	rs.planning.planner.SetGoal(demoAutonomyTagID, goal)
	rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{3.2, 2.4}, 0.3)
	if _, ok := rs.planning.planner.GetGoal(demoAutonomyTagID); !ok {
		t.Fatal("test setup: goal not set")
	}

	track := confirmedTrack(1, demoAutonomyTagID)
	track.WorldPos = goal

	rs.executeAutonomousControl([]tracking.Track{track})

	if _, ok := rs.planning.planner.GetGoal(demoAutonomyTagID); ok {
		t.Error("robot at its goal (by WorldPos) should have completed the goal")
	}
}

// The no-config demo fallback builds no planner; an obstacle edit from the UI
// used to nil-deref it inside the HTTP handler.
func TestDemoCallbacks_ObstaclesChangedToleratesNilPlanner(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	rs.planning.planner = nil
	rs.detection.detectionPipe = detection.NewDetectionPipeline(detection.AprilTagConfig{Family: "tag36h11"})
	rs.registerDemoCallbacks()

	rs.web.webServer.Callbacks.OnObstaclesChanged([]planning.Obstacle{
		planning.NewRectObstacle("a", [2]float64{0, 0}, [2]float64{0.1, 0.1}),
	})
}

func TestDrawLineOnRGBA_ZeroLengthDoesNotPanic(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	red := color.RGBA{R: 255, A: 255}

	drawLineOnRGBA(img, image.Pt(10, 10), image.Pt(10, 10), red, 3)

	if img.RGBAAt(10, 10) != red {
		t.Errorf("pixel at (10,10) = %v, want %v", img.RGBAAt(10, 10), red)
	}
}
