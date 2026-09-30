//go:build gocv

package main

import (
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/camera"
	"github.com/chrisl8/robot_tracker_go/internal/config"
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
	rs.cfg.AddRobotIfMissing(config.RobotConfig{TagID: 8, Name: "second", Diameter: 0.3})
	rs.registerWebServerCallbacks()
	rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{1, 1}, 0.3)
	rs.planning.planner.AddRobot(8, [2]float64{2, 2}, 0.3)

	set := rs.web.webServer.Callbacks.OnDestinationSet
	for _, dest := range []struct {
		robot int
		pos   [2]float64
	}{{demoAutonomyTagID, [2]float64{300, 300}}, {8, [2]float64{400, 400}}} {
		if err := set(dest.robot, dest.pos); err != nil {
			t.Fatalf("OnDestinationSet(%d) = %v", dest.robot, err)
		}
	}

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

// An uncalibrated system must refuse a destination with an error, so the web
// layer can tell the operator instead of showing a goal the planner never got.
func TestOnDestinationSet_RefusesWhenNotCalibrated(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	rs.registerWebServerCallbacks()
	rs.position.positionEst = nil

	err := rs.web.webServer.Callbacks.OnDestinationSet(demoAutonomyTagID, [2]float64{100, 100})

	if err == nil {
		t.Fatal("expected an error when the camera is not calibrated")
	}
	if len(rs.planning.planner.RobotsWithGoals()) != 0 {
		t.Error("no goal should have been set")
	}
}

// Only configured robots may be given goals: an unknown tag would otherwise be
// planned and driven with a default body size.
func TestOnDestinationSet_RejectsUnconfiguredRobot(t *testing.T) {
	rs := newDemoAutonomyRig(t) // configures only demoAutonomyTagID
	rs.registerWebServerCallbacks()

	err := rs.web.webServer.Callbacks.OnDestinationSet(99, [2]float64{100, 100})

	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("err = %v, want a 'not configured' error", err)
	}
	if len(rs.planning.planner.RobotsWithGoals()) != 0 {
		t.Error("no goal should be set for an unconfigured robot")
	}
	if err := rs.web.webServer.Callbacks.OnDestinationSet(demoAutonomyTagID, [2]float64{100, 100}); err != nil {
		t.Errorf("a configured robot should be accepted, got %v", err)
	}
}

// The demo's synthetic robots become real configured robots, so they follow the
// same rules as a real run; an explicit config entry is never overridden.
func TestRegisterDemoRobots(t *testing.T) {
	cfg := &config.Config{Robots: []config.RobotConfig{{TagID: 1, Name: "real", Diameter: 0.55}}}
	rs := NewRobotSystem(cfg)

	rs.registerDemoRobots()
	rs.registerDemoRobots() // idempotent

	if len(cfg.Robots) != demoTagCount {
		t.Fatalf("robots = %d, want %d (no duplicates)", len(cfg.Robots), demoTagCount)
	}
	for id := 1; id <= demoTagCount; id++ {
		if cfg.GetRobotByTagID(id) == nil {
			t.Errorf("demo tag %d is not a configured robot", id)
		}
	}
	if got := cfg.GetRobotByTagID(1); got == nil || got.Name != "real" || got.Diameter != 0.55 {
		t.Errorf("explicitly configured tag 1 was overridden: %+v", got)
	}
	// generateDemoTags must only emit tags the registration covers.
	for _, tag := range generateDemoTags(640, 480, 10) {
		if cfg.GetRobotByTagID(tag.TagID) == nil {
			t.Errorf("generated demo tag %d is not registered as a robot", tag.TagID)
		}
	}
}

func TestRegisterDemoRobots_NilConfigIsSafe(t *testing.T) {
	NewRobotSystem(nil).registerDemoRobots() // must not panic
}

// A robot with a goal whose track is no longer confirmed (tag lost or
// occluded) must be told to stop straight away, not left on its last command
// until the deadman expires.
func TestExecuteAutonomousControl_StopsRobotWhoseTrackIsLost(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	startRigQueue(t, rs)
	rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{3.2, 2.4}, 0.3)
	rs.planning.planner.SetGoal(demoAutonomyTagID, [2]float64{5.2, 5.4})
	rs.io.robotCommands[demoAutonomyTagID] = "forward"

	lost := confirmedTrack(1, demoAutonomyTagID)
	lost.State = tracking.TrackStateLost
	rs.executeAutonomousControl([]tracking.Track{lost})

	if got := rs.io.robotCommands[demoAutonomyTagID]; got != "stopped" {
		t.Errorf("motion state = %q, want stopped for a robot that is not in view", got)
	}
}

// Same when the tracker has dropped the track altogether.
func TestExecuteAutonomousControl_StopsRobotWithGoalAndNoTrack(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	startRigQueue(t, rs)
	rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{3.2, 2.4}, 0.3)
	rs.planning.planner.SetGoal(demoAutonomyTagID, [2]float64{5.2, 5.4})
	rs.io.robotCommands[demoAutonomyTagID] = "forward"

	rs.executeAutonomousControl(nil)

	if got := rs.io.robotCommands[demoAutonomyTagID]; got != "stopped" {
		t.Errorf("motion state = %q, want stopped", got)
	}
}

// A frame with no steering ends the run: a turn burst left over from before
// must not be finished (or a drive-while-waiting continued) in the next run.
func TestExecuteAutonomousControl_ResetsSteeringWhenNotSteering(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	startRigQueue(t, rs)
	rs.io.pathExecutor.BurstFrames = 3
	rs.io.pathExecutor.BearingToCommand(0, 60*math.Pi/180, 0) // starts a turn burst

	rs.executeAutonomousControl(nil) // nothing to steer this frame

	if got := rs.io.pathExecutor.BearingToCommand(0, 0, 0); got != controller.CommandForward {
		t.Errorf("first command of the next run = %q, want forward, not the stale burst", got)
	}
}

func TestIsRepeatFrame(t *testing.T) {
	tests := []struct {
		name    string
		lastSeq uint64
		frame   *camera.Frame
		want    bool
	}{
		{"same numbered frame again", 7, &camera.Frame{Seq: 7}, true},
		{"newer frame", 7, &camera.Frame{Seq: 8}, false},
		{"first frame", 0, &camera.Frame{Seq: 1}, false},
		{"unnumbered source is never a repeat", 0, &camera.Frame{Seq: 0}, false},
		{"unnumbered after numbered", 5, &camera.Frame{Seq: 0}, false},
		{"nil frame", 3, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRepeatFrame(tt.lastSeq, tt.frame); got != tt.want {
				t.Errorf("isRepeatFrame(%d, %+v) = %v, want %v", tt.lastSeq, tt.frame, got, tt.want)
			}
		})
	}
}

// A recalibration replaces the pixel->floor mapping, so a goal and path made
// under the old one are in the wrong frame. They must be released rather than
// followed (the planner would otherwise steer by stale waypoints until the next
// replan, and replan from a stale robot position).
func TestOnCalibrationComplete_ReleasesGoalsFromTheOldFrame(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	rs.registerWebServerCallbacks()
	rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{1, 1}, 0.3)
	rs.planning.planner.SetGoal(demoAutonomyTagID, [2]float64{3, 3})
	if _, ok := rs.planning.planner.GetNextWaypoint(demoAutonomyTagID); !ok {
		t.Fatal("setup: expected a path to the goal")
	}

	calibFile := filepath.Join(t.TempDir(), "calibration.yaml")
	const calib = "homography:\n  - [0.02, 0, 0]\n  - [0, 0.02, 0]\n  - [0, 0, 1]\nworld_scale: 50\n"
	if err := os.WriteFile(calibFile, []byte(calib), 0o600); err != nil {
		t.Fatal(err)
	}
	rs.web.webServer.Callbacks.OnCalibrationComplete(calibFile)

	if goals := rs.planning.planner.RobotsWithGoals(); len(goals) != 0 {
		t.Errorf("goals still set after recalibration: %v", goals)
	}
	if wp, ok := rs.planning.planner.GetNextWaypoint(demoAutonomyTagID); ok {
		t.Errorf("robot still has a path after recalibration (next waypoint %v)", wp)
	}
}
