package planning

import (
	"testing"
)

func TestNewCollisionDetector(t *testing.T) {
	cd := NewCollisionDetector(0.1)

	if cd == nil {
		t.Fatal("NewCollisionDetector returned nil")
	}
	if cd.margin != 0.1 {
		t.Errorf("margin = %f, want 0.1", cd.margin)
	}
}

func TestNewCollisionDetector_DefaultMargin(t *testing.T) {
	cd := NewCollisionDetector(0)

	if cd.margin != 0.05 {
		t.Errorf("Default margin = %f, want 0.05", cd.margin)
	}
}

func TestCollisionDetector_IsCollision_NoCollision(t *testing.T) {
	cd := NewCollisionDetector(0.1)
	robot := RobotState{
		Position: [2]float64{5, 5},
		Diameter: 0.2,
	}
	obstacle := Obstacle{
		WorldTopLeft:     [2]float64{10, 10},
		WorldBottomRight: [2]float64{20, 20},
	}

	if cd.IsCollision(robot, obstacle) {
		t.Error("Robot far from obstacle should not collide")
	}
}

func TestCollisionDetector_IsCollision_Colliding(t *testing.T) {
	cd := NewCollisionDetector(0.05)
	robot := RobotState{
		Position: [2]float64{5, 5},
		Diameter: 2.0,
	}
	obstacle := Obstacle{
		WorldTopLeft:     [2]float64{4, 4},
		WorldBottomRight: [2]float64{6, 6},
	}

	if !cd.IsCollision(robot, obstacle) {
		t.Error("Robot inside obstacle should collide")
	}
}

func TestCollisionDetector_IsCollision_Edge(t *testing.T) {
	cd := NewCollisionDetector(0.1)
	robot := RobotState{
		Position: [2]float64{5, 5},
		Diameter: 0.2,
	}
	obstacle := Obstacle{
		WorldTopLeft:     [2]float64{5.15, 10},
		WorldBottomRight: [2]float64{10, 15},
	}

	if cd.IsCollision(robot, obstacle) {
		t.Error("Robot just outside obstacle should not collide")
	}
}

func TestCollisionDetector_IsCollisionWithPath(t *testing.T) {
	cd := NewCollisionDetector(0.1)
	robot := RobotState{
		Position: [2]float64{0, 0},
		Diameter: 0.5,
	}
	path := [][2]float64{
		{1, 1},
		{2, 2},
		{5, 5},
	}
	obstacle := Obstacle{
		WorldTopLeft:     [2]float64{4, 4},
		WorldBottomRight: [2]float64{6, 6},
	}

	if !cd.IsCollisionWithPath(robot, path, obstacle) {
		t.Error("Path through obstacle should collide")
	}
}

func TestCollisionDetector_CheckAllCollisions(t *testing.T) {
	cd := NewCollisionDetector(0.1)
	robot := RobotState{
		Position: [2]float64{5, 5},
		Diameter: 0.5,
	}
	obstacles := []Obstacle{
		{WorldTopLeft: [2]float64{10, 10}, WorldBottomRight: [2]float64{20, 20}},
		{WorldTopLeft: [2]float64{4, 4}, WorldBottomRight: [2]float64{6, 6}},
	}

	collisions := cd.CheckAllCollisions(robot, obstacles)

	if len(collisions) != 1 {
		t.Errorf("Expected 1 collision, got %d", len(collisions))
	}
	if collisions[0].Name != "" {
		t.Error("Collision should be with second obstacle")
	}
}

func TestCollisionDetector_WillCollide(t *testing.T) {
	cd := NewCollisionDetector(0.1)
	robot := RobotState{
		Position: [2]float64{0, 0},
		Diameter: 0.5,
	}
	velocity := [2]float64{10, 10}
	obstacles := []Obstacle{
		{WorldTopLeft: [2]float64{5, 5}, WorldBottomRight: [2]float64{15, 15}},
	}

	if !cd.WillCollide(robot, velocity, obstacles, 1.0) {
		t.Error("Robot moving toward obstacle should collide")
	}
}

func TestCollisionDetector_DistanceToObstacle(t *testing.T) {
	cd := NewCollisionDetector(0.1)
	robot := RobotState{
		Position: [2]float64{5, 5},
		Diameter: 0.2,
	}
	obstacle := Obstacle{
		WorldTopLeft:     [2]float64{10, 10},
		WorldBottomRight: [2]float64{20, 20},
	}

	dist := cd.DistanceToObstacle(robot, obstacle)

	if dist < 6.9 || dist > 7.1 {
		t.Errorf("Distance = %f, want ~7.0", dist)
	}
}

func TestCollisionDetector_IsPointInObstacle(t *testing.T) {
	cd := NewCollisionDetector(0.1)
	obstacle := Obstacle{
		WorldTopLeft:     [2]float64{0, 0},
		WorldBottomRight: [2]float64{10, 10},
	}

	if !cd.IsPointInObstacle([2]float64{5, 5}, obstacle) {
		t.Error("Point inside obstacle should be in obstacle")
	}
	if cd.IsPointInObstacle([2]float64{15, 15}, obstacle) {
		t.Error("Point outside obstacle should not be in obstacle")
	}
}

func TestCollisionDetector_ExpandObstacle(t *testing.T) {
	cd := NewCollisionDetector(0.1)
	obstacle := Obstacle{
		WorldTopLeft:     [2]float64{5, 5},
		WorldBottomRight: [2]float64{10, 10},
	}

	expanded := cd.ExpandObstacle(obstacle, 0.5)

	if expanded.WorldTopLeft[0] != 4.5 {
		t.Errorf("Expanded TopLeft X = %f, want 4.5", expanded.WorldTopLeft[0])
	}
	if expanded.WorldBottomRight[0] != 10.5 {
		t.Errorf("Expanded BottomRight X = %f, want 10.5", expanded.WorldBottomRight[0])
	}
}

func TestCollisionDetector_GetClearance(t *testing.T) {
	cd := NewCollisionDetector(0.1)
	robot := RobotState{
		Position: [2]float64{5, 5},
		Diameter: 0.2,
	}
	obstacles := []Obstacle{
		{WorldTopLeft: [2]float64{10, 10}, WorldBottomRight: [2]float64{20, 20}},
	}

	clearance := cd.GetClearance(robot, obstacles)

	if clearance < 6.9 || clearance > 7.1 {
		t.Errorf("Clearance = %f, want ~7.0", clearance)
	}
}

func TestObstacle_Struct(t *testing.T) {
	obs := Obstacle{
		Name:              "test",
		WorldTopLeft:      [2]float64{0, 0},
		WorldBottomRight:  [2]float64{10, 10},
		PixelsTopLeft:     [2]int{100, 100},
		PixelsBottomRight: [2]int{200, 200},
	}

	if obs.Name != "test" {
		t.Errorf("Name = %s, want test", obs.Name)
	}
	if obs.WorldTopLeft[0] != 0 {
		t.Errorf("WorldTopLeft[0] = %f, want 0", obs.WorldTopLeft[0])
	}
}

func TestRobotState_Struct(t *testing.T) {
	robot := RobotState{
		Position: [2]float64{1, 2},
		Velocity: Velocity{VX: 0.1, VY: 0.2},
		RobotID:  1,
		Diameter: 0.5,
	}

	if robot.Position[0] != 1 {
		t.Errorf("Position[0] = %f, want 1", robot.Position[0])
	}
	if robot.Velocity.VX != 0.1 {
		t.Errorf("Velocity.VX = %f, want 0.1", robot.Velocity.VX)
	}
	if robot.RobotID != 1 {
		t.Errorf("RobotID = %d, want 1", robot.RobotID)
	}
	if robot.Diameter != 0.5 {
		t.Errorf("Diameter = %f, want 0.5", robot.Diameter)
	}
}

func TestVelocity_Struct(t *testing.T) {
	vel := Velocity{VX: 1.0, VY: 2.0}

	if vel.VX != 1.0 {
		t.Errorf("VX = %f, want 1.0", vel.VX)
	}
	if vel.VY != 2.0 {
		t.Errorf("VY = %f, want 2.0", vel.VY)
	}
}
