package planning

import "math"

type Obstacle struct {
	Name              string
	WorldTopLeft      [2]float64
	WorldBottomRight  [2]float64
	PixelsTopLeft     [2]int
	PixelsBottomRight [2]int
}

type RobotState struct {
	Position [2]float64
	Velocity Velocity
	RobotID  int
	Diameter float64
}

type Velocity struct {
	VX, VY float64
}

type CollisionDetector struct {
	margin float64
}

func NewCollisionDetector(margin float64) *CollisionDetector {
	if margin == 0 {
		margin = 0.05
	}
	return &CollisionDetector{margin: margin}
}

func (d *CollisionDetector) IsCollision(robot RobotState, obstacle Obstacle) bool {
	robotRadius := robot.Diameter / 2

	closestX := robot.Position[0]
	if robot.Position[0] < obstacle.WorldTopLeft[0] {
		closestX = obstacle.WorldTopLeft[0]
	} else if robot.Position[0] > obstacle.WorldBottomRight[0] {
		closestX = obstacle.WorldBottomRight[0]
	}

	closestY := robot.Position[1]
	if robot.Position[1] < obstacle.WorldTopLeft[1] {
		closestY = obstacle.WorldTopLeft[1]
	} else if robot.Position[1] > obstacle.WorldBottomRight[1] {
		closestY = obstacle.WorldBottomRight[1]
	}

	dx := robot.Position[0] - closestX
	dy := robot.Position[1] - closestY
	dist := math.Sqrt(dx*dx + dy*dy)

	return dist < robotRadius+d.margin
}

func (d *CollisionDetector) IsCollisionWithPath(robot RobotState, path [][2]float64, obstacle Obstacle) bool {
	for _, point := range path {
		tempRobot := robot
		tempRobot.Position = point
		if d.IsCollision(tempRobot, obstacle) {
			return true
		}
	}
	return false
}

func (d *CollisionDetector) CheckAllCollisions(robot RobotState, obstacles []Obstacle) []Obstacle {
	collisions := make([]Obstacle, 0)
	for _, obs := range obstacles {
		if d.IsCollision(robot, obs) {
			collisions = append(collisions, obs)
		}
	}
	return collisions
}

func (d *CollisionDetector) WillCollide(robot RobotState, velocity [2]float64, obstacles []Obstacle, timeHorizon float64) bool {
	futurePos := [2]float64{
		robot.Position[0] + velocity[0]*timeHorizon,
		robot.Position[1] + velocity[1]*timeHorizon,
	}

	tempRobot := robot
	tempRobot.Position = futurePos

	for _, obs := range obstacles {
		if d.IsCollision(tempRobot, obs) {
			return true
		}
	}

	return false
}

func (d *CollisionDetector) DistanceToObstacle(robot RobotState, obstacle Obstacle) float64 {
	robotRadius := robot.Diameter / 2

	closestX := robot.Position[0]
	if robot.Position[0] < obstacle.WorldTopLeft[0] {
		closestX = obstacle.WorldTopLeft[0]
	} else if robot.Position[0] > obstacle.WorldBottomRight[0] {
		closestX = obstacle.WorldBottomRight[0]
	}

	closestY := robot.Position[1]
	if robot.Position[1] < obstacle.WorldTopLeft[1] {
		closestY = obstacle.WorldTopLeft[1]
	} else if robot.Position[1] > obstacle.WorldBottomRight[1] {
		closestY = obstacle.WorldBottomRight[1]
	}

	dx := robot.Position[0] - closestX
	dy := robot.Position[1] - closestY
	dist := math.Sqrt(dx*dx + dy*dy)

	return dist - robotRadius
}

func (d *CollisionDetector) IsPointInObstacle(point [2]float64, obstacle Obstacle) bool {
	return point[0] >= obstacle.WorldTopLeft[0] &&
		point[0] <= obstacle.WorldBottomRight[0] &&
		point[1] >= obstacle.WorldTopLeft[1] &&
		point[1] <= obstacle.WorldBottomRight[1]
}

func (d *CollisionDetector) ExpandObstacle(obstacle Obstacle, amount float64) Obstacle {
	expanded := obstacle
	expanded.WorldTopLeft[0] -= amount
	expanded.WorldTopLeft[1] -= amount
	expanded.WorldBottomRight[0] += amount
	expanded.WorldBottomRight[1] += amount
	return expanded
}

func (d *CollisionDetector) GetClearance(robot RobotState, obstacles []Obstacle) float64 {
	minDist := 1e10
	for _, obs := range obstacles {
		dist := d.DistanceToObstacle(robot, obs)
		if dist < minDist {
			minDist = dist
		}
	}
	return minDist
}
