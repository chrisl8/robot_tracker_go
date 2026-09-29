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

// RobotState is what the planner knows about a robot.
type RobotState struct {
	Position [2]float64
	RobotID  int
	Diameter float64
}

// clearance returns the smallest gap in metres between the robot's body (a
// circle of its Diameter) and any obstacle footprint; negative means they
// overlap. With no obstacles it returns a very large value.
func clearance(robot RobotState, obstacles []Obstacle) float64 {
	robotRadius := robot.Diameter / 2
	minGap := 1e10
	for _, obs := range obstacles {
		if gap := distanceToQuad(robot.Position, obs.Quad) - robotRadius; gap < minGap {
			minGap = gap
		}
	}
	return minGap
}
