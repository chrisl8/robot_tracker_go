package planning

import (
	"container/heap"
	"testing"
)

func TestNewAStar(t *testing.T) {
	astar := NewAStar(nil)

	if astar == nil {
		t.Fatal("NewAStar returned nil")
	}
	if astar.config.GridWidth != 100 {
		t.Errorf("Default GridWidth = %d, want 100", astar.config.GridWidth)
	}
	if astar.config.GridHeight != 100 {
		t.Errorf("Default GridHeight = %d, want 100", astar.config.GridHeight)
	}
	if astar.config.Resolution != 0.05 {
		t.Errorf("Default Resolution = %f, want 0.05", astar.config.Resolution)
	}
}

func TestNewAStar_WithConfig(t *testing.T) {
	config := &AStarConfig{
		GridWidth:     200,
		GridHeight:    150,
		Resolution:    0.1,
		MaxIterations: 5000,
	}
	astar := NewAStar(config)

	if astar.config.GridWidth != 200 {
		t.Errorf("GridWidth = %d, want 200", astar.config.GridWidth)
	}
	if astar.config.GridHeight != 150 {
		t.Errorf("GridHeight = %d, want 150", astar.config.GridHeight)
	}
}

func TestAStar_Plan_EmptyObstacles(t *testing.T) {
	astar := NewAStar(nil)
	_, found := astar.Plan([2]float64{1, 1}, [2]float64{2, 2}, []Obstacle{})

	if !found {
		t.Error("Plan should find a path between close points")
	}
}

func TestAStar_Plan_SameStartGoal(t *testing.T) {
	astar := NewAStar(nil)
	point := [2]float64{1, 1}
	_, found := astar.Plan(point, point, []Obstacle{})

	if found {
		t.Error("Plan should not find a path when start equals goal")
	}
}

func TestAStar_Plan_BlockedPath(t *testing.T) {
	astar := NewAStar(nil)
	obstacles := []Obstacle{
		{
			Name:             "wall",
			WorldTopLeft:     [2]float64{1.5, 0},
			WorldBottomRight: [2]float64{1.6, 5},
		},
	}
	path, found := astar.Plan([2]float64{1, 1}, [2]float64{2, 3}, obstacles)

	if !found {
		t.Error("Plan should find a path around narrow wall using diagonal movement")
	}
	if len(path) < 2 {
		t.Error("Path should have at least start and goal points")
	}
}

func TestAStar_Plan_ObstacleAtStart(t *testing.T) {
	astar := NewAStar(nil)
	obstacles := []Obstacle{
		{
			Name:             "obs1",
			WorldTopLeft:     [2]float64{0, 0},
			WorldBottomRight: [2]float64{1, 1},
		},
	}
	// The planner handles "start inside obstacle" by clearing expanded-obstacle
	// cells around the start so A* can plan from the actual position. This should
	// succeed and produce a valid path.
	path, found := astar.Plan([2]float64{0.5, 0.5}, [2]float64{3, 3}, obstacles)

	if !found {
		t.Error("Plan should find a path even when start is inside an obstacle (escapes to nearest free cell)")
	}
	if len(path) < 2 {
		t.Errorf("Path should have at least 2 waypoints, got %d", len(path))
	}
}

func TestAStar_Plan_ObstacleAtGoal(t *testing.T) {
	astar := NewAStar(nil)
	obstacles := []Obstacle{
		{
			Name:             "obs1",
			WorldTopLeft:     [2]float64{2.5, 2.5},
			WorldBottomRight: [2]float64{3.5, 3.5},
		},
	}
	_, found := astar.Plan([2]float64{1, 1}, [2]float64{3, 3}, obstacles)

	if found {
		t.Error("Plan should not find a path when obstacle blocks goal")
	}
}

func TestAStar_Plan_LongPath(t *testing.T) {
	astar := NewAStar(nil)
	path, found := astar.Plan([2]float64{0, 0}, [2]float64{4, 4}, []Obstacle{})

	if !found {
		t.Error("Plan should find a path for long distance")
	}
	if len(path) < 2 {
		t.Error("Path should have at least start and goal")
	}
}

func TestNode_Struct(t *testing.T) {
	node := &Node{
		Pos:      [2]int{1, 2},
		G:        1.0,
		H:        2.0,
		F:        3.0,
		Obstacle: false,
	}

	if node.Pos[0] != 1 || node.Pos[1] != 2 {
		t.Errorf("Pos = %v, want [1, 2]", node.Pos)
	}
	if node.G != 1.0 {
		t.Errorf("G = %f, want 1.0", node.G)
	}
	if node.H != 2.0 {
		t.Errorf("H = %f, want 2.0", node.H)
	}
	if node.F != 3.0 {
		t.Errorf("F = %f, want 3.0", node.F)
	}
}

func TestPriorityQueue(t *testing.T) {
	pq := &PriorityQueue{}

	node1 := &Node{F: 3.0}
	node2 := &Node{F: 1.0}
	node3 := &Node{F: 2.0}

	heap.Push(pq, node1)
	heap.Push(pq, node2)
	heap.Push(pq, node3)

	if pq.Len() != 3 {
		t.Errorf("PriorityQueue length = %d, want 3", pq.Len())
	}
}

func TestAStarConfig_Struct(t *testing.T) {
	config := &AStarConfig{
		GridWidth:     100,
		GridHeight:    100,
		Resolution:    0.05,
		MaxIterations: 10000,
	}

	if config.GridWidth != 100 {
		t.Errorf("GridWidth = %d, want 100", config.GridWidth)
	}
	if config.GridHeight != 100 {
		t.Errorf("GridHeight = %d, want 100", config.GridHeight)
	}
	if config.Resolution != 0.05 {
		t.Errorf("Resolution = %f, want 0.05", config.Resolution)
	}
	if config.MaxIterations != 10000 {
		t.Errorf("MaxIterations = %d, want 10000", config.MaxIterations)
	}
}

func TestAStar_Heuristic(t *testing.T) {
	astar := NewAStar(nil)
	h := astar.heuristic([2]int{0, 0}, [2]int{60, 80})

	if h < 4.9 || h > 5.1 {
		t.Errorf("Heuristic for (60, 80) = %f, want ~5.0", h)
	}
}

func TestAStar_Dist(t *testing.T) {
	astar := NewAStar(nil)
	dist := astar.dist([2]int{0, 0}, [2]int{60, 80})

	if dist < 4.9 || dist > 5.1 {
		t.Errorf("dist for (60, 80) = %f, want ~5.0", dist)
	}
}

func TestWorldToGrid(t *testing.T) {
	obs := Obstacle{
		WorldTopLeft:     [2]float64{0, 0},
		WorldBottomRight: [2]float64{1, 1},
	}
	cells := worldToGrid(obs, 0.05, 100, 100)

	if len(cells) == 0 {
		t.Error("worldToGrid should return at least one cell")
	}
}

func TestAStar_Plan_OutsideGrid(t *testing.T) {
	astar := NewAStar(nil)
	_, found := astar.Plan([2]float64{1000, 1000}, [2]float64{2000, 2000}, []Obstacle{})

	if found {
		t.Error("Plan should not find a path when points are outside grid")
	}
}

func TestSegmentIntersectsObstacle(t *testing.T) {
	obs := Obstacle{
		Name:             "box",
		WorldTopLeft:     [2]float64{1.0, 1.0},
		WorldBottomRight: [2]float64{2.0, 2.0},
	}

	tests := []struct {
		name string
		p1   [2]float64
		p2   [2]float64
		want bool
	}{
		{
			name: "segment passes through obstacle",
			p1:   [2]float64{0, 0},
			p2:   [2]float64{3, 3},
			want: true,
		},
		{
			name: "segment misses obstacle entirely",
			p1:   [2]float64{0, 0},
			p2:   [2]float64{0, 3},
			want: false,
		},
		{
			name: "segment ends inside obstacle",
			p1:   [2]float64{0, 0},
			p2:   [2]float64{1.5, 1.5},
			want: true,
		},
		{
			name: "segment starts inside obstacle",
			p1:   [2]float64{1.5, 1.5},
			p2:   [2]float64{3, 3},
			want: true,
		},
		{
			name: "zero length point inside obstacle",
			p1:   [2]float64{1.5, 1.5},
			p2:   [2]float64{1.5, 1.5},
			want: true,
		},
		{
			name: "zero length point outside obstacle",
			p1:   [2]float64{0, 0},
			p2:   [2]float64{0, 0},
			want: false,
		},
		{
			name: "segment grazes obstacle edge",
			p1:   [2]float64{0, 1.5},
			p2:   [2]float64{3, 1.5},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := segmentIntersectsObstacle(tt.p1, tt.p2, obs, 0.05)
			if got != tt.want {
				t.Errorf("segmentIntersectsObstacle(%v, %v) = %v, want %v", tt.p1, tt.p2, got, tt.want)
			}
		})
	}
}

func TestValidateSimplifiedPath(t *testing.T) {
	// Obstacle in the middle that a diagonal shortcut would cross
	obs := []Obstacle{
		{
			Name:             "box",
			WorldTopLeft:     [2]float64{1.0, 0.5},
			WorldBottomRight: [2]float64{2.0, 1.5},
		},
	}

	t.Run("restores waypoints when simplified segment crosses obstacle", func(t *testing.T) {
		// Original path goes around the obstacle
		original := [][2]float64{
			{0, 0},
			{0.5, 0},
			{0.8, 0},
			{0.8, 0.3},
			{1.5, 0.3}, // passes below obstacle (y=0.3 < 0.5)
			{2.5, 0.3},
			{2.5, 1.0},
			{3, 2},
		}
		// Simplified path takes a diagonal shortcut through the obstacle
		simplified := [][2]float64{
			{0, 0},
			{3, 2},
		}

		validated := ValidateSimplifiedPath(original, simplified, obs, 0.05)
		// Should restore the original waypoints since the shortcut crosses the obstacle
		if len(validated) <= 2 {
			t.Errorf("validated path should have more than 2 waypoints (got %d), original waypoints should be restored", len(validated))
		}
	})

	t.Run("keeps simplified path when no obstacles crossed", func(t *testing.T) {
		// Path that doesn't cross any obstacle
		original := [][2]float64{
			{0, 3},
			{0.5, 3},
			{1, 3},
			{1.5, 3},
			{2, 3},
			{3, 3},
		}
		simplified := [][2]float64{
			{0, 3},
			{3, 3},
		}

		validated := ValidateSimplifiedPath(original, simplified, obs, 0.05)
		if len(validated) != 2 {
			t.Errorf("validated path should have 2 waypoints (no obstacles crossed), got %d", len(validated))
		}
	})

	t.Run("handles empty obstacles", func(t *testing.T) {
		simplified := [][2]float64{{0, 0}, {3, 3}}
		validated := ValidateSimplifiedPath(nil, simplified, nil, 0.05)
		if len(validated) != 2 {
			t.Errorf("with no obstacles, should return simplified path unchanged, got %d waypoints", len(validated))
		}
	})

	t.Run("handles single-point path", func(t *testing.T) {
		validated := ValidateSimplifiedPath([][2]float64{{0, 0}}, [][2]float64{{0, 0}}, obs, 0.05)
		if len(validated) != 1 {
			t.Errorf("single-point path should pass through unchanged, got %d waypoints", len(validated))
		}
	})
}
