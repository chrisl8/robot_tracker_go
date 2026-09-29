package planning

import (
	"math"
	"testing"
)

func TestClearance(t *testing.T) {
	box := NewRectObstacle("box", [2]float64{2, 2}, [2]float64{4, 4})

	tests := []struct {
		name      string
		robot     RobotState
		obstacles []Obstacle
		want      float64
	}{
		{"no obstacles is effectively unbounded", RobotState{Position: [2]float64{0, 0}, Diameter: 0.3}, nil, 1e10},
		{"gap is distance minus robot radius", RobotState{Position: [2]float64{0, 3}, Diameter: 0.4}, []Obstacle{box}, 2 - 0.2},
		{"zero-size robot measures the raw distance", RobotState{Position: [2]float64{0, 3}}, []Obstacle{box}, 2},
		{"overlapping robot is negative", RobotState{Position: [2]float64{2, 3}, Diameter: 0.5}, []Obstacle{box}, -0.25},
		{"robot inside the obstacle", RobotState{Position: [2]float64{3, 3}}, []Obstacle{box}, 0},
		{
			"nearest of several obstacles wins",
			RobotState{Position: [2]float64{0, 3}},
			[]Obstacle{NewRectObstacle("far", [2]float64{9, 2}, [2]float64{10, 4}), box},
			2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clearance(tt.robot, tt.obstacles); math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("clearance = %v, want %v", got, tt.want)
			}
		})
	}
}

// The point of the Quad cutover: for an elongated obstacle at 45 degrees,
// clearance must use its true (tight) footprint, not its axis-aligned bounding
// box, so a robot the old code would have stopped for now correctly passes.
func TestClearance_RotatedObstacle_TighterThanItsAABB(t *testing.T) {
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

	// This point is well clear of the actual stick but inside its AABB.
	robot := RobotState{Position: [2]float64{0.6, -0.6}}
	if robot.Position[0] < aabbTL[0] || robot.Position[0] > aabbBR[0] ||
		robot.Position[1] < aabbTL[1] || robot.Position[1] > aabbBR[1] {
		t.Fatal("test setup: the point should be inside the stick's AABB")
	}

	if got := clearance(robot, []Obstacle{stick}); got <= 0.1 {
		t.Errorf("clearance = %.3f, want well above 0 (clear of the actual stick shape)", got)
	}
}

func TestNewRectObstacle_QuadMatchesCorners(t *testing.T) {
	obs := NewRectObstacle("r", [2]float64{1, 2}, [2]float64{3, 5})

	tl, br := obs.Quad.Bounds()
	if tl != [2]float64{1, 2} || br != [2]float64{3, 5} {
		t.Errorf("Quad bounds = %v..%v, want (1,2)..(3,5)", tl, br)
	}
	if obs.WorldTopLeft != tl || obs.WorldBottomRight != br {
		t.Error("cached WorldTopLeft/BottomRight must match the Quad's bounds")
	}
}
