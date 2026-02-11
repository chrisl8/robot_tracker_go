package planning

import (
	"math"

	"robot_tracker_go/internal/utils"
)

type PlannerConfig struct {
	AStarConfig            *AStarConfig
	VelocityObstacleConfig *VelocityObstacleConfig
	CollisionMargin        float64
}

type Planner struct {
	globalPlanner     *AStar
	localPlanner      *LocalPlanner
	coordinator       *Coordinator
	collisionDetector *CollisionDetector
	obstacles         []Obstacle
	paths             map[int][][2]float64 // robotID -> list of waypoints
	currentWaypoint   map[int]int          // robotID -> index into paths
}

func NewPlanner(config *PlannerConfig) *Planner {
	planner := &Planner{
		paths:           make(map[int][][2]float64),
		currentWaypoint: make(map[int]int),
	}

	if config != nil {
		planner.globalPlanner = NewAStar(config.AStarConfig)
		planner.localPlanner = NewLocalPlanner(config.VelocityObstacleConfig)
		planner.collisionDetector = NewCollisionDetector(config.CollisionMargin)
	} else {
		planner.globalPlanner = NewAStar(nil)
		planner.localPlanner = NewLocalPlanner(nil)
		planner.collisionDetector = NewCollisionDetector(0)
	}

	planner.coordinator = NewCoordinator(planner.localPlanner, planner.collisionDetector)

	return planner
}

func (p *Planner) PlanPath(robotID int, start, goal [2]float64) ([][2]float64, bool) {
	obstacles := p.expandObstacles()
	return p.globalPlanner.Plan(start, goal, obstacles)
}

func (p *Planner) ComputeVelocity(robotID int, goal [2]float64) ([2]float64, bool) {
	robot, exists := p.coordinator.GetRobotState(robotID)
	if !exists {
		return [2]float64{0, 0}, false
	}

	otherRobots := p.coordinator.getOtherRobots(robotID)
	return p.localPlanner.ComputeVelocity(robot, goal, otherRobots)
}

func (p *Planner) ComputeVelocityWithDynamicObstacles(
	robotID int,
	goal [2]float64,
	dynamicObstacles []*DynamicObstacle,
	minConfidence float64,
) ([2]float64, bool) {
	robot, exists := p.coordinator.GetRobotState(robotID)
	if !exists {
		return [2]float64{0, 0}, false
	}

	otherRobots := p.coordinator.getOtherRobots(robotID)
	return p.localPlanner.ComputeVelocityWithObstacles(robot, goal, otherRobots, dynamicObstacles, minConfidence)
}

func (p *Planner) AddObstacle(obstacle Obstacle) {
	p.obstacles = append(p.obstacles, obstacle)
	p.coordinator.SetObstacles(p.obstacles)
}

func (p *Planner) SetObstacles(obstacles []Obstacle) {
	p.obstacles = obstacles
	p.coordinator.SetObstacles(obstacles)
}

func (p *Planner) AddRobot(id int, position [2]float64, diameter float64) {
	robot := RobotState{
		Position: position,
		Velocity: Velocity{0, 0},
		RobotID:  id,
		Diameter: diameter,
	}
	p.coordinator.AddRobot(id, robot)
}

func (p *Planner) SetGoal(robotID int, goal [2]float64) {
	p.coordinator.SetGoal(robotID, goal)

	robot, exists := p.coordinator.GetRobotState(robotID)
	if exists {
		path, success := p.PlanPath(robotID, robot.Position, goal)
		if success {
			p.paths[robotID] = path
			p.currentWaypoint[robotID] = 0
		}
	}
}

func (p *Planner) ComputeAllCommands() map[int][2]float64 {
	return p.coordinator.ComputeCommands()
}

func (p *Planner) ResolveConflicts(commands map[int][2]float64) map[int][2]float64 {
	return p.coordinator.ResolveConflicts(commands)
}

func (p *Planner) UpdateRobotState(robotID int, position [2]float64, velocity [2]float64) {
	positions := map[int][2]float64{robotID: position}
	velocities := map[int][2]float64{robotID: velocity}
	p.coordinator.UpdateRobots(positions, velocities)
}

func (p *Planner) GetRobotState(robotID int) (RobotState, bool) {
	return p.coordinator.GetRobotState(robotID)
}

func (p *Planner) GetAllRobotStates() map[int]RobotState {
	return p.coordinator.robots
}

func (p *Planner) CheckCollision(robot RobotState) bool {
	collisions := p.collisionDetector.CheckAllCollisions(robot, p.obstacles)
	return len(collisions) > 0
}

func (p *Planner) expandObstacles() []Obstacle {
	expanded := make([]Obstacle, len(p.obstacles))
	for i, obs := range p.obstacles {
		expanded[i] = p.collisionDetector.ExpandObstacle(obs, 0.05)
	}
	return expanded
}

func (p *Planner) RemoveObstacle(name string) {
	newObstacles := make([]Obstacle, 0, len(p.obstacles)-1)
	for _, obs := range p.obstacles {
		if obs.Name != name {
			newObstacles = append(newObstacles, obs)
		}
	}
	p.obstacles = newObstacles
	p.coordinator.SetObstacles(p.obstacles)
}

func (p *Planner) ClearObstacles() {
	p.obstacles = make([]Obstacle, 0)
	p.coordinator.SetObstacles(p.obstacles)
}

func (p *Planner) ClearAll() {
	p.obstacles = make([]Obstacle, 0)
	p.coordinator.ClearAll()
}

func (p *Planner) GetObstacles() []Obstacle {
	return p.obstacles
}

func (p *Planner) GetPathCost(path [][2]float64) float64 {
	if len(path) < 2 {
		return 0
	}

	cost := 0.0
	for i := 1; i < len(path); i++ {
		dx := path[i][0] - path[i-1][0]
		dy := path[i][1] - path[i-1][1]
		cost += math.Sqrt(dx*dx + dy*dy)
	}

	return cost
}

func (p *Planner) GetNextWaypoint(robotID int) ([2]float64, bool) {
	path, hasPath := p.paths[robotID]
	if !hasPath {
		return [2]float64{0, 0}, false
	}
	wpIndex := p.currentWaypoint[robotID]
	if wpIndex >= len(path) {
		return [2]float64{0, 0}, false
	}
	return path[wpIndex], true
}

func (p *Planner) AdvanceWaypoint(robotID int) bool {
	p.currentWaypoint[robotID]++
	path, hasPath := p.paths[robotID]
	if !hasPath {
		return false
	}
	if p.currentWaypoint[robotID] >= len(path) {
		delete(p.paths, robotID)
		delete(p.currentWaypoint, robotID)
		p.coordinator.SetGoal(robotID, [2]float64{0, 0})
		return false
	}
	return true
}

func (p *Planner) GetPaths() map[int][][2]float64 {
	return p.paths
}

func (p *Planner) GetPathsWithGoals() map[int][][2]float64 {
	utils.Logf("GPWG: coordinator.goals has %d entries", len(p.coordinator.goals))
	for robotID, goal := range p.coordinator.goals {
		utils.Logf("GPWG: Checking robot %d, goal=(%.2f, %.2f)", robotID, goal[0], goal[1])
		if _, hasPath := p.paths[robotID]; !hasPath {
			utils.Logf("GPWG: Robot %d has no path, checking if robot exists", robotID)
			robot, exists := p.coordinator.GetRobotState(robotID)
			if exists {
				utils.Logf("GPWG: Robot %d exists at (%.2f, %.2f), planning path", robotID, robot.Position[0], robot.Position[1])
				path, success := p.PlanPath(robotID, robot.Position, goal)
				if success {
					p.paths[robotID] = path
					p.currentWaypoint[robotID] = 0
					utils.Logf("GPWG: Planned %d waypoints for robot %d", len(path), robotID)
				} else {
					utils.Logf("GPWG: Planning failed for robot %d", robotID)
				}
			} else {
				utils.Logf("GPWG: Robot %d NOT FOUND in coordinator (not tracked yet)", robotID)
			}
		} else {
			utils.Logf("GPWG: Robot %d already has path with %d waypoints", robotID, len(p.paths[robotID]))
		}
	}
	utils.Logf("GPWG: Returning %d paths total", len(p.paths))
	return p.paths
}

func (p *Planner) LocalPlanner() *LocalPlanner {
	return p.localPlanner
}
