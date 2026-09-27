package planning

type Obstacle struct {
	Name              string
	WorldTopLeft      [2]float64
	WorldBottomRight  [2]float64
	PixelsTopLeft     [2]int
	PixelsBottomRight [2]int
	// Quad is the obstacle's exact footprint, four world-metre corners in a
	// consistent winding order (see Quad). WorldTopLeft/WorldBottomRight
	// remain a cached axis-aligned envelope (still used for cheap jitter
	// comparisons); Quad is what collision/clearance checks and A*
	// rasterization use, so a rotated obstacle is not inflated to its
	// bounding box. For a plain rectangle, use NewRectObstacle, which fills
	// both consistently. For a genuinely oriented obstacle (a detector's
	// fitted footprint), build the struct directly with a real Quad — just
	// keep WorldTopLeft/WorldBottomRight as that Quad's own bounding box. A
	// bare struct literal with only WorldTopLeft/WorldBottomRight set gets a
	// zero-value Quad, which callers must not do outside this package's own
	// tests.
	Quad Quad
}

// NewRectObstacle builds an axis-aligned rectangular obstacle: the ordinary
// case for a user-marked static obstacle or a legacy (non-oriented) detection.
// It keeps the legacy WorldTopLeft/WorldBottomRight fields and derives the
// matching (degenerate rectangular) Quad from them, so every Obstacle in the
// system is guaranteed to have both consistently populated.
func NewRectObstacle(name string, worldTopLeft, worldBottomRight [2]float64) Obstacle {
	return Obstacle{
		Name:             name,
		WorldTopLeft:     worldTopLeft,
		WorldBottomRight: worldBottomRight,
		Quad:             RectQuad(worldTopLeft, worldBottomRight),
	}
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
	return distanceToQuad(robot.Position, obstacle.Quad) < robotRadius+d.margin
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
	return distanceToQuad(robot.Position, obstacle.Quad) - robotRadius
}

func (d *CollisionDetector) IsPointInObstacle(point [2]float64, obstacle Obstacle) bool {
	return quadContains(point, obstacle.Quad)
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
