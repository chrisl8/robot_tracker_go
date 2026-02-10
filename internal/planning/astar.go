package planning

import (
	"container/heap"
	"math"
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
	startNode := &Node{Pos: [2]int{int(start[0] / a.config.Resolution), int(start[1] / a.config.Resolution)}}
	goalNode := &Node{Pos: [2]int{int(goal[0] / a.config.Resolution), int(goal[1] / a.config.Resolution)}}

	gridWidth := int(float64(a.config.GridWidth) / a.config.Resolution)
	gridHeight := int(float64(a.config.GridHeight) / a.config.Resolution)

	if startNode.Pos[0] < 0 || startNode.Pos[0] >= gridWidth ||
		startNode.Pos[1] < 0 || startNode.Pos[1] >= gridHeight {
		return nil, false
	}

	if goalNode.Pos[0] < 0 || goalNode.Pos[0] >= gridWidth ||
		goalNode.Pos[1] < 0 || goalNode.Pos[1] >= gridHeight {
		return nil, false
	}

	obstacleMap := make(map[[2]int]bool)
	for _, obs := range obstacles {
		gridObs := worldToGrid(obs, a.config.Resolution, gridWidth, gridHeight)
		for _, cell := range gridObs {
			obstacleMap[cell] = true
		}
	}

	if obstacleMap[startNode.Pos] || obstacleMap[goalNode.Pos] {
		return nil, false
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
			return nil, false
		}

		current := heap.Pop(openSet).(*Node)

		if current.Pos == goalNode.Pos {
			return a.reconstructPath(cameFrom, current, start, goal), true
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

func (a *AStar) reconstructPath(cameFrom map[[2]int]*Node, current *Node, start, goal [2]float64) [][2]float64 {
	path := make([][2]float64, 0)

	currentPos := [2]float64{float64(current.Pos[0]) * a.config.Resolution, float64(current.Pos[1]) * a.config.Resolution}
	path = append(path, currentPos)

	for {
		if _, exists := cameFrom[current.Pos]; !exists {
			break
		}
		current = cameFrom[current.Pos]
		pos := [2]float64{float64(current.Pos[0]) * a.config.Resolution, float64(current.Pos[1]) * a.config.Resolution}
		path = append(path, pos)
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return path
}

func worldToGrid(obs Obstacle, resolution float64, width, height int) [][2]int {
	cells := make([][2]int, 0)

	x1 := int(obs.WorldTopLeft[0] / resolution)
	y1 := int(obs.WorldTopLeft[1] / resolution)
	x2 := int(obs.WorldBottomRight[0] / resolution)
	y2 := int(obs.WorldBottomRight[1] / resolution)

	for x := x1; x <= x2; x++ {
		for y := y1; y <= y2; y++ {
			if x >= 0 && x < width && y >= 0 && y < height {
				cells = append(cells, [2]int{x, y})
			}
		}
	}

	return cells
}
