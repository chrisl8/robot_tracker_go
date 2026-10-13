package planning

import (
	"container/heap"
	"math"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// Node is one cell of the A* search: its grid position, the cost so far (G), the heuristic to the goal (H), their sum (F), and the cell it was reached from.
type Node struct {
	Pos      [2]int
	G        float64
	H        float64
	F        float64
	Parent   *Node
	Obstacle bool
}

// PriorityQueue is a min-heap of Nodes ordered by F, implementing container/heap.Interface for the A* open set.
type PriorityQueue []*Node

// Len returns the number of nodes in the queue.
func (pq PriorityQueue) Len() int { return len(pq) }

// Less orders nodes by F, lowest first; a nil node never sorts first.
func (pq PriorityQueue) Less(i, j int) bool {
	if pq[i] == nil || pq[j] == nil {
		return false
	}
	return pq[i].F < pq[j].F
}

// Swap exchanges the nodes at i and j.
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

// Push adds a *Node to the queue. It is for container/heap; use heap.Push.
func (pq *PriorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*Node))
}

// Pop removes and returns the last node. It is for container/heap; use heap.Pop.
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	*pq = old[0 : n-1]
	return node
}

// AStarConfig sizes the planning grid. GridWidthMeters/GridHeightMeters are the
// grid's real-world extent in *meters* (not cell counts) — the grid is divided
// into Resolution-sized cells, so with the documented defaults (100m, 0.05m)
// the actual grid is 2000x2000 cells. A caller wanting a specific cell count
// must multiply by Resolution first; setting GridWidthMeters to a cell count
// (e.g. 2000) would silently request a 100km-wide grid.
type AStarConfig struct {
	GridWidthMeters  float64
	GridHeightMeters float64
	Resolution       float64
	MaxIterations    int
}

// AStar plans grid paths between two world points around obstacles.
type AStar struct {
	config *AStarConfig
}

// NewAStar returns a planner for the given grid, or one with a 100 m by 100 m grid at 5 cm resolution when config is nil.
func NewAStar(config *AStarConfig) *AStar {
	if config == nil {
		config = &AStarConfig{
			GridWidthMeters:  100,
			GridHeightMeters: 100,
			Resolution:       0.05,
			MaxIterations:    10000,
		}
	}
	return &AStar{config: config}
}

// clearanceCostWeight scales how much a step through the soft clearance zone
// costs: a step right at the hard limit costs (1+weight) times a step in the
// open. Large enough that A* detours around a squeeze when there is room, small
// enough that it still takes the squeeze when there is no other way.
const clearanceCostWeight = 5.0

// PlanOptions says how far a path must stay from things.
//
// Hard is the closest a path may come to an obstacle: cells nearer than that
// are blocked. Soft (>= Hard) is the distance a path would like to keep: cells
// between Hard and Soft stay usable but cost more the nearer they are to Hard,
// so tight gaps are used only when needed. Edge is how far a path must stay
// inside the Area, which is a different thing from an obstacle: it only has
// to keep the robot's tag in view.
type PlanOptions struct {
	Hard, Soft, Edge float64
}

// Plan finds a path from start to goal that stays at least margin from every obstacle, reporting false if there is none. It is PlanWithin without an area.
func (a *AStar) Plan(start, goal [2]float64, obstacles []Obstacle, margin float64) ([][2]float64, bool) {
	return a.PlanWithin(start, goal, obstacles, margin, nil)
}

// PlanWithin is Plan restricted to area (nil means unrestricted): the path
// stays at least margin inside it, and a goal outside that is a failure. A
// start already closer to an edge than margin may stay no closer to it.
func (a *AStar) PlanWithin(start, goal [2]float64, obstacles []Obstacle, margin float64, area *Area) ([][2]float64, bool) {
	return a.PlanWithOptions(start, goal, obstacles, PlanOptions{Hard: margin, Soft: margin, Edge: margin}, area)
}

// PlanWithOptions is PlanWithin with separate obstacle and edge clearances and
// a graded (soft) obstacle clearance; see PlanOptions.
func (a *AStar) PlanWithOptions(start, goal [2]float64, obstacles []Obstacle, opts PlanOptions, area *Area) ([][2]float64, bool) {
	if opts.Soft < opts.Hard {
		opts.Soft = opts.Hard
	}
	gridWidth := int(a.config.GridWidthMeters / a.config.Resolution)
	gridHeight := int(a.config.GridHeightMeters / a.config.Resolution)

	// Offset so world (0,0) maps to grid center, allowing negative world coordinates
	offsetX := gridWidth / 2
	offsetY := gridHeight / 2

	startNode := &Node{Pos: [2]int{a.cellIndex(start[0], offsetX), a.cellIndex(start[1], offsetY)}}
	goalNode := &Node{Pos: [2]int{a.cellIndex(goal[0], offsetX), a.cellIndex(goal[1], offsetY)}}

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

	// outside reports grid cells (judged at their centre, like obstacles)
	// that lie beyond the area the robot may use.
	outside := func(x, y int) bool { return false }
	if area != nil {
		if !area.Contains(goal, opts.Edge) {
			utils.Debugf("A*: FAILED - goal outside the camera area")
			return nil, false
		}
		limit := area.limits(opts.Edge, start)
		outside = func(x, y int) bool {
			d := area.edgeDistances([2]float64{a.cellCentre(x, offsetX), a.cellCentre(y, offsetY)})
			for i := range d {
				if d[i] < limit[i] {
					return true
				}
			}
			return false
		}
	}

	obstacleMap := make(map[[2]int]bool)
	// penalty is 0..1 for cells in the soft zone: 1 at the hard limit, 0 at
	// the soft distance.
	penalty := make(map[[2]int]float64)
	for _, obs := range obstacles {
		for _, cd := range cellsWithin(obs, opts.Soft, a.config.Resolution, gridWidth, gridHeight) {
			if cd.dist <= opts.Hard {
				obstacleMap[cd.cell] = true
				continue
			}
			if f := (opts.Soft - cd.dist) / (opts.Soft - opts.Hard); f > penalty[cd.cell] {
				penalty[cd.cell] = f
			}
		}
	}

	if obstacleMap[goalNode.Pos] {
		utils.Debugf("A*: FAILED - goal is inside obstacle")
		return nil, false
	}

	// Already in the goal's cell: the plan is just to go to the goal. (This used
	// to be reported as a planning failure.)
	if startNode.Pos == goalNode.Pos {
		return [][2]float64{goal}, true
	}

	// If the robot's current position is inside an expanded obstacle, clear
	// obstacle cells around start so A* can plan from the actual position.
	// The robot is physically here, so the expanded-obstacle markup is wrong
	// for cells the robot already occupies. Clearing in expanding radius
	// until we reach free space lets A* route out naturally.
	if obstacleMap[startNode.Pos] {
		utils.Debugf("A*: start is inside expanded obstacle, clearing cells around start...")
		for radius := 0; radius <= 20; radius++ {
			reachedFreeSpace := false
			for dx := -radius; dx <= radius; dx++ {
				for dy := -radius; dy <= radius; dy++ {
					cell := [2]int{startNode.Pos[0] + dx, startNode.Pos[1] + dy}
					if cell[0] >= 0 && cell[0] < gridWidth &&
						cell[1] >= 0 && cell[1] < gridHeight {
						// Only check the perimeter for originally-free cells;
						// inner cells were cleared by previous iterations.
						onPerimeter := dx == -radius || dx == radius || dy == -radius || dy == radius
						if onPerimeter && !obstacleMap[cell] {
							reachedFreeSpace = true
						}
						delete(obstacleMap, cell)
					}
				}
			}
			if reachedFreeSpace {
				break
			}
		}
	}

	openSet := &PriorityQueue{}
	heap.Init(openSet)
	startNode.G = 0
	startNode.H = a.heuristic(startNode.Pos, goalNode.Pos)
	startNode.F = startNode.G + startNode.H
	heap.Push(openSet, startNode)

	cameFrom := make(map[[2]int]*Node)
	gScore := make(map[[2]int]float64)
	gScore[startNode.Pos] = 0

	iterations := 0
	for openSet.Len() > 0 {
		iterations++
		if iterations > a.config.MaxIterations {
			utils.Debugf("A*: FAILED - exceeded max iterations (%d)", a.config.MaxIterations)
			return nil, false
		}

		current := heap.Pop(openSet).(*Node)

		// Lazy deletion: container/heap has no decrease-key, so a cheaper path
		// to an already-queued node is recorded by pushing a fresh entry
		// rather than updating the stale one in place (see the push below).
		// That leaves stale, worse-G copies of the same cell in the heap;
		// skip any entry whose G no longer matches the best known gScore
		// instead of expanding it again.
		if current.G > gScore[current.Pos] {
			continue
		}

		if current.Pos == goalNode.Pos {
			path := a.reconstructPath(cameFrom, current, offsetX, offsetY)
			// Cell centres are on the grid's 5cm lattice; finish at the exact
			// point the operator asked for.
			path[len(path)-1] = goal
			utils.Debugf("A*: SUCCESS - found path with %d waypoints in %d iterations", len(path), iterations)
			return path, true
		}

		neighbors := a.getNeighbors(current, gridWidth, gridHeight, obstacleMap, outside)
		for _, neighbor := range neighbors {
			tentativeG := gScore[current.Pos] + a.dist(current.Pos, neighbor.Pos)*(1+clearanceCostWeight*penalty[neighbor.Pos])

			if existingG, exists := gScore[neighbor.Pos]; !exists || tentativeG < existingG {
				cameFrom[neighbor.Pos] = current
				gScore[neighbor.Pos] = tentativeG
				neighbor.G = tentativeG
				neighbor.H = a.heuristic(neighbor.Pos, goalNode.Pos)
				neighbor.F = neighbor.G + neighbor.H
				neighbor.Parent = current

				heap.Push(openSet, neighbor)
			}
		}
	}

	return nil, false
}

func (a *AStar) getNeighbors(node *Node, width, height int, obstacles map[[2]int]bool, outside func(x, y int) bool) []*Node {
	directions := [][2]int{
		{0, 1}, {1, 0}, {0, -1}, {-1, 0},
		{1, 1}, {1, -1}, {-1, 1}, {-1, -1},
	}

	free := func(x, y int) bool {
		return x >= 0 && x < width && y >= 0 && y < height && !obstacles[[2]int{x, y}] && !outside(x, y)
	}

	neighbors := make([]*Node, 0)
	for _, dir := range directions {
		nx, ny := node.Pos[0]+dir[0], node.Pos[1]+dir[1]
		if !free(nx, ny) {
			continue
		}
		// A diagonal move passes the corner shared with both orthogonal
		// neighbours; if either is blocked the robot would clip that obstacle
		// (or squeeze through a zero-width gap between two of them).
		if dir[0] != 0 && dir[1] != 0 && (!free(node.Pos[0]+dir[0], node.Pos[1]) || !free(node.Pos[0], node.Pos[1]+dir[1])) {
			continue
		}
		neighbors = append(neighbors, &Node{Pos: [2]int{nx, ny}})
	}

	return neighbors
}

// cellIndex is the grid index of the cell containing world coordinate v. It
// floors (not truncates toward zero), matching how obstacles are rasterized in
// worldToGrid; truncation put every negative coordinate one cell off.
func (a *AStar) cellIndex(v float64, offset int) int {
	return int(math.Floor(v/a.config.Resolution)) + offset
}

// cellCentre is the world coordinate of the centre of grid index idx, which is
// the point worldToGrid tests against obstacles.
func (a *AStar) cellCentre(idx, offset int) float64 {
	return (float64(idx-offset) + 0.5) * a.config.Resolution
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

	currentPos := [2]float64{a.cellCentre(current.Pos[0], offsetX), a.cellCentre(current.Pos[1], offsetY)}
	path = append(path, currentPos)

	for {
		if _, exists := cameFrom[current.Pos]; !exists {
			break
		}
		current = cameFrom[current.Pos]
		pos := [2]float64{a.cellCentre(current.Pos[0], offsetX), a.cellCentre(current.Pos[1], offsetY)}
		path = append(path, pos)
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return path
}

// SimplifyPathClear thins an A* path (a waypoint every cell) with
// Douglas-Peucker: a run of points collapses to its two ends when none of them
// is more than epsilon (metres) from the straight line between them AND that
// line is itself clear of the obstacles. Otherwise the run is split at its
// farthest point and each half is tried again, so a chord that would cut a
// corner costs a waypoint or two where it matters, not the whole stretch.
//
// A chord must keep opts.Soft from every obstacle, or as much clearance as the
// raw points it replaces had (never less than opts.Hard): see segmentMargin.
// A path squeezing through a gap is therefore not rejected for being in the
// gap, and one in open ground is not allowed to cut towards an obstacle.
func SimplifyPathClear(path [][2]float64, epsilon float64, obstacles []Obstacle, opts PlanOptions) [][2]float64 {
	if len(path) <= 2 {
		return path
	}
	var simplify func(lo, hi int) [][2]float64
	simplify = func(lo, hi int) [][2]float64 {
		a, b := path[lo], path[hi]
		if hi-lo < 2 {
			return [][2]float64{a, b}
		}
		maxDist, maxIdx := 0.0, (lo+hi)/2
		clearance := math.Inf(1)
		for i := lo; i <= hi; i++ {
			clearance = math.Min(clearance, clearanceAt(path[i], obstacles))
			if i > lo && i < hi {
				if d := perpendicularDistance(path[i], a, b); d > maxDist {
					maxDist, maxIdx = d, i
				}
			}
		}
		if maxDist <= epsilon && chordClear(a, b, obstacles, segmentMargin(opts, clearance)) {
			return [][2]float64{a, b}
		}
		left, right := simplify(lo, maxIdx), simplify(maxIdx, hi)
		return append(left[:len(left)-1], right...)
	}
	return simplify(0, len(path)-1)
}

// chordClear reports whether the segment a-b stays at least margin from every
// obstacle, sampling every 5 cm like the A* grid.
func chordClear(a, b [2]float64, obstacles []Obstacle, margin float64) bool {
	for _, obs := range obstacles {
		if segmentIntersectsObstacle(a, b, obs, margin, 0.05) {
			return false
		}
	}
	return true
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
// segmentIntersectsObstacle checks whether a line segment from p1 to p2 comes
// within margin of the obstacle's exact (possibly rotated) footprint, by
// sampling points along it — the same margin the A* grid rasterization uses,
// so a simplified path segment is only accepted when it would not have been
// blocked in the grid either.
func segmentIntersectsObstacle(p1, p2 [2]float64, obs Obstacle, margin, resolution float64) bool {
	dx := p2[0] - p1[0]
	dy := p2[1] - p1[1]
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return distanceToQuad(p1, obs.Quad) <= margin
	}
	steps := int(length/resolution) + 1
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := p1[0] + t*dx
		y := p1[1] + t*dy
		if distanceToQuad([2]float64{x, y}, obs.Quad) <= margin {
			return true
		}
	}
	return false
}

// cellDist is a grid cell and its centre's distance to an obstacle's footprint.
type cellDist struct {
	cell [2]int
	dist float64
}

// worldToGrid returns the grid cells within margin of obs's exact footprint
// (its Quad — a plain rectangle for an axis-aligned obstacle, an arbitrary
// quadrilateral for a detected, oriented one). This folds rasterization and
// the robot-radius safety margin into a single pass: a cell is blocked when
// its center is within margin of the shape, so there is no separate
// expand-the-rectangle step before this runs.
func worldToGrid(obs Obstacle, margin, resolution float64, width, height int) [][2]int {
	within := cellsWithin(obs, margin, resolution, width, height)
	cells := make([][2]int, 0, len(within))
	for _, cd := range within {
		cells = append(cells, cd.cell)
	}
	return cells
}

// cellsWithin is worldToGrid that also reports each cell's distance.
func cellsWithin(obs Obstacle, margin, resolution float64, width, height int) []cellDist {
	cells := make([]cellDist, 0)

	offsetX := width / 2
	offsetY := height / 2

	tl, br := obs.Quad.Bounds()
	x1 := int(math.Floor((tl[0]-margin)/resolution)) + offsetX
	y1 := int(math.Floor((tl[1]-margin)/resolution)) + offsetY
	x2 := int(math.Ceil((br[0]+margin)/resolution)) + offsetX
	y2 := int(math.Ceil((br[1]+margin)/resolution)) + offsetY

	for x := x1; x <= x2; x++ {
		if x < 0 || x >= width {
			continue
		}
		for y := y1; y <= y2; y++ {
			if y < 0 || y >= height {
				continue
			}
			cellCenter := [2]float64{
				(float64(x-offsetX) + 0.5) * resolution,
				(float64(y-offsetY) + 0.5) * resolution,
			}
			if d := distanceToQuad(cellCenter, obs.Quad); d <= margin {
				cells = append(cells, cellDist{cell: [2]int{x, y}, dist: d})
			}
		}
	}

	return cells
}
