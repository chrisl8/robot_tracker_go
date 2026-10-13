//go:build gocv

package main

import (
	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// ControlMode is who is driving the robot: nobody (hold), the operator's keys (manual), or the planner (autonomous).
type ControlMode int

const (
	// ControlModeIdle is "hold": the robot is not driven. It is the default, and what an unknown mode name parses to.
	ControlModeIdle ControlMode = iota
	// ControlModeManual drives the robot only from the operator's key presses.
	ControlModeManual
	// ControlModeAutonomous drives the robot along planned paths to its goal.
	ControlModeAutonomous
)

// String returns the mode's API name: "hold", "manual" or "autonomous".
func (m ControlMode) String() string {
	switch m {
	case ControlModeManual:
		return "manual"
	case ControlModeAutonomous:
		return "autonomous"
	default:
		return "hold"
	}
}

// ParseControlMode converts an API mode name to a ControlMode. Anything unrecognised is ControlModeIdle, the safe choice.
func ParseControlMode(s string) ControlMode {
	switch s {
	case "manual":
		return ControlModeManual
	case "autonomous":
		return ControlModeAutonomous
	case "hold":
		return ControlModeIdle
	default:
		return ControlModeIdle
	}
}

// GetControlMode returns the current control mode.
func (rs *RobotSystem) GetControlMode() ControlMode {
	rs.control.controlMu.RLock()
	defer rs.control.controlMu.RUnlock()
	return rs.control.controlMode
}

// SetControlMode switches the control mode. It is ignored while the emergency stop is active. Leaving manual or autonomous clears the active command and stops the robot.
func (rs *RobotSystem) SetControlMode(mode ControlMode) {
	rs.control.controlMu.Lock()
	defer rs.control.controlMu.Unlock()
	if rs.control.emergencyStopped {
		return
	}
	prev := rs.control.controlMode
	rs.control.controlMode = mode
	utils.Logf("Control mode changed to: %s", mode)

	// When leaving Manual or Autonomous, clear active command and stop the robot
	if prev != mode && (prev == ControlModeManual || prev == ControlModeAutonomous) {
		if rs.io.commandQueue != nil {
			rs.io.commandQueue.ClearActiveCommand()
			rs.io.commandQueue.Enqueue(controller.CommandStop)
		}
	}
}

// IsEmergencyStopped reports whether the emergency stop is active.
func (rs *RobotSystem) IsEmergencyStopped() bool {
	rs.control.controlMu.RLock()
	defer rs.control.controlMu.RUnlock()
	return rs.control.emergencyStopped
}

// EmergencyStop halts the robot at once and refuses all movement until ClearEmergencyStop. The Stop goes straight to the Arduino so it does not wait for the command queue or an autonomy frame.
func (rs *RobotSystem) EmergencyStop() {
	// Stop the robot before touching controlMu: an autonomy frame holds its
	// read lock while it runs (see executeAutonomousControl), and an e-stop must
	// not wait for one. Halting the queue first is what makes that safe: a frame
	// still in flight can only enqueue a movement command the queue drops.
	// Send stop directly to Arduino, bypassing queue for reliability
	if rs.io.arduino != nil {
		_ = rs.io.arduino.SendCommand(controller.CommandStop)
	}
	if rs.io.commandQueue != nil {
		rs.io.commandQueue.EmergencyStop()
	}

	rs.control.controlMu.Lock()
	rs.control.emergencyStopped = true
	rs.control.controlMode = ControlModeIdle
	rs.control.controlMu.Unlock()
	utils.Logf("EMERGENCY STOP activated")
}

// ClearEmergencyStop ends the emergency stop and restarts the command queue. The robot stays stopped until it is driven again.
func (rs *RobotSystem) ClearEmergencyStop() {
	rs.control.controlMu.Lock()
	rs.control.emergencyStopped = false
	rs.control.controlMu.Unlock()

	// Restart the command queue so it can accept commands again
	if rs.io.commandQueue != nil {
		rs.io.commandQueue.Start()
	}
	utils.Logf("Emergency stop cleared")
}
