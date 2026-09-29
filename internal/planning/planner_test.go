package planning

import (
	"math"
	"sync"
	"testing"
	"time"
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
			GridWidthMeters:  200,
			GridHeightMeters: 200,
			Resolution:       0.1,
		},
	}

	planner := NewPlanner(config)

	if planner.globalPlanner == nil {
		t.Error("globalPlanner should be initialized")
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

func TestPlanner_GetPathsWithGoals(t *testing.T) {
	planner := NewPlanner(nil)

	planner.AddRobot(1, [2]float64{1, 1}, 0.18)
	planner.SetGoal(1, [2]float64{2, 2})

	paths := planner.GetPathsWithGoals()

	if paths == nil {
		t.Error("GetPathsWithGoals should not return nil")
	}

	if len(paths) != 1 {
		t.Error("Should have 1 path for robot 1")
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
		name        string
		path        [][2]float64
		pos         [2]float64
		threshold   float64
		wantMore    bool // expect more waypoints remaining
		wantWpIndex int  // expected currentWaypoint after call (-1 = path deleted)
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
			name:        "past intermediate keeps goal waypoint",
			path:        [][2]float64{{0, 0}, {1, 0}},
			pos:         [2]float64{1.0, 0},
			threshold:   0.1,
			wantMore:    true,
			wantWpIndex: 1,
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
			name:        "single waypoint (goal) stays even within threshold",
			path:        [][2]float64{{1, 1}},
			pos:         [2]float64{1.05, 1},
			threshold:   0.1,
			wantMore:    true,
			wantWpIndex: 0,
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

func TestPlanner_ClearPathOnly(t *testing.T) {
	planner := NewPlanner(nil)
	planner.AddRobot(1, [2]float64{1, 1}, 0.18)
	planner.SetGoal(1, [2]float64{3, 3})

	// Verify path and goal exist
	_, hasPath := planner.GetNextWaypoint(1)
	if !hasPath {
		t.Fatal("Should have path after SetGoal")
	}
	_, hasGoal := planner.GetGoal(1)
	if !hasGoal {
		t.Fatal("Should have goal after SetGoal")
	}

	// ClearPathOnly should remove path but keep goal
	planner.ClearPathOnly(1)

	_, hasPath = planner.GetNextWaypoint(1)
	if hasPath {
		t.Error("Path should be cleared after ClearPathOnly")
	}
	_, hasGoal = planner.GetGoal(1)
	if !hasGoal {
		t.Error("Goal should still exist after ClearPathOnly")
	}
}

func TestPlanner_SetPath(t *testing.T) {
	planner := NewPlanner(nil)
	planner.AddRobot(1, [2]float64{0, 0}, 0.18)

	customPath := [][2]float64{{0, 0}, {1, 1}, {2, 2}, {3, 3}}
	planner.SetPath(1, customPath)

	wp, hasPath := planner.GetNextWaypoint(1)
	if !hasPath {
		t.Fatal("Should have path after SetPath")
	}
	if wp != customPath[0] {
		t.Errorf("First waypoint = %v, want %v", wp, customPath[0])
	}

	// Advance (robot reaches the first waypoint) and verify second waypoint
	planner.AdvancePastWaypoints(1, customPath[0], 0.1)
	wp, _ = planner.GetNextWaypoint(1)
	if wp != customPath[1] {
		t.Errorf("Second waypoint = %v, want %v", wp, customPath[1])
	}
}

// TestPlanner_ConcurrentAccess mirrors production: the frame-processing loop
// (AddRobot/AdvancePastWaypoints/GetPathsWithGoals every frame) races against
// webserver request handlers (SetGoal/CompletePath on destination set/clear).
// Run with `go test -race` to catch unsynchronized map access.
func TestPlanner_ConcurrentAccess(t *testing.T) {
	planner := NewPlanner(nil)
	planner.AddRobot(1, [2]float64{0, 0}, 0.18)

	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(3)

	// Simulates the main frame loop.
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			planner.AddRobot(1, [2]float64{float64(i) * 0.01, 0}, 0.18)
			planner.AdvancePastWaypoints(1, [2]float64{float64(i) * 0.01, 0}, 0.1)
			planner.GetPathsWithGoals()
			planner.GetClearance(1)
		}
	}()

	// Simulates webserver destination-set requests.
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			planner.SetGoal(1, [2]float64{float64(i%5) + 1, float64(i%3) + 1})
		}
	}()

	// Simulates webserver destination-clear requests.
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			planner.CompletePath(1)
		}
	}()

	wg.Wait()
}

func TestPlanner_SetPath_OverwriteExisting(t *testing.T) {
	planner := NewPlanner(nil)
	planner.AddRobot(1, [2]float64{1, 1}, 0.18)
	planner.SetGoal(1, [2]float64{3, 3})

	// Advance past first waypoint (the robot is standing on it)
	planner.AdvancePastWaypoints(1, [2]float64{1, 1}, 0.5)

	// Overwrite with new path — should reset waypoint index to 0
	newPath := [][2]float64{{0, 0}, {5, 5}}
	planner.SetPath(1, newPath)

	wp, hasPath := planner.GetNextWaypoint(1)
	if !hasPath {
		t.Fatal("Should have path after SetPath")
	}
	if wp != newPath[0] {
		t.Errorf("Waypoint should be reset to start of new path, got %v want %v", wp, newPath[0])
	}
}

func obstacleAt(name string, x0, y0, x1, y1 float64) Obstacle {
	return Obstacle{Name: name, WorldTopLeft: [2]float64{x0, y0}, WorldBottomRight: [2]float64{x1, y1}}
}

func TestObstaclesEquivalent(t *testing.T) {
	a := obstacleAt("temp_1", 0, 0, 0.2, 0.2)
	b := obstacleAt("temp_2", 1, 1, 1.3, 1.2)

	tests := []struct {
		name string
		x, y []Obstacle
		want bool
	}{
		{"both empty", nil, nil, true},
		{"identical", []Obstacle{a, b}, []Obstacle{a, b}, true},
		{"reordered", []Obstacle{a, b}, []Obstacle{b, a}, true},
		{"renamed only", []Obstacle{a}, []Obstacle{obstacleAt("other", 0, 0, 0.2, 0.2)}, true},
		{"jitter inside epsilon", []Obstacle{a}, []Obstacle{obstacleAt("temp_1", 0.02, -0.02, 0.22, 0.19)}, true},
		{"moved beyond epsilon", []Obstacle{a}, []Obstacle{obstacleAt("temp_1", 0.05, 0, 0.25, 0.2)}, false},
		{"one added", []Obstacle{a}, []Obstacle{a, b}, false},
		{"one removed", []Obstacle{a, b}, []Obstacle{a}, false},
		{"one box cannot match two", []Obstacle{a, a}, []Obstacle{a, b}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := obstaclesEquivalent(tt.x, tt.y, dynamicObstacleEpsilon); got != tt.want {
				t.Errorf("obstaclesEquivalent = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPlanner_SetDynamicObstacles_DoesNotStormReplans guards against flickering
// or jittering detections forcing every robot to replan on every frame.
func TestPlanner_SetDynamicObstacles_DoesNotStormReplans(t *testing.T) {
	p := NewPlanner(nil)
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	p.clock = func() time.Time { return now }
	advance := func(d time.Duration) { now = now.Add(d) }

	a := obstacleAt("temp_1", 0, 0, 0.2, 0.2)
	moved := obstacleAt("temp_1", 0.5, 0.5, 0.7, 0.7)

	p.SetDynamicObstacles([]Obstacle{a})
	if p.replans != 1 {
		t.Fatalf("first change should replan immediately, replans=%d", p.replans)
	}

	// Identical, reordered-equivalent and sub-epsilon jitter: never replans.
	for i := 0; i < 20; i++ {
		advance(100 * time.Millisecond)
		p.SetDynamicObstacles([]Obstacle{obstacleAt("temp_9", 0.01, 0.01, 0.21, 0.19)})
	}
	if p.replans != 1 {
		t.Errorf("jitter caused %d replans, want no more than the first", p.replans)
	}

	// A real change long after the last replan replans immediately...
	moved2 := obstacleAt("temp_1", 1.0, 1.0, 1.2, 1.2)
	advance(100 * time.Millisecond)
	p.SetDynamicObstacles([]Obstacle{moved})
	if p.replans != 2 {
		t.Fatalf("a real change after a quiet period should replan, replans=%d", p.replans)
	}

	// ...but a further change too soon afterwards is deferred, not dropped.
	advance(100 * time.Millisecond)
	p.SetDynamicObstacles([]Obstacle{moved2})
	if p.replans != 2 {
		t.Errorf("replan was not rate limited (replans=%d)", p.replans)
	}
	if len(p.dynamicObstacles) != 1 || p.dynamicObstacles[0].WorldTopLeft[0] != 1.0 {
		t.Error("the new obstacle set should be installed immediately for clearance checks")
	}

	// Later calls with the same set run the deferred replan exactly once.
	advance(minDynamicReplanInterval)
	p.SetDynamicObstacles([]Obstacle{moved2})
	if p.replans != 3 {
		t.Errorf("deferred replan did not run (replans=%d)", p.replans)
	}
	advance(minDynamicReplanInterval)
	p.SetDynamicObstacles([]Obstacle{moved2})
	if p.replans != 3 {
		t.Errorf("replan repeated with nothing new (replans=%d)", p.replans)
	}
}

func TestPlanner_RobotsWithGoals(t *testing.T) {
	planner := NewPlanner(nil)
	if got := planner.RobotsWithGoals(); len(got) != 0 {
		t.Fatalf("no goals set, got %v", got)
	}

	planner.SetGoal(3, [2]float64{1, 1})
	planner.SetGoal(1, [2]float64{2, 2})
	got := planner.RobotsWithGoals()
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Errorf("RobotsWithGoals = %v, want [1 3]", got)
	}

	planner.CompletePath(1)
	got = planner.RobotsWithGoals()
	if len(got) != 1 || got[0] != 3 {
		t.Errorf("after CompletePath(1), RobotsWithGoals = %v, want [3]", got)
	}
}

// A goal set for a robot that has not been tracked yet must be planned from
// where the robot turns out to be, not from the world origin. BroadcastPaths
// calls GetPathsWithGoals every few frames, so it used to plan (and store) a
// path from (0,0) in the gap before the robot appeared; AddRobot then kept it.
func TestPlanner_GoalBeforeRobotSeen_PlansFromActualPosition(t *testing.T) {
	p := NewPlanner(nil)
	p.SetGoal(1, [2]float64{2, 2}) // robot 1 is not tracked yet

	if paths := p.GetPathsWithGoals(); len(paths) != 0 {
		t.Fatalf("path planned for a robot with no known position: %v", paths)
	}

	p.AddRobot(1, [2]float64{1.5, 1.5}, 0.3) // the robot appears

	wp, ok := p.GetNextWaypoint(1)
	if !ok {
		t.Fatal("no path after the robot appeared with a goal already set")
	}
	if math.Hypot(wp[0]-1.5, wp[1]-1.5) > 1.0 {
		t.Errorf("first waypoint %v is far from the robot at (1.5, 1.5): planned from the wrong start", wp)
	}
}
