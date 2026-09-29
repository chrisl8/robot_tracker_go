//go:build gocv

package main

import (
	"image"
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/config"
	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
	"github.com/chrisl8/robot_tracker_go/internal/ui"
)

const demoAutonomyTagID = 7

// newDemoAutonomyRig builds a RobotSystem with the same subsystems
// Initialize() would wire up (planner, tracker, calibrated position
// estimator, path executor, command queue), but without opening a camera,
// an Arduino, or a real HTTP listener -- enough to call ProcessDemoFrame
// directly and observe planner/heading/command-queue side effects.
func newDemoAutonomyRig(t *testing.T) *RobotSystem {
	t.Helper()
	est, err := position.NewPositionEstimator("")
	if err != nil {
		t.Fatalf("NewPositionEstimator: %v", err)
	}
	est.GetHomography().SetFromValues(0.01, 0, 0, 0, 0.01, 0, 0, 0, 1) // 100 px per metre

	cfg := &config.Config{Robots: []config.RobotConfig{
		{TagID: demoAutonomyTagID, Diameter: 0.3},
	}}

	rs := NewRobotSystem(cfg)
	rs.demoMode = true
	rs.tracking.tracker = tracking.NewByteTrack(nil)
	rs.planning.planner = planning.NewPlanner(nil)
	rs.position.positionEst = est
	rs.io.pathExecutor = controller.NewPathExecutor(0.15, 0.5)
	rs.io.waypointThreshold = 0.1
	rs.io.commandQueue = controller.NewCommandQueue(nil, 100, 0) // not Start()ed: Enqueue is safe, nothing consumes it
	rs.web.webServer = ui.NewWebServer(":0")                  // not Start()ed: no real listener needed for this test
	rs.control.controlMode = ControlModeAutonomous
	return rs
}

// demoTagAt returns a single synthetic AprilTag detection for
// demoAutonomyTagID, centered at (cx, cy) with a fixed size, matching the
// shape ProcessDemoFrame expects from generateDemoTags.
func demoTagAt(cx, cy float64) detection.AprilTag {
	const size = 60.0
	return detection.AprilTag{
		TagID:  demoAutonomyTagID,
		Family: "tag36h11",
		Corners: [4][2]float64{
			{cx - size, cy - size},
			{cx + size, cy - size},
			{cx + size, cy + size},
			{cx - size, cy + size},
		},
		CenterX: cx,
		CenterY: cy,
		Size:    size * 2,
	}
}

// TestProcessDemoFrame_RegistersRobotAndRunsAutonomousControl guards
// against the demo-mode divergence described in
// docs/archived/code-review-2026-09-27.md tech-debt #1: ProcessDemoFrame drew and
// broadcast tracks but never called AddRobot/computeTrackHeading/
// executeAutonomousControl, so demo robots never got a path, a heading, or
// autonomous driving even with the UI switched to Autonomous. This feeds a
// stationary demo tag through several frames (enough for ByteTrack to
// confirm the track) with a goal set far away, and asserts all three now
// ran.
func TestProcessDemoFrame_RegistersRobotAndRunsAutonomousControl(t *testing.T) {
	rs := newDemoAutonomyRig(t)

	// Goal set before the robot is registered: AddRobot's first call plans
	// a path immediately once it sees this stored goal (see
	// internal/planning/planner.go's AddRobot).
	rs.planning.planner.SetGoal(demoAutonomyTagID, [2]float64{5.2, 5.4})

	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	tag := demoTagAt(320, 240)

	// ByteTrack confirms a track after 3 hits; run a few extra frames so
	// heading/AddRobot/autonomous-control all see a Confirmed track.
	for i := 0; i < 5; i++ {
		rs.ProcessDemoFrame(img, i, []detection.AprilTag{tag})
	}

	// GetClearance can't distinguish "robot never registered" from "no
	// obstacles nearby" -- both return CollisionDetector's no-obstacles
	// sentinel (1e10, internal/planning/collision.go). GetNextWaypoint
	// only returns hasPath=true once AddRobot has registered the robot and
	// planned a path against its already-stored goal (see AddRobot in
	// internal/planning/planner.go), so it's the reliable signal here.
	if _, hasPath := rs.planning.planner.GetNextWaypoint(demoAutonomyTagID); !hasPath {
		t.Errorf("AddRobot did not run: planner has no path for robot %d", demoAutonomyTagID)
	}
	if _, ok := rs.heading.lastHeading[demoAutonomyTagID]; !ok {
		t.Errorf("computeTrackHeading did not run: rs.heading.lastHeading has no entry for robot %d", demoAutonomyTagID)
	}
	if _, ok := rs.io.robotCommands[demoAutonomyTagID]; !ok {
		t.Errorf("executeAutonomousControl did not run: rs.io.robotCommands has no entry for robot %d", demoAutonomyTagID)
	}
}

// TestExecuteAutonomousControl_DemoModeGuardOnlyBlocksWhenArduinoConnected
// covers the boolean guard added alongside the demo-mode autonomy fix
// (executeAutonomousControl's "rs.demoMode && rs.io.arduino != nil &&
// rs.io.arduino.IsConnected()" check): with demo mode active but no
// Arduino wired up (the common case, and what newDemoAutonomyRig sets up),
// the guard must NOT block autonomous control -- otherwise every demo run
// would silently stop working. The case this guard actually exists to
// block -- demo mode *with* a real, connected Arduino -- requires a live
// serial connection to exercise IsConnected() honestly and isn't faked
// here; it was verified by manual inspection (see the guard's comment in
// executeAutonomousControl) rather than by this automated test.
func TestExecuteAutonomousControl_DemoModeGuardOnlyBlocksWhenArduinoConnected(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	if rs.io.arduino != nil {
		t.Fatalf("test setup: expected no Arduino wired up, got one")
	}
	rs.planning.planner.SetGoal(demoAutonomyTagID, [2]float64{5.2, 5.4})
	rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{3.2, 2.4}, 0.3)

	tagID := demoAutonomyTagID
	track := tracking.Track{
		TrackID:  1,
		TagID:    &tagID,
		State:    tracking.TrackStateConfirmed,
		WorldPos: [2]float64{3.2, 2.4},
	}

	rs.executeAutonomousControl([]tracking.Track{track})

	if _, ok := rs.io.robotCommands[demoAutonomyTagID]; !ok {
		t.Errorf("demo mode with no Arduino wired up should not be blocked by the guard, but rs.io.robotCommands was never set")
	}
}
