//go:build gocv

package main

import (
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/controller"
)

func robotLinkRig(t *testing.T) (rs *RobotSystem, setGoal func(), hasGoal func() bool) {
	t.Helper()
	rs = newDemoAutonomyRig(t)
	rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{3.2, 2.4}, 0.3)
	setGoal = func() { rs.planning.planner.SetGoal(demoAutonomyTagID, [2]float64{5.2, 5.4}) }
	hasGoal = func() bool {
		_, ok := rs.planning.planner.GetGoal(demoAutonomyTagID)
		return ok
	}
	return rs, setGoal, hasGoal
}

// A switched-off robot is still visible to the camera, so autonomy kept commanding
// it and it walked toward the old goal the moment it was switched back on. The
// goal is released once, when the robot goes silent.
func TestApplyRobotLink_ReleasesGoalsWhenRobotGoesSilent(t *testing.T) {
	rs, setGoal, hasGoal := robotLinkRig(t)

	setGoal()
	rs.applyRobotLink(controller.RobotLinkUnknown) // just connected
	rs.applyRobotLink(controller.RobotLinkAlive)
	if !hasGoal() {
		t.Fatal("an alive robot's goal must be kept")
	}

	rs.applyRobotLink(controller.RobotLinkSilent)
	if hasGoal() {
		t.Error("the goal must be released when the robot stops answering")
	}

	// Still silent: only the transition releases, so a goal set now is kept.
	setGoal()
	rs.applyRobotLink(controller.RobotLinkSilent)
	if !hasGoal() {
		t.Error("staying silent must not keep releasing goals")
	}

	// Coming back does not release anything; going silent again does.
	rs.applyRobotLink(controller.RobotLinkAlive)
	if !hasGoal() {
		t.Error("the robot coming back must not release the goal")
	}
	rs.applyRobotLink(controller.RobotLinkSilent)
	if hasGoal() {
		t.Error("the second outage must release the goal again")
	}
}

// A robot that never answers after connecting goes unknown -> silent directly.
func TestApplyRobotLink_ReleasesGoalsWhenRobotNeverAnswered(t *testing.T) {
	rs, setGoal, hasGoal := robotLinkRig(t)
	setGoal()

	rs.applyRobotLink(controller.RobotLinkUnknown)
	rs.applyRobotLink(controller.RobotLinkSilent)

	if hasGoal() {
		t.Error("the goal must be released when the robot never answered")
	}
}

// The Arduino dropping (alive or silent -> unknown) releases the goal: found on the
// hardware, the robot otherwise resumed driving by itself when the cable was replugged.
func TestApplyRobotLink_ReleasesGoalsWhenArduinoDrops(t *testing.T) {
	for _, from := range []controller.RobotLink{controller.RobotLinkAlive, controller.RobotLinkSilent} {
		t.Run(string(from), func(t *testing.T) {
			rs, setGoal, hasGoal := robotLinkRig(t)
			rs.applyRobotLink(from)
			setGoal()

			rs.applyRobotLink(controller.RobotLinkUnknown)

			if hasGoal() {
				t.Error("the goal must be released when the Arduino connection drops")
			}
		})
	}
}

// Unknown at startup (never connected, or demo mode with no controller) is not a drop.
func TestApplyRobotLink_InitialUnknownDoesNotReleaseGoals(t *testing.T) {
	rs, setGoal, hasGoal := robotLinkRig(t)
	setGoal()

	rs.applyRobotLink(controller.RobotLinkUnknown)

	if !hasGoal() {
		t.Error("the first unknown link is not a disconnect and must not release the goal")
	}
}

func TestUpdateRobotLink_WithoutControllerIsUnknown(t *testing.T) {
	rs, _, _ := robotLinkRig(t)
	rs.io.arduino = nil

	info := rs.updateRobotLink()

	if info.Link != controller.RobotLinkUnknown {
		t.Errorf("link = %q, want unknown when there is no controller (demo mode)", info.Link)
	}
}
