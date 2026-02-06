package planning

import "math"

type DynamicObstacle struct {
	X          float64
	Y          float64
	VX         float64
	VY         float64
	Radius     float64
	ClassName  string
	Confidence float64
	IsRobot    bool
}

func NewDynamicObstacle(x, y, vx, vy, radius float64, className string, confidence float64, isRobot bool) *DynamicObstacle {
	return &DynamicObstacle{
		X:          x,
		Y:          y,
		VX:         vx,
		VY:         vy,
		Radius:     radius,
		ClassName:  className,
		Confidence: confidence,
		IsRobot:    isRobot,
	}
}

func (o *DynamicObstacle) ContainsPoint(px, py float64) bool {
	dx := px - o.X
	dy := py - o.Y
	return dx*dx+dy*dy <= o.Radius*o.Radius
}

func (o *DynamicObstacle) DistanceTo(ox, oy float64) float64 {
	dx := ox - o.X
	dy := oy - o.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func (o *DynamicObstacle) DistanceToObstacle(other *DynamicObstacle) float64 {
	dx := other.X - o.X
	dy := other.Y - o.Y
	dist := math.Sqrt(dx*dx + dy*dy)
	return dist - o.Radius - other.Radius
}

func (o *DynamicObstacle) ToRobotState() RobotState {
	return RobotState{
		Position: [2]float64{o.X, o.Y},
		Velocity: Velocity{VX: o.VX, VY: o.VY},
		RobotID:  -1,
		Diameter: o.Radius * 2,
	}
}
