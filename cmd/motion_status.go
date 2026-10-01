//go:build gocv

package main

import (
	"sync"

	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/ui"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// motionInputs is everything that decides what the operator is told about the
// robot's motion. Gathering it is publishMotion's job; deciding is
// computeMotion's, so the priority order can be tested without a rig.
type motionInputs struct {
	estop         bool
	cameraStalled bool
	arduinoUp     bool
	mode          ControlMode
	robotLink     controller.RobotLink
	demoRefused   bool // demo tags with a real Arduino attached: autonomy refuses to drive

	hasGoal       bool
	inView        bool   // the goal robot has a confirmed track this frame
	hasPath       bool   // the goal robot has a path to follow
	cmd           string // what autonomy is doing with it this frame ("forward", "stopped", ...)
	proximityHeld bool   // stopped because the next move would close in on an obstacle
	lastStop      string // why the last goal ended ("Reached the goal"), shown while there is no goal
}

// Motion status codes. The first group is "the robot is being driven"; the UI
// treats them alike.
const (
	motionDriving = "driving"
	motionTurning = "turning"
	motionPausing = "pausing"
)

// computeMotion answers "why is the robot (not) moving right now?". The checks
// run from the most fundamental cause to the most specific, and the first one
// that applies wins: an e-stop explains the stillness whatever else is true.
func computeMotion(in motionInputs) ui.MotionStatus {
	m := func(code, severity, text string) ui.MotionStatus {
		return ui.MotionStatus{Code: code, Severity: severity, Text: text}
	}
	switch {
	case in.estop:
		return m("estop", "error", "Emergency stop is active: clear it to move")
	case in.cameraStalled:
		return m("camera_stalled", "error", "No video from the camera: the robot is halted")
	case !in.arduinoUp:
		return m("no_controller", "error", "Controller (Arduino) is disconnected")
	case in.mode == ControlModeIdle && in.hasGoal:
		return m("mode_hold", "warn", "Hold mode: a goal is set but autonomy is off. Switch to Autonomous")
	case in.mode == ControlModeIdle:
		return m("mode_hold", "info", "Hold mode: the robot is not being driven")
	case in.mode == ControlModeManual && in.hasGoal:
		return m("mode_manual", "warn", "Manual mode: a goal is set but the robot only moves on your keys. Switch to Autonomous")
	case in.mode == ControlModeManual:
		return m("mode_manual", "info", "Manual mode: the robot moves only on your keys")
	case in.demoRefused:
		return m("demo_refused", "error", "Demo mode with a real Arduino connected: autonomy will not drive")
	case in.robotLink == controller.RobotLinkSilent:
		return m("robot_silent", "error", "The robot is not answering: off or out of radio range")
	case !in.hasGoal:
		text := "No goal set"
		if in.lastStop != "" {
			text += " (" + in.lastStop + ")"
		}
		return m("no_goal", "info", text)
	case !in.inView:
		return m("not_in_view", "error", "Goal set, but the robot's tag is not seen by the camera")
	case !in.hasPath:
		return m("no_path", "error", "Goal set, but there is no path: it is outside the camera view or blocked")
	case in.proximityHeld:
		return m("proximity", "warn", "Holding: the next move would go closer to an obstacle")
	}
	switch in.cmd {
	case "forward":
		return m(motionDriving, "ok", "Driving forward toward the goal")
	case "backward":
		return m(motionDriving, "ok", "Backing up toward the goal")
	case "rotating_left", "rotating_right":
		return m(motionTurning, "ok", "Turning to face the next waypoint")
	}
	return m(motionPausing, "ok", "Pausing between steering bursts")
}

// motionLogKey merges the codes that flip from frame to frame while the robot
// is being driven, so the log records driving starting and stopping, not every
// change of steering.
func motionLogKey(code string) string {
	switch code {
	case motionDriving, motionTurning, motionPausing:
		return "moving"
	}
	return code
}

// lastStopNote remembers why the most recent goal ended, for the status shown
// while there is no goal. It is written by the frame loop and the HTTP handlers.
type lastStopNote struct {
	mu   sync.Mutex
	text string
}

func (n *lastStopNote) set(text string) {
	n.mu.Lock()
	n.text = text
	n.mu.Unlock()
}

func (n *lastStopNote) get() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.text
}

// publishMotion gathers the inputs, decides, logs a change and hands the
// result to the UI. Called from the frame loop at the end of
// executeAutonomousControl, which holds controlMu for reading, so it reads
// the control fields directly.
func (rs *RobotSystem) publishMotion(rep autonomyReport) {
	if rs.web.webServer == nil {
		return
	}
	rs.io.lastAutonomy = rep
	in := motionInputs{
		estop:         rs.control.emergencyStopped,
		arduinoUp:     rs.io.arduino != nil && rs.io.arduino.IsConnected(),
		mode:          rs.control.controlMode,
		robotLink:     rs.io.lastRobotLink,
		demoRefused:   rep.demoRefused,
		hasGoal:       rep.hasGoal,
		inView:        rep.inView,
		hasPath:       rep.hasPath,
		cmd:           rep.cmd,
		proximityHeld: rep.proximityHeld,
		lastStop:      rs.io.lastStop.get(),
	}
	status := computeMotion(in)
	if prev := rs.io.lastMotionCode; motionLogKey(prev) != motionLogKey(status.Code) {
		utils.Logf("Motion: %q -> %q: %s", prev, status.Code, status.Text)
	}
	rs.io.lastMotionCode = status.Code
	rs.web.webServer.SetMotionStatus(status)
}
