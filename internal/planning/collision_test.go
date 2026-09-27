package planning

import (
	"math"
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
	obstacle := NewRectObstacle("", [2]float64{10, 10}, [2]float64{20, 20})

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
	obstacle := NewRectObstacle("", [2]float64{4, 4}, [2]float64{6, 6})

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
	obstacle := NewRectObstacle("", [2]float64{5.15, 10}, [2]float64{10, 15})

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
	obstacle := NewRectObstacle("", [2]float64{4, 4}, [2]float64{6, 6})

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
		NewRectObstacle("", [2]float64{10, 10}, [2]float64{20, 20}),
		NewRectObstacle("", [2]float64{4, 4}, [2]float64{6, 6}),
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
		NewRectObstacle("", [2]float64{5, 5}, [2]float64{15, 15}),
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
	obstacle := NewRectObstacle("", [2]float64{10, 10}, [2]float64{20, 20})

	dist := cd.DistanceToObstacle(robot, obstacle)

	if dist < 6.9 || dist > 7.1 {
		t.Errorf("Distance = %f, want ~7.0", dist)
	}
}

func TestCollisionDetector_IsPointInObstacle(t *testing.T) {
	cd := NewCollisionDetector(0.1)
	obstacle := NewRectObstacle("", [2]float64{0, 0}, [2]float64{10, 10})

	if !cd.IsPointInObstacle([2]float64{5, 5}, obstacle) {
		t.Error("Point inside obstacle should be in obstacle")
	}
	if cd.IsPointInObstacle([2]float64{15, 15}, obstacle) {
		t.Error("Point outside obstacle should not be in obstacle")
	}
}

func TestCollisionDetector_GetClearance(t *testing.T) {
	cd := NewCollisionDetector(0.1)
	robot := RobotState{
		Position: [2]float64{5, 5},
		Diameter: 0.2,
	}
	obstacles := []Obstacle{
		NewRectObstacle("", [2]float64{10, 10}, [2]float64{20, 20}),
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

// TestCollisionDetector_RotatedObstacle_TighterThanItsAABB is the point of the
// Quad cutover: for an elongated obstacle at 45 degrees, distance/clearance
// checks must use its true (tight) footprint, not its axis-aligned bounding
// box, so a robot the old code would have stopped for now correctly passes.
func TestCollisionDetector_RotatedObstacle_TighterThanItsAABB(t *testing.T) {
	cd := NewCollisionDetector(0)

	// A 2 x 0.2 stick centred at the origin, rotated 45 degrees: its AABB
	// diagonal is about 2.0, but the stick itself is only 0.2 wide.
	const l, w = 1.0, 0.1 // half-length, half-width
	local := [4][2]float64{{-l, -w}, {l, -w}, {l, w}, {-l, w}}
	c, s := math.Cos(math.Pi/4), math.Sin(math.Pi/4)
	var quad Quad
	for i, p := range local {
		quad[i] = [2]float64{p[0]*c - p[1]*s, p[0]*s + p[1]*c}
	}
	stick := Obstacle{Name: "stick", Quad: quad}
	aabbTL, aabbBR := stick.Quad.Bounds()
	stick.WorldTopLeft, stick.WorldBottomRight = aabbTL, aabbBR
	t.Logf("stick AABB: %v to %v", aabbTL, aabbBR)

	// This point is well clear of the actual stick but inside its AABB.
	robot := RobotState{Position: [2]float64{0.6, -0.6}, Diameter: 0}

	if cd.IsCollision(robot, stick) {
		t.Error("a point outside the rotated stick, but inside its AABB, should not collide")
	}
	if d := cd.DistanceToObstacle(robot, stick); d <= 0 {
		t.Errorf("DistanceToObstacle = %.3f, want > 0 (clear of the actual stick shape)", d)
	}

	// Sanity: the old AABB-based formula WOULD have called this a collision,
	// proving the fix actually changes behaviour for rotated obstacles.
	rectTL, rectBR := aabbTL, aabbBR
	clampedX := math.Max(rectTL[0], math.Min(robot.Position[0], rectBR[0]))
	clampedY := math.Max(rectTL[1], math.Min(robot.Position[1], rectBR[1]))
	oldDist := math.Hypot(robot.Position[0]-clampedX, robot.Position[1]-clampedY)
	if oldDist != 0 {
		t.Fatalf("test setup: point should be inside the AABB (old dist %.3f)", oldDist)
	}
}

func TestCollisionDetector_IsPointInObstacle_RotatedQuad(t *testing.T) {
	cd := NewCollisionDetector(0)
	diamond := Obstacle{Quad: Quad{{5, 0}, {10, 5}, {5, 10}, {0, 5}}}

	if !cd.IsPointInObstacle([2]float64{5, 5}, diamond) {
		t.Error("center of the diamond should be inside")
	}
	if cd.IsPointInObstacle([2]float64{1, 1}, diamond) {
		t.Error("corner of the diamond's AABB, outside the diamond itself, should not be inside")
	}
}
