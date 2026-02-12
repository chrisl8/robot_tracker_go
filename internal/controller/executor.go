package controller

import (
	"math"

	"robot_tracker_go/internal/utils"
)

type Velocity struct {
	VX float64
	VY float64
}

type PathExecutor struct {
	maxSpeed  float64
	turnSpeed float64
}

func NewPathExecutor(maxSpeed, turnSpeed float64) *PathExecutor {
	return &PathExecutor{
		maxSpeed:  maxSpeed,
		turnSpeed: turnSpeed,
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

	forwardThreshold := math.Pi / 6       // 30°
	rearThreshold := math.Pi * 8 / 9      // 160°
	spinThreshold := 10 * math.Pi / 180   // 10°/frame

	isSpinning := math.Abs(headingDelta) > spinThreshold
	// Spinning toward target: delta and diff have same sign (closing the gap)
	spinningToward := angleDiff*headingDelta > 0

	var cmd Command
	switch {
	case math.Abs(angleDiff) > rearThreshold:
		// Rule 3: facing away — always turn Right to break ±180° oscillation
		cmd = CommandRight
	case isSpinning && spinningToward && math.Abs(angleDiff) < forwardThreshold+math.Abs(headingDelta)*2:
		// Rule 2: spinning toward target and close enough — brake before overshoot
		cmd = CommandStop
	case math.Abs(angleDiff) < forwardThreshold && !isSpinning:
		// Rule 1: aligned and heading is stable — drive forward
		cmd = CommandForward
	default:
		// Rule 4: turn toward target
		if angleDiff > 0 {
			cmd = CommandRight
		} else {
			cmd = CommandLeft
		}
	}

	utils.Debugf("STEERING: heading=%.2f° bearing=%.2f° diff=%.2f° delta=%.2f° spinning=%v -> %c",
		robotHeading*180/math.Pi, bearingToWaypoint*180/math.Pi,
		angleDiff*180/math.Pi, headingDelta*180/math.Pi, isSpinning, byte(cmd))
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
