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
