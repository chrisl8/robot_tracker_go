package planning

import (
	"container/heap"
	"math"

	"robot_tracker_go/internal/utils"
)

type Node struct {
	Pos      [2]int
	G        float64
	H        float64
	F        float64
	Parent   *Node
	Obstacle bool
}

type PriorityQueue []*Node

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	if pq[i] == nil || pq[j] == nil {
		return false
	}
	return pq[i].F < pq[j].F
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*Node))
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	*pq = old[0 : n-1]
	return node
}

type AStarConfig struct {
	GridWidth     int
	GridHeight    int
	Resolution    float64
	MaxIterations int
}

type AStar struct {
	config *AStarConfig
}

func NewAStar(config *AStarConfig) *AStar {
	if config == nil {
		config = &AStarConfig{
			GridWidth:     100,
			GridHeight:    100,
			Resolution:    0.05,
			MaxIterations: 10000,
		}
	}
	return &AStar{config: config}
}

func (a *AStar) Plan(start, goal [2]float64, obstacles []Obstacle) ([][2]float64, bool) {
	gridWidth := int(float64(a.config.GridWidth) / a.config.Resolution)
	gridHeight := int(float64(a.config.GridHeight) / a.config.Resolution)

	// Offset so world (0,0) maps to grid center, allowing negative world coordinates
	offsetX := gridWidth / 2
	offsetY := gridHeight / 2

	startNode := &Node{Pos: [2]int{int(start[0]/a.config.Resolution) + offsetX, int(start[1]/a.config.Resolution) + offsetY}}
	goalNode := &Node{Pos: [2]int{int(goal[0]/a.config.Resolution) + offsetX, int(goal[1]/a.config.Resolution) + offsetY}}

	utils.Debugf("A*: start world=(%.2f,%.2f) grid=(%d,%d), goal world=(%.2f,%.2f) grid=(%d,%d), gridSize=%dx%d, obstacles=%d\n",
		start[0], start[1], startNode.Pos[0], startNode.Pos[1],
		goal[0], goal[1], goalNode.Pos[0], goalNode.Pos[1],
		gridWidth, gridHeight, len(obstacles))

	if startNode.Pos[0] < 0 || startNode.Pos[0] >= gridWidth ||
		startNode.Pos[1] < 0 || startNode.Pos[1] >= gridHeight {
		utils.Debugf("A*: FAILED - start out of bounds")
		return nil, false
	}

	if goalNode.Pos[0] < 0 || goalNode.Pos[0] >= gridWidth ||
		goalNode.Pos[1] < 0 || goalNode.Pos[1] >= gridHeight {
		utils.Debugf("A*: FAILED - goal out of bounds")
		return nil, false
	}

	if startNode.Pos[0] == goalNode.Pos[0] && startNode.Pos[1] == goalNode.Pos[1] {
		utils.Debugf("A*: FAILED - start == goal")
		return nil, false
	}

	obstacleMap := make(map[[2]int]bool)
	for _, obs := range obstacles {
		gridObs := worldToGrid(obs, a.config.Resolution, gridWidth, gridHeight)
		for _, cell := range gridObs {
			obstacleMap[cell] = true
		}
	}

	if obstacleMap[goalNode.Pos] {
		utils.Debugf("A*: FAILED - goal is inside obstacle")
		return nil, false
	}

	// If the robot's current position is inside an expanded obstacle, find the
	// nearest free cell so the planner can still route out of it.
	if obstacleMap[startNode.Pos] {
		utils.Debugf("A*: start is inside expanded obstacle, searching for nearest free cell...")
		found := false
		for radius := 1; radius <= 20; radius++ {
			for dx := -radius; dx <= radius; dx++ {
				for dy := -radius; dy <= radius; dy++ {
					if abs(dx) != radius && abs(dy) != radius {
						continue // only check the perimeter of this radius
					}
					candidate := [2]int{startNode.Pos[0] + dx, startNode.Pos[1] + dy}
					if candidate[0] >= 0 && candidate[0] < gridWidth &&
						candidate[1] >= 0 && candidate[1] < gridHeight &&
						!obstacleMap[candidate] {
						utils.Debugf("A*: found free cell at grid=(%d,%d), offset=(%d,%d) from start",candidate[0], candidate[1], dx, dy)
						startNode.Pos = candidate
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			utils.Debugf("A*: FAILED - could not find free cell near start")
			return nil, false
		}
	}

	openSet := &PriorityQueue{}
	heap.Init(openSet)
	heap.Push(openSet, startNode)

	cameFrom := make(map[[2]int]*Node)
	gScore := make(map[[2]int]float64)
	gScore[startNode.Pos] = 0

	iterations := 0
	for openSet.Len() > 0 {
		iterations++
		if iterations > a.config.MaxIterations {
			utils.Debugf("A*: FAILED - exceeded max iterations (%d)",a.config.MaxIterations)
			return nil, false
		}

		current := heap.Pop(openSet).(*Node)

		if current.Pos == goalNode.Pos {
			path := a.reconstructPath(cameFrom, current, offsetX, offsetY)
			utils.Debugf("A*: SUCCESS - found path with %d waypoints in %d iterations",len(path), iterations)
			return path, true
		}

		neighbors := a.getNeighbors(current, gridWidth, gridHeight, obstacleMap)
		for _, neighbor := range neighbors {
			tentativeG := gScore[current.Pos] + a.dist(current.Pos, neighbor.Pos)

			if _, exists := gScore[neighbor.Pos]; !exists || tentativeG < gScore[neighbor.Pos] {
				cameFrom[neighbor.Pos] = current
				gScore[neighbor.Pos] = tentativeG
				neighbor.G = tentativeG
				neighbor.H = a.heuristic(neighbor.Pos, goalNode.Pos)
				neighbor.F = neighbor.G + neighbor.H
				neighbor.Parent = current

				if !a.inOpenSet(openSet, neighbor) {
					heap.Push(openSet, neighbor)
				}
			}
		}
	}

	return nil, false
}

func (a *AStar) getNeighbors(node *Node, width, height int, obstacles map[[2]int]bool) []*Node {
	directions := [][2]int{
		{0, 1}, {1, 0}, {0, -1}, {-1, 0},
		{1, 1}, {1, -1}, {-1, 1}, {-1, -1},
	}

	neighbors := make([]*Node, 0)
	for _, dir := range directions {
		nx, ny := node.Pos[0]+dir[0], node.Pos[1]+dir[1]
		pos := [2]int{nx, ny}

		if nx >= 0 && nx < width && ny >= 0 && ny < height && !obstacles[pos] {
			neighbors = append(neighbors, &Node{Pos: pos})
		}
	}

	return neighbors
}

func (a *AStar) inOpenSet(pq *PriorityQueue, node *Node) bool {
	for _, n := range *pq {
		if n == nil {
			continue
		}
		if n.Pos == node.Pos {
			return true
		}
	}
	return false
}

func (a *AStar) heuristic(aPos, bPos [2]int) float64 {
	return math.Sqrt(float64((aPos[0]-bPos[0])*(aPos[0]-bPos[0])+(aPos[1]-bPos[1])*(aPos[1]-bPos[1]))) * a.config.Resolution
}

func (a *AStar) dist(aPos, bPos [2]int) float64 {
	dx := float64(bPos[0]-aPos[0]) * a.config.Resolution
	dy := float64(bPos[1]-aPos[1]) * a.config.Resolution
	return math.Sqrt(dx*dx + dy*dy)
}

func (a *AStar) reconstructPath(cameFrom map[[2]int]*Node, current *Node, offsetX, offsetY int) [][2]float64 {
	path := make([][2]float64, 0)

	currentPos := [2]float64{float64(current.Pos[0]-offsetX) * a.config.Resolution, float64(current.Pos[1]-offsetY) * a.config.Resolution}
	path = append(path, currentPos)

	for {
		if _, exists := cameFrom[current.Pos]; !exists {
			break
		}
		current = cameFrom[current.Pos]
		pos := [2]float64{float64(current.Pos[0]-offsetX) * a.config.Resolution, float64(current.Pos[1]-offsetY) * a.config.Resolution}
		path = append(path, pos)
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return path
}

// SimplifyPath removes waypoints that are nearly collinear using Douglas-Peucker.
// epsilon is the max perpendicular distance (meters) a point can be from the line
// before it's considered significant.
func SimplifyPath(path [][2]float64, epsilon float64) [][2]float64 {
	if len(path) <= 2 {
		return path
	}

	// Find point with max distance from line between first and last
	maxDist := 0.0
	maxIdx := 0
	start := path[0]
	end := path[len(path)-1]

	for i := 1; i < len(path)-1; i++ {
		d := perpendicularDistance(path[i], start, end)
		if d > maxDist {
			maxDist = d
			maxIdx = i
		}
	}

	if maxDist > epsilon {
		left := SimplifyPath(path[:maxIdx+1], epsilon)
		right := SimplifyPath(path[maxIdx:], epsilon)
		return append(left[:len(left)-1], right...)
	}
	return [][2]float64{start, end}
}

func perpendicularDistance(point, lineStart, lineEnd [2]float64) float64 {
	dx := lineEnd[0] - lineStart[0]
	dy := lineEnd[1] - lineStart[1]
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		dx2 := point[0] - lineStart[0]
		dy2 := point[1] - lineStart[1]
		return math.Sqrt(dx2*dx2 + dy2*dy2)
	}
	return math.Abs(dx*(lineStart[1]-point[1])-(lineStart[0]-point[0])*dy) / length
}

// segmentIntersectsObstacle checks if a line segment from p1 to p2 passes through
// an obstacle's axis-aligned bounding box by sampling at the given resolution.
func segmentIntersectsObstacle(p1, p2 [2]float64, obs Obstacle, resolution float64) bool {
	dx := p2[0] - p1[0]
	dy := p2[1] - p1[1]
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return p1[0] >= obs.WorldTopLeft[0] && p1[0] <= obs.WorldBottomRight[0] &&
			p1[1] >= obs.WorldTopLeft[1] && p1[1] <= obs.WorldBottomRight[1]
	}
	steps := int(length/resolution) + 1
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := p1[0] + t*dx
		y := p1[1] + t*dy
		if x >= obs.WorldTopLeft[0] && x <= obs.WorldBottomRight[0] &&
			y >= obs.WorldTopLeft[1] && y <= obs.WorldBottomRight[1] {
			return true
		}
	}
	return false
}

// ValidateSimplifiedPath checks each segment of a simplified path against expanded
// obstacles. If a simplified segment passes through an obstacle, the original
// unsimplified waypoints for that segment are restored.
func ValidateSimplifiedPath(originalPath, simplifiedPath [][2]float64, obstacles []Obstacle, resolution float64) [][2]float64 {
	if len(simplifiedPath) <= 1 || len(obstacles) == 0 {
		return simplifiedPath
	}

	// Build index mapping: for each simplified waypoint, find its index in the original path.
	// SimplifyPath preserves exact points from the original (Douglas-Peucker property).
	origIndices := make([]int, len(simplifiedPath))
	origIdx := 0
	for si, sp := range simplifiedPath {
		for origIdx < len(originalPath) {
			if originalPath[origIdx] == sp {
				origIndices[si] = origIdx
				break
			}
			origIdx++
		}
	}

	validated := [][2]float64{simplifiedPath[0]}
	for i := 0; i < len(simplifiedPath)-1; i++ {
		p1 := simplifiedPath[i]
		p2 := simplifiedPath[i+1]

		blocked := false
		for _, obs := range obstacles {
			if segmentIntersectsObstacle(p1, p2, obs, resolution) {
				blocked = true
				break
			}
		}

		if blocked {
			// Restore original waypoints between these two simplified points
			startOrig := origIndices[i] + 1
			endOrig := origIndices[i+1]
			for j := startOrig; j <= endOrig; j++ {
				validated = append(validated, originalPath[j])
			}
		} else {
			validated = append(validated, p2)
		}
	}

	return validated
}

func worldToGrid(obs Obstacle, resolution float64, width, height int) [][2]int {
	cells := make([][2]int, 0)

	offsetX := width / 2
	offsetY := height / 2

	x1 := int(obs.WorldTopLeft[0]/resolution) + offsetX
	y1 := int(obs.WorldTopLeft[1]/resolution) + offsetY
	x2 := int(obs.WorldBottomRight[0]/resolution) + offsetX
	y2 := int(obs.WorldBottomRight[1]/resolution) + offsetY

	for x := x1; x <= x2; x++ {
		for y := y1; y <= y2; y++ {
			if x >= 0 && x < width && y >= 0 && y < height {
				cells = append(cells, [2]int{x, y})
			}
		}
	}

	return cells
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
