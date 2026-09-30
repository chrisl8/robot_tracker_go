//go:build gocv

package main

import (
	"math"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// proximityClearance is the gap (metres, robot edge to obstacle) under which
// the robot may not drive any closer to the obstacle. It may still turn in
// place and drive away, so a robot that starts, or ends up, this close can
// always get out.
const proximityClearance = 0.08

// proximityProbeStep is how far (metres) ahead movesTowardObstacle looks.
const proximityProbeStep = 0.05

// movesTowardObstacle reports whether cmd would take the robot closer to an
// obstacle: a Forward or Backward move whose position one probe step along the
// heading has less clearance than now. Turns and Stop never change the gap.
func (rs *RobotSystem) movesTowardObstacle(robotID int, cmd controller.Command, pos position.Point2D, heading, clearance float64) bool {
	dir := heading
	switch cmd {
	case controller.CommandForward:
	case controller.CommandBackward:
		dir += math.Pi
	default:
		return false
	}
	probe := [2]float64{pos.X + proximityProbeStep*math.Cos(dir), pos.Y + proximityProbeStep*math.Sin(dir)}
	return rs.planning.planner.ClearanceAt(robotID, probe) < clearance
}

// executeAutonomousControl handles path-following for all tracked robots.
func (rs *RobotSystem) executeAutonomousControl(tracks []tracking.Track) {
	if rs.GetControlMode() != ControlModeAutonomous || rs.IsEmergencyStopped() {
		return
	}
	// Demo mode can fall back from a real camera that's temporarily
	// unavailable while a real Arduino stays connected (Initialize() wires
	// up Arduino/commandQueue regardless of demo vs. real-camera mode). Never
	// let synthetic demo-tag positions drive real hardware.
	if rs.demoMode && rs.io.arduino != nil && rs.io.arduino.IsConnected() {
		utils.Logf("Refusing autonomous control: demo mode is active with a real Arduino connected")
		return
	}

	commandIssued := false
	anyPath := false
	steered := false
	inView := make(map[int]bool) // robots with a confirmed track this frame
	for i := range tracks {
		track := &tracks[i]
		if track.State != tracking.TrackStateConfirmed || track.TagID == nil {
			continue
		}

		robotID := *track.TagID
		inView[robotID] = true

		if _, hasPath := rs.planning.planner.GetNextWaypoint(robotID); hasPath {
			anyPath = true
			if rs.position.positionEst == nil {
				continue
			}

			// Use the position ProcessFrame computed (it includes the robot's
			// center_offset correction and is what the planner was given), not
			// a fresh uncorrected projection of the bbox.
			worldPos := position.Point2D{X: track.WorldPos[0], Y: track.WorldPos[1]}

			// Close to an obstacle: replan from here (the planner routes out of
			// the margin), but never park the robot. Being near an obstacle is
			// not a reason to stop; only driving closer is (see below).
			clearance := rs.planning.planner.GetClearance(robotID)
			tooClose := clearance < proximityClearance
			if tooClose {
				// Replan at most once every 3 seconds to avoid thrashing
				if lastReplan, ok := rs.planning.lastReplanTime[robotID]; !ok || time.Since(lastReplan) > 3*time.Second {
					if goal, hasGoal := rs.planning.planner.GetGoal(robotID); hasGoal {
						utils.Logf("PROXIMITY: Robot %d clearance=%.3fm — replanning away from the obstacle", robotID, clearance)
						rs.planning.planner.ClearPathOnly(robotID)
						pos := [2]float64{worldPos.X, worldPos.Y}
						if newPath, ok := rs.planning.planner.PlanPath(robotID, pos, goal); ok {
							rs.planning.planner.SetPath(robotID, newPath)
							utils.Logf("Robot %d replanned: %d waypoints from (%.2f,%.2f)", robotID, len(newPath), pos[0], pos[1])
						} else {
							utils.Logf("Robot %d replan FAILED from (%.2f,%.2f) to (%.2f,%.2f)", robotID, pos[0], pos[1], goal[0], goal[1])
						}
						rs.planning.lastReplanTime[robotID] = time.Now()
					}
				}
			}

			// Check if robot is close to final destination
			if goal, hasGoal := rs.planning.planner.GetGoal(robotID); hasGoal {
				dx := worldPos.X - goal[0]
				dy := worldPos.Y - goal[1]
				distToGoal := math.Sqrt(dx*dx + dy*dy)
				if distToGoal < rs.io.waypointThreshold {
					rs.planning.planner.CompletePath(robotID)
					rs.web.webServer.ClearDestination(robotID)
					utils.Logf("Robot %d reached goal (%.2fm away), stopping", robotID, distToGoal)
					if rs.io.commandQueue != nil {
						rs.io.commandQueue.Enqueue(controller.CommandStop)
						commandIssued = true
					}
					rs.io.robotCommands[robotID] = "stopped"
					continue
				}
			}

			// Advance past any reached or overshot waypoints
			if !rs.planning.planner.AdvancePastWaypoints(robotID, [2]float64{worldPos.X, worldPos.Y}, rs.io.waypointThreshold) {
				utils.Logf("Robot %d reached final waypoint, stopping", robotID)
				if rs.io.commandQueue != nil {
					rs.io.commandQueue.Enqueue(controller.CommandStop)
					commandIssued = true
				}
				rs.io.robotCommands[robotID] = "stopped"
				continue
			}

			// Get updated waypoint after advancing
			waypoint, stillHasPath := rs.planning.planner.GetNextWaypoint(robotID)
			if !stillHasPath {
				continue
			}

			// Heading-based steering: turn to face waypoint, then drive forward
			dx := waypoint[0] - worldPos.X
			dy := waypoint[1] - worldPos.Y
			bearingToWaypoint := math.Atan2(dy, dx)

			if rs.io.pathExecutor != nil && rs.io.commandQueue != nil {
				delta := rs.heading.headingDelta[robotID]
				steered = true
				cmd := rs.io.pathExecutor.BearingToCommand(track.Heading, bearingToWaypoint, delta)
				if tooClose && rs.movesTowardObstacle(robotID, cmd, worldPos, track.Heading, clearance) {
					utils.Debugf("PROXIMITY: Robot %d clearance=%.3fm, %c would close in: stopping", robotID, clearance, byte(cmd))
					cmd = controller.CommandStop
				}
				rs.io.commandQueue.Enqueue(cmd)
				commandIssued = true
				switch cmd {
				case controller.CommandForward:
					rs.io.robotCommands[robotID] = "forward"
				case controller.CommandBackward:
					rs.io.robotCommands[robotID] = "backward"
				case controller.CommandLeft:
					rs.io.robotCommands[robotID] = "rotating_left"
				case controller.CommandRight:
					rs.io.robotCommands[robotID] = "rotating_right"
				case controller.CommandStop:
					rs.io.robotCommands[robotID] = "stopped"
				}
			}
		}
	}

	// A robot that has a goal but was not confirmed in view this frame (tag
	// lost, occluded, or never seen) must not keep driving on the last command:
	// stop it now rather than waiting for the deadman.
	for _, robotID := range rs.planning.planner.RobotsWithGoals() {
		if inView[robotID] {
			continue
		}
		if rs.io.commandQueue != nil && rs.io.robotCommands[robotID] != "stopped" {
			utils.Logf("Robot %d has a goal but is not confirmed in view: stopping", robotID)
		}
		if rs.io.commandQueue != nil {
			rs.io.commandQueue.Enqueue(controller.CommandStop)
		}
		rs.io.robotCommands[robotID] = "stopped"
	}

	// Steering state (burst/wait phase, turn hysteresis) belongs to one
	// continuous run; a frame with no steering ends it.
	if !steered && rs.io.pathExecutor != nil {
		rs.io.pathExecutor.Reset()
	}

	// Only clear the active command when no robot is following a path. Doing
	// it per robot let a path-less robot wipe the command just issued for the
	// robot that does have one.
	if !anyPath && rs.io.commandQueue != nil && rs.io.commandQueue.IsRunning() {
		rs.io.commandQueue.ClearActiveCommand()
	}

	// Safety: stop re-sending stale commands when tracking is lost for too long.
	if commandIssued {
		rs.io.lastCommandTime = time.Now()
	} else if rs.io.commandQueue != nil && time.Since(rs.io.lastCommandTime) > rs.io.trackingLostTimeout {
		rs.io.commandQueue.ClearActiveCommand()
	}
}
