package planning

// Coordinator is a shared per-robot state store used by Planner: current
// position/velocity (robots), the active destination (goals), and the
// obstacle set last pushed to it. It does not itself decide how a robot
// should move — Planner reads and writes this state directly and drives
// robots via A* (globalPlanner) plus bearing-based steering in cmd/main.go.
//
// Coordinator previously also held a priority-ordered, per-robot
// velocity-command/conflict-resolution scheme (ComputeCommands,
// ResolveConflicts, willCollide, adjustForConflict, AssignGoals) intended as
// an alternative to that live pipeline. It was never wired into
// cmd/main.go — Planner's own thin wrappers around it (ComputeAllCommands,
// ResolveConflicts) had zero callers either — so it was removed as dead code
// (code review tech-debt: "Dead second collision-avoidance/coordination
// system"). See docs/code-review-2026-09-27.md for the investigation.
type Coordinator struct {
	robots    map[int]RobotState
	goals     map[int][2]float64
	obstacles []Obstacle
}

func NewCoordinator() *Coordinator {
	return &Coordinator{
		robots:    make(map[int]RobotState),
		goals:     make(map[int][2]float64),
		obstacles: make([]Obstacle, 0),
	}
}

func (c *Coordinator) AddRobot(id int, state RobotState) {
	c.robots[id] = state
}

func (c *Coordinator) SetGoal(robotID int, goal [2]float64) {
	c.goals[robotID] = goal
}

func (c *Coordinator) ClearGoal(robotID int) {
	delete(c.goals, robotID)
}

func (c *Coordinator) SetObstacles(obstacles []Obstacle) {
	c.obstacles = obstacles
}

func (c *Coordinator) RemoveRobot(id int) {
	delete(c.robots, id)
	delete(c.goals, id)
}

func (c *Coordinator) GetRobotState(id int) (RobotState, bool) {
	state, exists := c.robots[id]
	return state, exists
}

func (c *Coordinator) UpdateRobots(positions map[int][2]float64, velocities map[int][2]float64) {
	for id, pos := range positions {
		if robot, exists := c.robots[id]; exists {
			robot.Position = pos
			if vel, hasVel := velocities[id]; hasVel {
				robot.Velocity = Velocity{VX: vel[0], VY: vel[1]}
			}
			c.robots[id] = robot
		}
	}
}

func (c *Coordinator) GetRobotCount() int {
	return len(c.robots)
}

func (c *Coordinator) GetGoal(robotID int) ([2]float64, bool) {
	goal, exists := c.goals[robotID]
	return goal, exists
}

func (c *Coordinator) ClearAll() {
	c.robots = make(map[int]RobotState)
	c.goals = make(map[int][2]float64)
}
