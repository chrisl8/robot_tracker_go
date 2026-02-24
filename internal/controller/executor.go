package controller

import (
	"math"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

type Velocity struct {
	VX float64
	VY float64
}

// PathExecutor converts bearing-to-waypoint into discrete robot commands (F/B/L/R/S).
//
// Key design decisions driven by real-world AprilTag heading noise measurements:
//
//   - AprilTag heading noise: 0-3° when stationary, 5-15° during forward driving,
//     20-70° transient spikes during rotation (corner detection confusion).
//
//   - Hysteresis on forward/turn: A single forward threshold (e.g. 30° or 45°)
//     causes oscillation because heading noise constantly crosses any single boundary.
//     Instead, we use two thresholds: ForwardThresholdDeg (tight, e.g. 20°) to ENTER
//     forward mode, and ForwardThresholdDeg+10° (wide, e.g. 30°) to EXIT forward mode.
//     The 10° dead zone absorbs typical heading noise without triggering mode switches.
//
//   - Burst/drive pattern: The robot's heading in the camera feed lags physical rotation
//     by 1-3 frames. Sending continuous turn commands causes overshoot. Instead, send
//     a short burst of turn commands, then drive forward while the heading catches up.
//     This mimics how a human controls the robot: brief turn taps interleaved with
//     forward motion, never stopping between corrections.
//
//   - Nudge corrections: While driving forward, if the heading drifts past half the
//     exit threshold (~15°), a single turn command is injected to correct course.
//     A cooldown prevents over-correcting from noise. This creates three forward-mode
//     zones: 0-15° = pure forward, 15-30° = forward with periodic nudges, >30° = turn.
//
//   - Spin detection: Large heading deltas (>20°/frame) usually indicate AprilTag
//     detection noise during rotation, not actual robot spinning. The robot stops
//     to let detection stabilize.
type PathExecutor struct {
	maxSpeed            float64
	turnSpeed           float64
	SpinThresholdDeg    float64 // heading delta (°/frame) above which robot is "spinning"
	BurstFrames         int     // frames of continuous turning per burst
	MaxWaitFrames       int     // frames to wait after burst for heading to update
	ForwardThresholdDeg float64 // angle diff (°) to ENTER forward mode (hysteresis: exits at +10°)
	isTurning           bool    // hysteresis state: true = actively turning, widens forward entry
	nudgeCooldown       int     // frames remaining before next nudge correction allowed
	waitingForUpdate    bool
	waitHeading         float64
	waitFrames          int
	burstRemaining      int
	burstCmd            Command
}

func NewPathExecutor(maxSpeed, turnSpeed float64) *PathExecutor {
	return &PathExecutor{
		maxSpeed:            maxSpeed,
		turnSpeed:           turnSpeed,
		SpinThresholdDeg:    10.0, // conservative default; config overrides to 20° for AprilTag noise
		BurstFrames:         3,    // conservative default; config overrides to 2 for tighter turns
		MaxWaitFrames:       3,    // conservative default; config overrides to 2
		ForwardThresholdDeg: 45.0, // conservative default; config overrides to 25° (hysteresis: exits at 40°)
	}
}

func (e *PathExecutor) MaxSpeed() float64 {
	return e.maxSpeed
}

func (e *PathExecutor) TurnSpeed() float64 {
	return e.turnSpeed
}

func (e *PathExecutor) VelocityToCommand(vx, vy float64) Command {
	threshold := e.maxSpeed * 0.3
	turnThreshold := e.turnSpeed * 0.3

	if utils.AbsFloat64(vx) < threshold && utils.AbsFloat64(vy) < threshold {
		return CommandStop
	}

	if utils.AbsFloat64(vy) < turnThreshold {
		if vx > threshold {
			return CommandForward
		} else if vx < -threshold {
			return CommandBackward
		}
	}

	absVX := utils.AbsFloat64(vx)
	absVY := utils.AbsFloat64(vy)

	if absVY > absVX {
		if vy > 0 {
			return CommandRight
		} else {
			return CommandLeft
		}
	}

	if vx > 0 {
		return CommandForward
	} else {
		return CommandBackward
	}
}

func (e *PathExecutor) VelocityToCommandWithHeading(worldVx, worldVy, heading float64) Command {
	cosH := math.Cos(heading)
	sinH := math.Sin(heading)
	robotVx := worldVx*cosH + worldVy*sinH
	robotVy := -worldVx*sinH + worldVy*cosH
	cmd := e.VelocityToCommand(robotVx, robotVy)
	utils.Debugf("HEADING: world=(%.3f,%.3f) heading=%.2f° robot=(%.3f,%.3f) -> %c",
		worldVx, worldVy, heading*180/math.Pi, robotVx, robotVy, cmd)
	return cmd
}

func (e *PathExecutor) BearingToCommand(robotHeading, bearingToWaypoint, headingDelta float64) Command {
	angleDiff := bearingToWaypoint - robotHeading
	// Normalize to [-π, π]
	for angleDiff > math.Pi {
		angleDiff -= 2 * math.Pi
	}
	for angleDiff < -math.Pi {
		angleDiff += 2 * math.Pi
	}

	// Hysteresis: two thresholds prevent oscillation at the forward/turn boundary.
	// Without hysteresis, heading noise of 5-15° causes the angle diff to constantly
	// cross a single threshold, producing rapid forward/turn/forward/turn cycling.
	// With hysteresis, the robot must be well-aligned (enterForwardThresh) to START
	// driving forward, but heading must drift far off (exitForwardThresh) before
	// switching back to turning. The 15° dead zone absorbs typical noise.
	enterForwardThresh := e.ForwardThresholdDeg * math.Pi / 180       // e.g. 25° — must be this aligned to start forward
	exitForwardThresh := (e.ForwardThresholdDeg + 10) * math.Pi / 180 // e.g. 30° — must drift this far to start turning
	// Use the wider tolerance when already going forward, tighter when turning
	forwardThreshold := enterForwardThresh
	if !e.isTurning {
		forwardThreshold = exitForwardThresh
	}
	rearThreshold := 3 * math.Pi / 4         // 135°
	spinThreshold := e.SpinThresholdDeg * math.Pi / 180
	headingUpdateThresh := 2 * math.Pi / 180 // 2° change confirms detection caught up
	maxWaitFrames := e.MaxWaitFrames
	burstFrames := e.BurstFrames

	isSpinning := math.Abs(headingDelta) > spinThreshold
	if !e.isTurning {
		isSpinning = false // can't spin while driving forward; large deltas are detection noise
	}
	// Spinning toward target: delta and diff have same sign (closing the gap)
	spinningToward := angleDiff*headingDelta > 0

	// --- Burst phase: continue sending the same turn command ---
	if e.burstRemaining > 0 {
		e.burstRemaining--
		utils.Debugf("STEERING: heading=%.2f° bearing=%.2f° diff=%.2f° delta=%.2f° burst=%d -> %c",
			robotHeading*180/math.Pi, bearingToWaypoint*180/math.Pi,
			angleDiff*180/math.Pi, headingDelta*180/math.Pi, e.burstRemaining, byte(e.burstCmd))
		if e.burstRemaining == 0 {
			// Burst exhausted — enter wait mode
			e.waitingForUpdate = true
			e.waitHeading = robotHeading
			e.waitFrames = 0
		}
		return e.burstCmd
	}

	// --- Wait phase: wait for heading to catch up after a burst ---
	if e.waitingForUpdate {
		e.waitFrames++
		hChange := robotHeading - e.waitHeading
		for hChange > math.Pi {
			hChange -= 2 * math.Pi
		}
		for hChange < -math.Pi {
			hChange += 2 * math.Pi
		}
		if math.Abs(hChange) > headingUpdateThresh || e.waitFrames > maxWaitFrames {
			e.waitingForUpdate = false // heading updated or timeout, re-evaluate
		} else {
			// Drive forward between correction bursts when angle is moderate,
			// like a human rapidly alternating turn taps with forward motion.
			// This replaces dead time (Stop) with forward progress.
			// Safety: only when angle < 90° and not spinning (noise spike).
			waitCmd := CommandStop
			if math.Abs(angleDiff) < math.Pi/2 && !isSpinning {
				waitCmd = CommandForward
			}
			utils.Debugf("STEERING: heading=%.2f° bearing=%.2f° diff=%.2f° delta=%.2f° waiting=%d -> %c",
				robotHeading*180/math.Pi, bearingToWaypoint*180/math.Pi,
				angleDiff*180/math.Pi, headingDelta*180/math.Pi, e.waitFrames, byte(waitCmd))
			return waitCmd
		}
	}

	// --- Continuous turn: large angle, just keep turning (no burst/wait overhead) ---
	// When the angle error is very large, stopping to verify heading every 2 frames
	// wastes ~50% of turn time. Instead, send turn commands continuously and let
	// the spin detector handle heading spikes. Burst/wait is only needed for
	// precision alignment near the forward entry threshold.
	continuousTurnThresh := exitForwardThresh * 1.5 // e.g. 60° when exit=40°
	if math.Abs(angleDiff) > continuousTurnThresh && !isSpinning &&
		math.Abs(angleDiff) < rearThreshold {
		e.isTurning = true
		e.burstRemaining = 0      // cancel any stale burst
		e.waitingForUpdate = false // cancel any stale wait
		var cmd Command
		if angleDiff > 0 {
			cmd = CommandRight
		} else {
			cmd = CommandLeft
		}
		utils.Debugf("STEERING: heading=%.2f° bearing=%.2f° diff=%.2f° delta=%.2f° spinning=%v turning=%v fwdThresh=%.0f° -> %c (continuous)",
			robotHeading*180/math.Pi, bearingToWaypoint*180/math.Pi,
			angleDiff*180/math.Pi, headingDelta*180/math.Pi, isSpinning, e.isTurning,
			forwardThreshold*180/math.Pi, byte(cmd))
		return cmd
	}

	var cmd Command
	switch {
	case math.Abs(angleDiff) > rearThreshold && isSpinning:
		// Rule 3a: facing away and spinning — brake before reversing
		cmd = CommandStop
	case math.Abs(angleDiff) > rearThreshold:
		// Rule 3b: facing away and stable — drive backward toward target
		cmd = CommandBackward
	case isSpinning && spinningToward && math.Abs(angleDiff) < forwardThreshold+math.Abs(headingDelta)*2:
		// Rule 2: spinning toward target and close enough — brake before overshoot
		cmd = CommandStop
	case math.Abs(angleDiff) < forwardThreshold && !isSpinning:
		// Rule 1: aligned and heading is stable — drive forward
		// forwardThreshold is already hysteresis-aware (25° if turning, 40° if forward)
		e.isTurning = false
		// Nudge correction: if drifting past half the exit threshold,
		// inject a single turn command to correct course.
		// Like a human tapping L/R while mostly driving forward.
		nudgeThresh := exitForwardThresh / 2 // ~20° with current config
		if math.Abs(angleDiff) > nudgeThresh && e.nudgeCooldown <= 0 {
			if angleDiff > 0 {
				cmd = CommandRight
			} else {
				cmd = CommandLeft
			}
			e.nudgeCooldown = 3 // wait 3 frames before next nudge
		} else {
			cmd = CommandForward
			if e.nudgeCooldown > 0 {
				e.nudgeCooldown--
			}
		}
	default:
		// Rule 4: turn toward target — send a burst, then wait for precision alignment
		e.isTurning = true
		if angleDiff > 0 {
			cmd = CommandRight
		} else {
			cmd = CommandLeft
		}
		e.burstRemaining = burstFrames - 1 // first frame sends immediately
		e.burstCmd = cmd
	}

	// Nudge: turn command while in forward mode (isTurning=false)
	if !e.isTurning && (cmd == CommandLeft || cmd == CommandRight) {
		utils.Debugf("STEERING: heading=%.2f° bearing=%.2f° diff=%.2f° delta=%.2f° spinning=%v turning=%v fwdThresh=%.0f° -> %c (nudge, cooldown=%d)",
			robotHeading*180/math.Pi, bearingToWaypoint*180/math.Pi,
			angleDiff*180/math.Pi, headingDelta*180/math.Pi, isSpinning, e.isTurning,
			forwardThreshold*180/math.Pi, byte(cmd), e.nudgeCooldown)
	} else {
		utils.Debugf("STEERING: heading=%.2f° bearing=%.2f° diff=%.2f° delta=%.2f° spinning=%v turning=%v fwdThresh=%.0f° -> %c",
			robotHeading*180/math.Pi, bearingToWaypoint*180/math.Pi,
			angleDiff*180/math.Pi, headingDelta*180/math.Pi, isSpinning, e.isTurning,
			forwardThreshold*180/math.Pi, byte(cmd))
	}
	return cmd
}

func (e *PathExecutor) CommandToVelocity(cmd Command) Velocity {
	switch cmd {
	case CommandForward:
		return Velocity{VX: e.maxSpeed, VY: 0}
	case CommandBackward:
		return Velocity{VX: -e.maxSpeed, VY: 0}
	case CommandLeft:
		return Velocity{VX: 0, VY: e.turnSpeed}
	case CommandRight:
		return Velocity{VX: 0, VY: -e.turnSpeed}
	case CommandStop:
		return Velocity{VX: 0, VY: 0}
	default:
		return Velocity{VX: 0, VY: 0}
	}
}
