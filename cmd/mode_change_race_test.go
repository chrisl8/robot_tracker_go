//go:build gocv

package main

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/tracking"
)

// Leaving Auto stops the robot and clears its command. A frame that had already
// passed the mode check used to enqueue its drive command after that Stop, so
// the robot kept driving on it (until the 2 s deadman) after the operator hit
// Hold. Once SetControlMode returns, no later autonomy call may leave a drive
// command behind.
func TestSetControlMode_LeavingAuto_NoDriveCommandSurvivesInFlightFrame(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	startRigQueue(t, rs)
	rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{3.2, 2.4}, 0.3)
	rs.planning.planner.SetGoal(demoAutonomyTagID, [2]float64{5.2, 5.4})
	tracks := []tracking.Track{confirmedTrack(1, demoAutonomyTagID)}

	var calls atomic.Int64
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() { // the frame loop
		defer close(finished)
		for {
			select {
			case <-done:
				return
			default:
			}
			rs.executeAutonomousControl(tracks)
			calls.Add(1)
		}
	}()
	defer func() { close(done); <-finished }()

	for i := 0; i < 400; i++ {
		rs.SetControlMode(ControlModeAutonomous)
		time.Sleep(200 * time.Microsecond) // frames drive in Auto
		rs.SetControlMode(ControlModeIdle)

		// Let every frame that was already in flight finish, then check.
		target := calls.Load() + 2
		for calls.Load() < target {
			time.Sleep(50 * time.Microsecond)
		}
		if rs.io.commandQueue.HasActiveCommand() {
			t.Fatalf("iteration %d: a drive command was left active after the switch to Hold", i)
		}
	}
}

// An autonomy frame holds the control-state read lock while it runs, so the
// e-stop must stop the robot without first waiting for that lock.
func TestEmergencyStop_StopsRobotWhileAutonomyFrameHoldsControlLock(t *testing.T) {
	rs := newDemoAutonomyRig(t)
	startRigQueue(t, rs)

	rs.control.controlMu.RLock() // a frame in flight
	released := false
	defer func() {
		if !released {
			rs.control.controlMu.RUnlock()
		}
	}()

	stopped := make(chan struct{})
	go func() {
		rs.EmergencyStop()
		close(stopped)
	}()

	deadline := time.Now().Add(time.Second)
	for rs.io.commandQueue.IsRunning() {
		if time.Now().After(deadline) {
			t.Fatal("the e-stop did not halt the command queue while a frame held the control lock")
		}
		time.Sleep(time.Millisecond)
	}

	rs.control.controlMu.RUnlock()
	released = true
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("EmergencyStop never finished after the frame released the lock")
	}
	if !rs.IsEmergencyStopped() || rs.GetControlMode() != ControlModeIdle {
		t.Errorf("after the e-stop: stopped=%v mode=%v", rs.IsEmergencyStopped(), rs.GetControlMode())
	}
}

// The UI draws thrust animations from each track's motion state. It used to be
// written only by autonomy and never cleared, so after a Hold, an e-stop or a
// cleared goal the robot kept being drawn as driving forward.
func TestExecuteAutonomousControl_MotionStateDoesNotOutliveTheDrive(t *testing.T) {
	setup := func(t *testing.T) (*RobotSystem, []tracking.Track) {
		rs := newDemoAutonomyRig(t)
		rs.planning.planner.AddRobot(demoAutonomyTagID, [2]float64{3.2, 2.4}, 0.3)
		rs.planning.planner.SetGoal(demoAutonomyTagID, [2]float64{5.2, 2.4}) // dead ahead: heading is 0
		tracks := []tracking.Track{confirmedTrack(1, demoAutonomyTagID)}
		rs.executeAutonomousControl(tracks)
		if got := rs.io.robotCommands[demoAutonomyTagID]; got != "forward" {
			t.Fatalf("setup: motion state = %q, want forward", got)
		}
		return rs, tracks
	}

	t.Run("mode left", func(t *testing.T) {
		rs, tracks := setup(t)
		rs.SetControlMode(ControlModeIdle)
		rs.executeAutonomousControl(tracks)
		if got := rs.io.robotCommands[demoAutonomyTagID]; got == "forward" {
			t.Errorf("still %q after switching to Hold", got)
		}
	})
	t.Run("emergency stop", func(t *testing.T) {
		rs, tracks := setup(t)
		rs.EmergencyStop()
		rs.executeAutonomousControl(tracks)
		if got := rs.io.robotCommands[demoAutonomyTagID]; got == "forward" {
			t.Errorf("still %q after the e-stop", got)
		}
	})
	t.Run("goal cleared", func(t *testing.T) {
		rs, tracks := setup(t)
		rs.planning.planner.CompletePath(demoAutonomyTagID) // the operator cleared the destination
		rs.executeAutonomousControl(tracks)
		if got := rs.io.robotCommands[demoAutonomyTagID]; got == "forward" {
			t.Errorf("still %q after the goal was cleared", got)
		}
	})
}
