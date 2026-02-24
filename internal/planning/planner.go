package planning

import (
	"math"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
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
	dynamicObstacles  []Obstacle
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
	// Expand obstacles by the robot's radius so the path keeps the full body clear.
	// Add safety margin beyond the radius to account for YOLO bbox inaccuracy and chair legs.
	margin := 0.15
	if robot, exists := p.coordinator.GetRobotState(robotID); exists && robot.Diameter > 0 {
		margin = robot.Diameter/2 + 0.12
	}
	allObstacles := make([]Obstacle, 0, len(p.obstacles)+len(p.dynamicObstacles))
	allObstacles = append(allObstacles, p.obstacles...)
	allObstacles = append(allObstacles, p.dynamicObstacles...)
	obstacles := p.expandObstacles(allObstacles, margin)
	path, ok := p.globalPlanner.Plan(start, goal, obstacles)
	if ok && len(path) > 2 {
		before := len(path)
		originalPath := make([][2]float64, len(path))
		copy(originalPath, path)
		simplified := SimplifyPath(path, 0.15)
		path = ValidateSimplifiedPath(originalPath, simplified, obstacles, 0.05)
		path = SimplifyPath(path, 0.15)
		utils.Debugf("Path simplified: %d -> %d waypoints (validated against %d obstacles)", before, len(path), len(obstacles))
	}
	return path, ok
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
	p.replanAllPaths()
}

func (p *Planner) SetDynamicObstacles(obstacles []Obstacle) {
	if obstaclesEqual(p.dynamicObstacles, obstacles) {
		return
	}
	p.dynamicObstacles = obstacles
	p.replanAllPaths()
}

func (p *Planner) AddRobot(id int, position [2]float64, diameter float64) {
	robot := RobotState{
		Position: position,
		Velocity: Velocity{0, 0},
		RobotID:  id,
		Diameter: diameter,
	}
	p.coordinator.AddRobot(id, robot)

	// Only plan if the robot has a goal but no existing path
	if _, hasPath := p.paths[id]; !hasPath {
		if goal, hasGoal := p.coordinator.GetGoal(id); hasGoal {
			utils.Debugf("AddRobot: robot %d has goal (%.2f,%.2f) but no path, planning...",id, goal[0], goal[1])
			path, success := p.PlanPath(id, position, goal)
			utils.Debugf("AddRobot: PlanPath success=%v pathLen=%d",success, len(path))
			if success {
				p.paths[id] = path
				p.currentWaypoint[id] = 0
			}
		}
	}
}

func (p *Planner) SetGoal(robotID int, goal [2]float64) {
	utils.Debugf("SetGoal: robotID=%d goal=(%.2f,%.2f)",robotID, goal[0], goal[1])
	p.coordinator.SetGoal(robotID, goal)

	robot, exists := p.coordinator.GetRobotState(robotID)
	utils.Debugf("SetGoal: robot exists=%v",exists)
	if exists {
		utils.Debugf("SetGoal: robot position=(%.2f,%.2f), planning path...",robot.Position[0], robot.Position[1])
		path, success := p.PlanPath(robotID, robot.Position, goal)
		utils.Debugf("SetGoal: PlanPath success=%v pathLen=%d",success, len(path))
		if success {
			p.paths[robotID] = path
			p.currentWaypoint[robotID] = 0
		}
	} else {
		utils.Debugf("SetGoal: robot %d NOT in planner yet, goal stored for later",robotID)
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

func (p *Planner) expandObstacles(obstacles []Obstacle, margin float64) []Obstacle {
	expanded := make([]Obstacle, len(obstacles))
	for i, obs := range obstacles {
		expanded[i] = p.collisionDetector.ExpandObstacle(obs, margin)
	}
	return expanded
}

func (p *Planner) replanAllPaths() {
	for robotID := range p.paths {
		robot, exists := p.coordinator.GetRobotState(robotID)
		if !exists {
			continue
		}
		goal, hasGoal := p.coordinator.GetGoal(robotID)
		if !hasGoal {
			continue
		}
		path, success := p.PlanPath(robotID, robot.Position, goal)
		if success {
			p.paths[robotID] = path
			p.currentWaypoint[robotID] = 0
		}
	}
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
	p.replanAllPaths()
}

func (p *Planner) ClearObstacles() {
	p.obstacles = make([]Obstacle, 0)
	p.coordinator.SetObstacles(p.obstacles)
	p.replanAllPaths()
}

func (p *Planner) ClearAll() {
	p.obstacles = make([]Obstacle, 0)
	p.coordinator.ClearAll()
}

func (p *Planner) GetObstacles() []Obstacle {
	return p.obstacles
}

// GetClearance returns the minimum distance from the robot's edge to any obstacle surface.
func (p *Planner) GetClearance(robotID int) float64 {
	robot, exists := p.coordinator.GetRobotState(robotID)
	if !exists {
		return 1e10
	}
	allObstacles := make([]Obstacle, 0, len(p.obstacles)+len(p.dynamicObstacles))
	allObstacles = append(allObstacles, p.obstacles...)
	allObstacles = append(allObstacles, p.dynamicObstacles...)
	return p.collisionDetector.GetClearance(robot, allObstacles)
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
		p.coordinator.ClearGoal(robotID)
		return false
	}
	return true
}

// AdvancePastWaypoints skips past any waypoints the robot has reached or overshot.
// It returns true if there are still waypoints remaining, false if the path is complete.
func (p *Planner) AdvancePastWaypoints(robotID int, pos [2]float64, threshold float64) bool {
	path, hasPath := p.paths[robotID]
	if !hasPath {
		return false
	}

	wpIdx := p.currentWaypoint[robotID]
	advanced := 0

	for wpIdx < len(path)-1 {
		dx := path[wpIdx][0] - pos[0]
		dy := path[wpIdx][1] - pos[1]
		distToCurrent := math.Sqrt(dx*dx + dy*dy)

		// Within threshold — advance past this waypoint
		if distToCurrent < threshold {
			wpIdx++
			advanced++
			continue
		}

		// Check if we overshot: closer to next waypoint than current one
		if wpIdx+1 < len(path) {
			nx := path[wpIdx+1][0] - pos[0]
			ny := path[wpIdx+1][1] - pos[1]
			distToNext := math.Sqrt(nx*nx + ny*ny)
			if distToNext < distToCurrent {
				wpIdx++
				advanced++
				continue
			}
		}

		// Check if current waypoint is behind direction of travel toward next waypoint.
		// A negative dot product means the current wp is opposite the direction to the next wp.
		if wpIdx+1 < len(path) {
			dnx := path[wpIdx+1][0] - pos[0]
			dny := path[wpIdx+1][1] - pos[1]
			dcx := path[wpIdx][0] - pos[0]
			dcy := path[wpIdx][1] - pos[1]
			if dnx*dcx+dny*dcy < 0 {
				wpIdx++
				advanced++
				continue
			}
		}

		break
	}

	if advanced > 0 {
		utils.Logf("Robot %d skipped %d waypoints", robotID, advanced)
	}

	p.currentWaypoint[robotID] = wpIdx

	// All waypoints consumed — path complete
	if wpIdx >= len(path) {
		delete(p.paths, robotID)
		delete(p.currentWaypoint, robotID)
		p.coordinator.ClearGoal(robotID)
		return false
	}

	return true
}

func (p *Planner) GetPaths() map[int][][2]float64 {
	return p.paths
}

func (p *Planner) GetPathsWithGoals() map[int][][2]float64 {
	utils.Debugf("GetPathsWithGoals: %d goals, %d existing paths",len(p.coordinator.goals), len(p.paths))
	for robotID, goal := range p.coordinator.goals {
		if _, hasPath := p.paths[robotID]; !hasPath {
			robot, exists := p.coordinator.GetRobotState(robotID)
			startPos := [2]float64{0, 0}
			if exists {
				startPos = robot.Position
			}
			utils.Debugf("GetPathsWithGoals: planning for robot %d, start=(%.2f,%.2f) goal=(%.2f,%.2f)",robotID, startPos[0], startPos[1], goal[0], goal[1])
			path, success := p.PlanPath(robotID, startPos, goal)
			utils.Debugf("GetPathsWithGoals: PlanPath success=%v pathLen=%d",success, len(path))
			if success {
				p.paths[robotID] = path
				p.currentWaypoint[robotID] = 0
			}
		}
	}

	// Return only remaining waypoints (from current waypoint onward)
	remaining := make(map[int][][2]float64)
	for robotID, path := range p.paths {
		wpIdx := p.currentWaypoint[robotID]
		if wpIdx < len(path) {
			remaining[robotID] = path[wpIdx:]
		}
	}
	return remaining
}

func (p *Planner) GetGoal(robotID int) ([2]float64, bool) {
	return p.coordinator.GetGoal(robotID)
}

func (p *Planner) CompletePath(robotID int) {
	delete(p.paths, robotID)
	delete(p.currentWaypoint, robotID)
	p.coordinator.ClearGoal(robotID)
}

// ClearPathOnly removes the current path without clearing the goal,
// allowing replanning to the same destination.
func (p *Planner) ClearPathOnly(robotID int) {
	delete(p.paths, robotID)
	delete(p.currentWaypoint, robotID)
}

// SetPath stores a pre-computed path for a robot and resets the waypoint index.
func (p *Planner) SetPath(robotID int, path [][2]float64) {
	p.paths[robotID] = path
	p.currentWaypoint[robotID] = 0
}

func (p *Planner) LocalPlanner() *LocalPlanner {
	return p.localPlanner
}

// obstaclesEqual returns true if two obstacle slices have the same contents.
// World coordinates are compared with a small epsilon to avoid replanning on
// sub-centimeter jitter from detection noise.
func obstaclesEqual(a, b []Obstacle) bool {
	if len(a) != len(b) {
		return false
	}
	const eps = 0.01
	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
		if math.Abs(a[i].WorldTopLeft[0]-b[i].WorldTopLeft[0]) > eps ||
			math.Abs(a[i].WorldTopLeft[1]-b[i].WorldTopLeft[1]) > eps ||
			math.Abs(a[i].WorldBottomRight[0]-b[i].WorldBottomRight[0]) > eps ||
			math.Abs(a[i].WorldBottomRight[1]-b[i].WorldBottomRight[1]) > eps {
			return false
		}
	}
	return true
}
