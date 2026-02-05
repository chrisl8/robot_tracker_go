package planning

import "math"

type Coordinator struct {
	robots            map[int]RobotState
	goals             map[int][2]float64
	obstacles         []Obstacle
	localPlanner      *LocalPlanner
	collisionDetector *CollisionDetector
	priorities        map[int]int
}

func NewCoordinator(localPlanner *LocalPlanner, collisionDetector *CollisionDetector) *Coordinator {
	return &Coordinator{
		robots:            make(map[int]RobotState),
		goals:             make(map[int][2]float64),
		obstacles:         make([]Obstacle, 0),
		localPlanner:      localPlanner,
		collisionDetector: collisionDetector,
		priorities:        make(map[int]int),
	}
}

func (c *Coordinator) AddRobot(id int, state RobotState) {
	c.robots[id] = state
	c.priorities[id] = id
}

func (c *Coordinator) SetGoal(robotID int, goal [2]float64) {
	c.goals[robotID] = goal
}

func (c *Coordinator) SetObstacles(obstacles []Obstacle) {
	c.obstacles = obstacles
}

func (c *Coordinator) RemoveRobot(id int) {
	delete(c.robots, id)
	delete(c.goals, id)
	delete(c.priorities, id)
}

func (c *Coordinator) GetRobotState(id int) (RobotState, bool) {
	state, exists := c.robots[id]
	return state, exists
}

func (c *Coordinator) ComputeCommands() map[int][2]float64 {
	commands := make(map[int][2]float64)

	sortedRobots := c.getRobotsByPriority()

	for _, robotID := range sortedRobots {
		robot := c.robots[robotID]
		goal, hasGoal := c.goals[robotID]

		if !hasGoal {
			commands[robotID] = [2]float64{0, 0}
			continue
		}

		otherRobots := c.getOtherRobots(robotID)
		velocity, safe := c.localPlanner.ComputeVelocity(robot, goal, otherRobots)

		if !safe {
			velocity = [2]float64{0, 0}
		}

		commands[robotID] = velocity
	}

	return commands
}

func (c *Coordinator) getRobotsByPriority() []int {
	robots := make([]int, 0, len(c.robots))
	for id := range c.robots {
		robots = append(robots, id)
	}

	for i := 0; i < len(robots); i++ {
		for j := i + 1; j < len(robots); j++ {
			if c.priorities[robots[i]] > c.priorities[robots[j]] {
				robots[i], robots[j] = robots[j], robots[i]
			}
		}
	}

	return robots
}

func (c *Coordinator) getOtherRobots(selfID int) []RobotState {
	others := make([]RobotState, 0, len(c.robots)-1)
	for id, robot := range c.robots {
		if id != selfID {
			others = append(others, robot)
		}
	}
	return others
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

func (c *Coordinator) ResolveConflicts(commands map[int][2]float64) map[int][2]float64 {
	resolved := make(map[int][2]float64)

	for robotID, vel := range commands {
		robot := c.robots[robotID]

		collides := false
		for otherID, otherVel := range commands {
			if robotID == otherID {
				continue
			}

			otherRobot := c.robots[otherID]
			if c.willCollide(robot, vel, otherRobot, otherVel) {
				collides = true
				break
			}
		}

		if collides {
			resolved[robotID] = c.adjustForConflict(robotID, vel, commands)
		} else {
			resolved[robotID] = vel
		}
	}

	return resolved
}

func (c *Coordinator) willCollide(robot1 RobotState, vel1 [2]float64, robot2 RobotState, vel2 [2]float64) bool {
	combinedRadius := (robot1.Diameter + robot2.Diameter) / 2

	relVx := vel1[0] - vel2[0]
	relVy := vel1[1] - vel2[1]
	relVx += robot2.Velocity.VX - robot1.Velocity.VX
	relVy += robot2.Velocity.VY - robot1.Velocity.VY

	dx := robot2.Position[0] - robot1.Position[0]
	dy := robot2.Position[1] - robot1.Position[1]

	timeToCollision := (dx*relVx + dy*relVy) / (relVx*relVx + relVy*relVy + 0.0001)
	if timeToCollision < 0 {
		return false
	}

	collisionX := robot1.Position[0] + vel1[0]*timeToCollision
	collisionY := robot1.Position[1] + vel1[1]*timeToCollision

	distX := collisionX - robot2.Position[0]
	distY := collisionY - robot2.Position[1]
	dist := math.Sqrt(distX*distX + distY*distY)

	return dist < combinedRadius
}

func (c *Coordinator) adjustForConflict(robotID int, vel [2]float64, allCommands map[int][2]float64) [2]float64 {
	adjusted := vel
	priority := c.priorities[robotID]

	for otherID := range allCommands {
		if robotID == otherID {
			continue
		}

		if c.priorities[otherID] > priority {
			continue
		}

		robot := c.robots[robotID]
		other := c.robots[otherID]

		dx := other.Position[0] - robot.Position[0]
		dy := other.Position[1] - robot.Position[1]
		dist := math.Sqrt(dx*dx + dy*dy)

		if dist < 0.5 {
			avoidX := -dx / dist * 0.1
			avoidY := -dy / dist * 0.1
			adjusted[0] += avoidX
			adjusted[1] += avoidY
		}
	}

	velMag := math.Sqrt(adjusted[0]*adjusted[0] + adjusted[1]*adjusted[1])
	if velMag > 0.3 {
		adjusted[0] = (adjusted[0] / velMag) * 0.3
		adjusted[1] = (adjusted[1] / velMag) * 0.3
	}

	return adjusted
}

func (c *Coordinator) AssignGoals(assignments map[int][2]float64) {
	for robotID, goal := range assignments {
		c.goals[robotID] = goal
	}
}

func (c *Coordinator) GetRobotCount() int {
	return len(c.robots)
}

func (c *Coordinator) ClearAll() {
	c.robots = make(map[int]RobotState)
	c.goals = make(map[int][2]float64)
	c.priorities = make(map[int]int)
}
