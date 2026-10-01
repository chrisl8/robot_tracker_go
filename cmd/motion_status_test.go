//go:build gocv

package main

import (
	"strings"
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
)

// driveReady is a robot that is being driven: every blocker is clear.
func driveReady() motionInputs {
	return motionInputs{
		arduinoUp: true,
		mode:      ControlModeAutonomous,
		robotLink: controller.RobotLinkAlive,
		hasGoal:   true,
		inView:    true,
		hasPath:   true,
		cmd:       "forward",
	}
}

func TestComputeMotion(t *testing.T) {
	for _, tc := range []struct {
		name         string
		change       func(*motionInputs)
		code, sev    string
		textContains string
	}{
		{"driving", func(*motionInputs) {}, motionDriving, "ok", "forward"},
		{"backing", func(in *motionInputs) { in.cmd = "backward" }, motionDriving, "ok", "Backing"},
		{"turning", func(in *motionInputs) { in.cmd = "rotating_left" }, motionTurning, "ok", "Turning"},
		{"pausing", func(in *motionInputs) { in.cmd = "stopped" }, motionPausing, "ok", "Pausing"},
		{"proximity", func(in *motionInputs) { in.proximityHeld = true }, "proximity", "warn", "obstacle"},
		{"no path", func(in *motionInputs) { in.hasPath = false }, "no_path", "error", "no path"},
		{"not in view", func(in *motionInputs) { in.inView = false; in.hasPath = false }, "not_in_view", "error", "tag"},
		{"no goal", func(in *motionInputs) { *in = motionInputs{arduinoUp: true, mode: ControlModeAutonomous} }, "no_goal", "info", "No goal set"},
		{"no goal with last stop", func(in *motionInputs) {
			*in = motionInputs{arduinoUp: true, mode: ControlModeAutonomous, lastStop: "reached the goal"}
		}, "no_goal", "info", "(reached the goal)"},
		{"robot silent", func(in *motionInputs) { in.robotLink = controller.RobotLinkSilent }, "robot_silent", "error", "not answering"},
		{"robot link unknown is not a blocker", func(in *motionInputs) { in.robotLink = controller.RobotLinkUnknown }, motionDriving, "ok", ""},
		{"demo refused", func(in *motionInputs) { in.demoRefused = true }, "demo_refused", "error", "Demo"},
		{"hold with goal", func(in *motionInputs) { in.mode = ControlModeIdle }, "mode_hold", "warn", "Switch to Autonomous"},
		{"hold no goal", func(in *motionInputs) { in.mode = ControlModeIdle; in.hasGoal = false }, "mode_hold", "info", "Hold"},
		{"manual with goal", func(in *motionInputs) { in.mode = ControlModeManual }, "mode_manual", "warn", "Switch to Autonomous"},
		{"manual no goal", func(in *motionInputs) { in.mode = ControlModeManual; in.hasGoal = false }, "mode_manual", "info", "Manual"},
		{"no controller", func(in *motionInputs) { in.arduinoUp = false }, "no_controller", "error", "Arduino"},
		{"camera stalled", func(in *motionInputs) { in.cameraStalled = true }, "camera_stalled", "error", "video"},
		{"estop", func(in *motionInputs) { in.estop = true }, "estop", "error", "Emergency"},
		// Priority: each of these has a lower-priority problem too.
		{"estop beats everything", func(in *motionInputs) {
			in.estop, in.cameraStalled, in.arduinoUp, in.mode = true, true, false, ControlModeIdle
		}, "estop", "error", ""},
		{"camera beats controller", func(in *motionInputs) { in.cameraStalled, in.arduinoUp = true, false }, "camera_stalled", "error", ""},
		{"controller beats mode", func(in *motionInputs) { in.arduinoUp, in.mode = false, ControlModeIdle }, "no_controller", "error", ""},
		{"mode beats missing path", func(in *motionInputs) { in.mode, in.hasPath = ControlModeIdle, false }, "mode_hold", "warn", ""},
		{"silent robot beats no goal", func(in *motionInputs) { in.robotLink, in.hasGoal = controller.RobotLinkSilent, false }, "robot_silent", "error", ""},
		{"not in view beats no path", func(in *motionInputs) { in.inView, in.hasPath = false, false }, "not_in_view", "error", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := driveReady()
			tc.change(&in)
			got := computeMotion(in)
			if got.Code != tc.code || got.Severity != tc.sev {
				t.Errorf("got code=%q severity=%q (%s), want code=%q severity=%q", got.Code, got.Severity, got.Text, tc.code, tc.sev)
			}
			if tc.textContains != "" && !strings.Contains(got.Text, tc.textContains) {
				t.Errorf("text %q does not contain %q", got.Text, tc.textContains)
			}
		})
	}
}

// executeAutonomousControl publishes a status every frame, whichever way it
// returns. The rig has no Arduino, so that is the first thing it reports; an
// e-stop outranks it.
func TestExecuteAutonomousControl_PublishesMotionEveryFrame(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	rs.planning.planner.SetGoal(demoAutonomyTagID, [2]float64{1, 1})

	rs.executeAutonomousControl([]tracking.Track{})
	if got := rs.io.lastMotionCode; got != "no_controller" {
		t.Errorf("autonomous, goal, no Arduino: lastMotionCode = %q, want no_controller", got)
	}

	rs.control.emergencyStopped = true // the early-return path
	rs.executeAutonomousControl([]tracking.Track{})
	if got := rs.io.lastMotionCode; got != "estop" {
		t.Errorf("e-stopped: lastMotionCode = %q, want estop", got)
	}
}

func TestLastStopNote(t *testing.T) {
	var n lastStopNote
	if n.get() != "" {
		t.Fatal("zero value should be empty")
	}
	n.set("reached the goal")
	if got := n.get(); got != "reached the goal" {
		t.Errorf("got %q", got)
	}
}
