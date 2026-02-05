package controller

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

func (e *PathExecutor) VelocityToCommand(vx, vy float64) Command {
	threshold := e.maxSpeed * 0.3
	turnThreshold := e.turnSpeed * 0.3

	if abs(vx) < threshold && abs(vy) < threshold {
		return CommandStop
	}

	if abs(vy) < turnThreshold {
		if vx > threshold {
			return CommandForward
		} else if vx < -threshold {
			return CommandBackward
		}
	}

	absVX := abs(vx)
	absVY := abs(vy)

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

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
