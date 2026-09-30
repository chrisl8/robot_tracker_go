//go:build gocv

package main

import (
	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

type ControlMode int

const (
	ControlModeIdle ControlMode = iota
	ControlModeManual
	ControlModeAutonomous
)

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

func (rs *RobotSystem) GetControlMode() ControlMode {
	rs.control.controlMu.RLock()
	defer rs.control.controlMu.RUnlock()
	return rs.control.controlMode
}

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

func (rs *RobotSystem) IsEmergencyStopped() bool {
	rs.control.controlMu.RLock()
	defer rs.control.controlMu.RUnlock()
	return rs.control.emergencyStopped
}

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
