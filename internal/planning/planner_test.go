package planning

import (
	"testing"
)

func TestNewPlanner(t *testing.T) {
	planner := NewPlanner(nil)

	if planner == nil {
		t.Fatal("NewPlanner returned nil")
	}

	if planner.paths == nil {
		t.Error("paths map should be initialized")
	}

	if planner.currentWaypoint == nil {
		t.Error("currentWaypoint map should be initialized")
	}
}

func TestNewPlanner_WithConfig(t *testing.T) {
	config := &PlannerConfig{
		AStarConfig: &AStarConfig{
			GridWidth:  200,
			GridHeight: 200,
			Resolution: 0.1,
		},
		VelocityObstacleConfig: &VelocityObstacleConfig{
			MaxVelocity: 0.5,
		},
		CollisionMargin: 0.1,
	}

	planner := NewPlanner(config)

	if planner.globalPlanner == nil {
		t.Error("globalPlanner should be initialized")
	}

	if planner.localPlanner == nil {
		t.Error("localPlanner should be initialized")
	}

	if planner.collisionDetector == nil {
		t.Error("collisionDetector should be initialized")
	}
}

func TestPlanner_SetGoal_StoresPath(t *testing.T) {
	planner := NewPlanner(nil)

	planner.AddRobot(1, [2]float64{1, 1}, 0.18)
	planner.SetGoal(1, [2]float64{3, 3})

	_, hasPath := planner.GetNextWaypoint(1)
	if !hasPath {
		t.Error("GetNextWaypoint should return true after SetGoal")
	}
}

func TestPlanner_GetNextWaypoint_NoPath(t *testing.T) {
	planner := NewPlanner(nil)

	_, hasPath := planner.GetNextWaypoint(999)
	if hasPath {
		t.Error("GetNextWaypoint should return false when robot has no path")
	}
}

func TestPlanner_AdvanceWaypoint(t *testing.T) {
	planner := NewPlanner(nil)

	planner.AddRobot(1, [2]float64{1, 1}, 0.18)
	planner.SetGoal(1, [2]float64{3, 3})

	_, hasPath := planner.GetNextWaypoint(1)
	if !hasPath {
		t.Fatal("Should have path after SetGoal")
	}

	planner.AdvanceWaypoint(1)

	_, hasPath2 := planner.GetNextWaypoint(1)
	if !hasPath2 {
		t.Error("Should still have path after first advance")
	}
}

func TestPlanner_AdvanceWaypoint_CompletesPath(t *testing.T) {
	planner := NewPlanner(nil)

	planner.AddRobot(1, [2]float64{1, 1}, 0.18)
	planner.SetGoal(1, [2]float64{1.5, 1.5})

	path := planner.paths[1]
	numWaypoints := len(path)
	for i := 0; i < numWaypoints; i++ {
		planner.AdvanceWaypoint(1)
	}

	_, hasPath := planner.GetNextWaypoint(1)
	if hasPath {
		t.Error("Should not have path after completing all waypoints")
	}

	_, hasGoal := planner.coordinator.goals[1]
	if hasGoal {
		t.Error("Goal should be removed after path completion")
	}
}

func TestPlanner_GetPaths(t *testing.T) {
	planner := NewPlanner(nil)

	planner.AddRobot(1, [2]float64{1, 1}, 0.18)
	planner.SetGoal(1, [2]float64{2, 2})

	paths := planner.GetPaths()

	if paths == nil {
		t.Error("GetPaths should not return nil")
	}

	if len(paths) != 1 {
		t.Error("Should have 1 path for robot 1")
	}
}

func TestPlanner_LocalPlanner(t *testing.T) {
	planner := NewPlanner(nil)

	lp := planner.LocalPlanner()

	if lp == nil {
		t.Error("LocalPlanner should not return nil")
	}
}

func TestPlanner_SetGoal_UpdatesCoordinator(t *testing.T) {
	planner := NewPlanner(nil)

	planner.AddRobot(1, [2]float64{1, 1}, 0.18)
	planner.SetGoal(1, [2]float64{5, 5})

	_, exists := planner.coordinator.GetRobotState(1)
	if !exists {
		t.Error("Robot 1 should exist in coordinator")
	}

	goal, hasGoal := planner.coordinator.goals[1]
	if !hasGoal {
		t.Error("Goal should be set in coordinator")
	}

	if goal[0] != 5 || goal[1] != 5 {
		t.Errorf("Goal = (%f, %f), want (5, 5)", goal[0], goal[1])
	}
}

func TestPlanner_MultipleRobots(t *testing.T) {
	planner := NewPlanner(nil)

	planner.AddRobot(1, [2]float64{1, 1}, 0.18)
	planner.AddRobot(2, [2]float64{3, 3}, 0.18)

	planner.SetGoal(1, [2]float64{2, 2})
	planner.SetGoal(2, [2]float64{4, 4})

	_, hasPath1 := planner.GetNextWaypoint(1)
	_, hasPath2 := planner.GetNextWaypoint(2)

	if !hasPath1 || !hasPath2 {
		t.Error("Both robots should have paths")
	}
}

func TestPlanner_AdvancePastWaypoints(t *testing.T) {
	// Helper to set up a planner with a manual path
	setup := func(path [][2]float64) *Planner {
		p := NewPlanner(nil)
		p.AddRobot(1, path[0], 0.18)
		p.coordinator.SetGoal(1, path[len(path)-1])
		p.paths[1] = path
		p.currentWaypoint[1] = 0
		return p
	}

	tests := []struct {
		name          string
		path          [][2]float64
		pos           [2]float64
		threshold     float64
		wantMore      bool // expect more waypoints remaining
		wantWpIndex   int  // expected currentWaypoint after call (-1 = path deleted)
	}{
		{
			name:        "within threshold advances one",
			path:        [][2]float64{{0, 0}, {1, 0}, {2, 0}, {3, 0}},
			pos:         [2]float64{0.05, 0},
			threshold:   0.1,
			wantMore:    true,
			wantWpIndex: 1,
		},
		{
			name:        "overshoot skips to closer waypoint",
			path:        [][2]float64{{0, 0}, {1, 0}, {2, 0}, {3, 0}},
			pos:         [2]float64{1.6, 0}, // past wp0 and wp1, closer to wp2 than wp1
			threshold:   0.1,
			wantMore:    true,
			wantWpIndex: 2,
		},
		{
			name:        "multi-waypoint skip",
			path:        [][2]float64{{0, 0}, {0.05, 0}, {0.1, 0}, {0.25, 0}, {5, 0}},
			pos:         [2]float64{0.12, 0}, // within threshold of wp0,wp1,wp2; wp3 at 0.25 is 0.13m away (>0.1 threshold)
			threshold:   0.1,
			wantMore:    true,
			wantWpIndex: 3,
		},
		{
			name:        "past all waypoints completes path",
			path:        [][2]float64{{0, 0}, {1, 0}},
			pos:         [2]float64{1.0, 0},
			threshold:   0.1,
			wantMore:    false,
			wantWpIndex: -1,
		},
		{
			name:        "off-path sideways stays on current",
			path:        [][2]float64{{0, 0}, {1, 0}, {2, 0}},
			pos:         [2]float64{0, 5}, // far away sideways
			threshold:   0.1,
			wantMore:    true,
			wantWpIndex: 0,
		},
		{
			name:        "single waypoint within threshold completes",
			path:        [][2]float64{{1, 1}},
			pos:         [2]float64{1.05, 1},
			threshold:   0.1,
			wantMore:    false,
			wantWpIndex: -1,
		},
		{
			name:        "single waypoint out of range stays",
			path:        [][2]float64{{1, 1}},
			pos:         [2]float64{0, 0},
			threshold:   0.1,
			wantMore:    true,
			wantWpIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := setup(tt.path)
			got := p.AdvancePastWaypoints(1, tt.pos, tt.threshold)
			if got != tt.wantMore {
				t.Errorf("AdvancePastWaypoints() = %v, want %v", got, tt.wantMore)
			}
			if tt.wantWpIndex == -1 {
				if _, hasPath := p.paths[1]; hasPath {
					t.Error("path should be deleted when complete")
				}
				if _, hasGoal := p.coordinator.goals[1]; hasGoal {
					t.Error("goal should be removed when path completes")
				}
			} else if p.currentWaypoint[1] != tt.wantWpIndex {
				t.Errorf("currentWaypoint = %d, want %d", p.currentWaypoint[1], tt.wantWpIndex)
			}
		})
	}
}

func TestPlanner_AdvancePastWaypoints_NoPath(t *testing.T) {
	p := NewPlanner(nil)
	got := p.AdvancePastWaypoints(999, [2]float64{0, 0}, 0.1)
	if got {
		t.Error("should return false when robot has no path")
	}
}

func TestPlanner_AddRobot_WithExistingGoal(t *testing.T) {
	planner := NewPlanner(nil)

	planner.SetGoal(1, [2]float64{5, 5})

	_, hasPathBefore := planner.GetNextWaypoint(1)
	if hasPathBefore {
		t.Error("Should not have path before robot is added")
	}

	planner.AddRobot(1, [2]float64{1, 1}, 0.18)

	_, hasPathAfter := planner.GetNextWaypoint(1)
	if !hasPathAfter {
		t.Error("Should have path after robot is added with existing goal")
	}
}
