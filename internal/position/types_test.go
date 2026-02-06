package position

import (
	"math"
	"testing"
)

func TestPoint2D_DistanceTo(t *testing.T) {
	tests := []struct {
		name     string
		p1       Point2D
		p2       Point2D
		expected float64
	}{
		{
			name:     "same point",
			p1:       Point2D{X: 0, Y: 0},
			p2:       Point2D{X: 0, Y: 0},
			expected: 0,
		},
		{
			name:     "unit distance",
			p1:       Point2D{X: 0, Y: 0},
			p2:       Point2D{X: 1, Y: 0},
			expected: 1,
		},
		{
			name:     "diagonal distance",
			p1:       Point2D{X: 0, Y: 0},
			p2:       Point2D{X: 3, Y: 4},
			expected: 5,
		},
		{
			name:     "negative coordinates",
			p1:       Point2D{X: -1, Y: -1},
			p2:       Point2D{X: -4, Y: -5},
			expected: 5,
		},
		{
			name:     "large numbers",
			p1:       Point2D{X: 1000, Y: 2000},
			p2:       Point2D{X: 1000, Y: 2000},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.p1.DistanceTo(tt.p2)
			if math.Abs(result-tt.expected) > 0.001 {
				t.Errorf("DistanceTo() = %f, want %f", result, tt.expected)
			}
		})
	}
}

func TestPoint2D_Add(t *testing.T) {
	tests := []struct {
		name     string
		p1       Point2D
		p2       Point2D
		expected Point2D
	}{
		{
			name:     "basic addition",
			p1:       Point2D{X: 1, Y: 2},
			p2:       Point2D{X: 3, Y: 4},
			expected: Point2D{X: 4, Y: 6},
		},
		{
			name:     "zero addition",
			p1:       Point2D{X: 5, Y: 5},
			p2:       Point2D{X: 0, Y: 0},
			expected: Point2D{X: 5, Y: 5},
		},
		{
			name:     "negative addition",
			p1:       Point2D{X: 10, Y: 10},
			p2:       Point2D{X: -3, Y: -4},
			expected: Point2D{X: 7, Y: 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.p1.Add(tt.p2)
			if result.X != tt.expected.X || result.Y != tt.expected.Y {
				t.Errorf("Add() = (%f, %f), want (%f, %f)", result.X, result.Y, tt.expected.X, tt.expected.Y)
			}
		})
	}
}

func TestPoint2D_Sub(t *testing.T) {
	tests := []struct {
		name     string
		p1       Point2D
		p2       Point2D
		expected Point2D
	}{
		{
			name:     "basic subtraction",
			p1:       Point2D{X: 5, Y: 6},
			p2:       Point2D{X: 2, Y: 3},
			expected: Point2D{X: 3, Y: 3},
		},
		{
			name:     "zero subtraction",
			p1:       Point2D{X: 5, Y: 5},
			p2:       Point2D{X: 0, Y: 0},
			expected: Point2D{X: 5, Y: 5},
		},
		{
			name:     "negative result",
			p1:       Point2D{X: 2, Y: 3},
			p2:       Point2D{X: 5, Y: 6},
			expected: Point2D{X: -3, Y: -3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.p1.Sub(tt.p2)
			if result.X != tt.expected.X || result.Y != tt.expected.Y {
				t.Errorf("Sub() = (%f, %f), want (%f, %f)", result.X, result.Y, tt.expected.X, tt.expected.Y)
			}
		})
	}
}

func TestPoint2D_Scale(t *testing.T) {
	tests := []struct {
		name     string
		p        Point2D
		factor   float64
		expected Point2D
	}{
		{
			name:     "scale by 2",
			p:        Point2D{X: 3, Y: 4},
			factor:   2,
			expected: Point2D{X: 6, Y: 8},
		},
		{
			name:     "scale by 0.5",
			p:        Point2D{X: 4, Y: 6},
			factor:   0.5,
			expected: Point2D{X: 2, Y: 3},
		},
		{
			name:     "scale by -1",
			p:        Point2D{X: 5, Y: -5},
			factor:   -1,
			expected: Point2D{X: -5, Y: 5},
		},
		{
			name:     "scale by 0",
			p:        Point2D{X: 10, Y: 20},
			factor:   0,
			expected: Point2D{X: 0, Y: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.p.Scale(tt.factor)
			if result.X != tt.expected.X || result.Y != tt.expected.Y {
				t.Errorf("Scale(%f) = (%f, %f), want (%f, %f)", tt.factor, result.X, result.Y, tt.expected.X, tt.expected.Y)
			}
		})
	}
}

func TestPoint2D_Length(t *testing.T) {
	tests := []struct {
		name     string
		p        Point2D
		expected float64
	}{
		{
			name:     "zero length",
			p:        Point2D{X: 0, Y: 0},
			expected: 0,
		},
		{
			name:     "unit length",
			p:        Point2D{X: 1, Y: 0},
			expected: 1,
		},
		{
			name:     "3-4-5 triangle",
			p:        Point2D{X: 3, Y: 4},
			expected: 5,
		},
		{
			name:     "negative coordinates",
			p:        Point2D{X: -3, Y: -4},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.p.Length()
			if math.Abs(result-tt.expected) > 0.001 {
				t.Errorf("Length() = %f, want %f", result, tt.expected)
			}
		})
	}
}

func TestPoint2D_Normalize(t *testing.T) {
	tests := []struct {
		name      string
		p         Point2D
		expectedX float64
		expectedY float64
	}{
		{
			name:      "zero vector",
			p:         Point2D{X: 0, Y: 0},
			expectedX: 0,
			expectedY: 0,
		},
		{
			name:      "unit vector",
			p:         Point2D{X: 1, Y: 0},
			expectedX: 1,
			expectedY: 0,
		},
		{
			name:      "normalized 3-4-5",
			p:         Point2D{X: 3, Y: 4},
			expectedX: 0.6,
			expectedY: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.p.Normalize()
			if math.Abs(result.X-tt.expectedX) > 0.001 || math.Abs(result.Y-tt.expectedY) > 0.001 {
				t.Errorf("Normalize() = (%f, %f), want (%f, %f)", result.X, result.Y, tt.expectedX, tt.expectedY)
			}
		})
	}
}

func TestPoint2D_Dot(t *testing.T) {
	tests := []struct {
		name     string
		p1       Point2D
		p2       Point2D
		expected float64
	}{
		{
			name:     "perpendicular vectors",
			p1:       Point2D{X: 1, Y: 0},
			p2:       Point2D{X: 0, Y: 1},
			expected: 0,
		},
		{
			name:     "parallel vectors",
			p1:       Point2D{X: 2, Y: 3},
			p2:       Point2D{X: 4, Y: 6},
			expected: 26,
		},
		{
			name:     "opposite vectors",
			p1:       Point2D{X: 1, Y: 1},
			p2:       Point2D{X: -1, Y: -1},
			expected: -2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.p1.Dot(tt.p2)
			if math.Abs(result-tt.expected) > 0.001 {
				t.Errorf("Dot() = %f, want %f", result, tt.expected)
			}
		})
	}
}

func TestPoint2D_String(t *testing.T) {
	p := Point2D{X: 1.234, Y: 5.678}
	result := p.String()
	expected := "(1.234, 5.678)"
	if result != expected {
		t.Errorf("String() = %s, want %s", result, expected)
	}
}

func TestPose_Struct(t *testing.T) {
	pose := Pose{
		Position: Point2D{X: 10, Y: 20},
		Rotation: 45.0,
	}

	if pose.Position.X != 10 || pose.Position.Y != 20 {
		t.Errorf("Position = (%f, %f), want (10, 20)", pose.Position.X, pose.Position.Y)
	}
	if pose.Rotation != 45.0 {
		t.Errorf("Rotation = %f, want 45.0", pose.Rotation)
	}
}

func TestVelocity_Struct(t *testing.T) {
	vel := Velocity{
		VX: 1.5,
		VY: 2.5,
	}

	if vel.VX != 1.5 || vel.VY != 2.5 {
		t.Errorf("Velocity = (%f, %f), want (1.5, 2.5)", vel.VX, vel.VY)
	}
}

func TestRobotState_Struct(t *testing.T) {
	state := RobotState{
		RobotID:   1,
		Position:  Point2D{X: 100, Y: 200},
		Velocity:  Velocity{VX: 10, VY: 20},
		Timestamp: 1234.5,
		Diameter:  0.3,
	}

	if state.RobotID != 1 {
		t.Errorf("RobotID = %d, want 1", state.RobotID)
	}
	if state.Position.X != 100 || state.Position.Y != 200 {
		t.Errorf("Position = (%f, %f), want (100, 200)", state.Position.X, state.Position.Y)
	}
	if state.Velocity.VX != 10 || state.Velocity.VY != 20 {
		t.Errorf("Velocity = (%f, %f), want (10, 20)", state.Velocity.VX, state.Velocity.VY)
	}
	if state.Timestamp != 1234.5 {
		t.Errorf("Timestamp = %f, want 1234.5", state.Timestamp)
	}
	if state.Diameter != 0.3 {
		t.Errorf("Diameter = %f, want 0.3", state.Diameter)
	}
}

func TestObstacle_Struct(t *testing.T) {
	obstacle := Obstacle{
		Name:              "table",
		WorldTopLeft:      Point2D{X: 0, Y: 0},
		WorldBottomRight:  Point2D{X: 1, Y: 1},
		PixelsTopLeft:     [2]int{100, 100},
		PixelsBottomRight: [2]int{200, 200},
	}

	if obstacle.Name != "table" {
		t.Errorf("Name = %s, want table", obstacle.Name)
	}
	if obstacle.WorldTopLeft.X != 0 || obstacle.WorldTopLeft.Y != 0 {
		t.Errorf("WorldTopLeft = (%f, %f), want (0, 0)", obstacle.WorldTopLeft.X, obstacle.WorldTopLeft.Y)
	}
	if obstacle.PixelsTopLeft[0] != 100 || obstacle.PixelsTopLeft[1] != 100 {
		t.Errorf("PixelsTopLeft = [%d, %d], want [100, 100]", obstacle.PixelsTopLeft[0], obstacle.PixelsTopLeft[1])
	}
}
