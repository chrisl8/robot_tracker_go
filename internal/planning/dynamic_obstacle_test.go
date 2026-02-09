package planning

import (
	"math"
	"testing"
)

func TestDynamicObstacle_NewDynamicObstacle(t *testing.T) {
	obs := NewDynamicObstacle(1.0, 2.0, 0.1, 0.2, 0.5, "person", 0.95, false)

	if obs.X != 1.0 {
		t.Errorf("X = %v, want 1.0", obs.X)
	}
	if obs.Y != 2.0 {
		t.Errorf("Y = %v, want 2.0", obs.Y)
	}
	if obs.VX != 0.1 {
		t.Errorf("VX = %v, want 0.1", obs.VX)
	}
	if obs.VY != 0.2 {
		t.Errorf("VY = %v, want 0.2", obs.VY)
	}
	if obs.Radius != 0.5 {
		t.Errorf("Radius = %v, want 0.5", obs.Radius)
	}
	if obs.ClassName != "person" {
		t.Errorf("ClassName = %v, want person", obs.ClassName)
	}
	if obs.Confidence != 0.95 {
		t.Errorf("Confidence = %v, want 0.95", obs.Confidence)
	}
	if obs.IsRobot {
		t.Errorf("IsRobot = %v, want false", obs.IsRobot)
	}
}

func TestDynamicObstacle_ContainsPoint(t *testing.T) {
	obs := NewDynamicObstacle(0, 0, 0, 0, 1.0, "cup", 0.9, false)

	tests := []struct {
		name     string
		px, py   float64
		expected bool
	}{
		{"center", 0, 0, true},
		{"on edge", 1, 0, true},
		{"inside", 0.5, 0.5, true},
		{"outside", 1.5, 0, false},
		{"far outside", 5, 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := obs.ContainsPoint(tt.px, tt.py); got != tt.expected {
				t.Errorf("ContainsPoint(%v, %v) = %v, want %v", tt.px, tt.py, got, tt.expected)
			}
		})
	}
}

func TestDynamicObstacle_DistanceTo(t *testing.T) {
	obs := NewDynamicObstacle(0, 0, 0, 0, 0.5, "person", 0.9, false)

	tests := []struct {
		name     string
		ox, oy   float64
		expected float64
	}{
		{"same point", 0, 0, 0},
		{"1 unit away", 1, 0, 1},
		{"diagonal", 3, 4, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := obs.DistanceTo(tt.ox, tt.oy)
			if math.Abs(got-tt.expected) > 0.001 {
				t.Errorf("DistanceTo(%v, %v) = %v, want %v", tt.ox, tt.oy, got, tt.expected)
			}
		})
	}
}

func TestDynamicObstacle_DistanceToObstacle(t *testing.T) {
	obs1 := NewDynamicObstacle(0, 0, 0, 0, 0.5, "person", 0.9, false)
	obs2 := NewDynamicObstacle(2, 0, 0, 0, 0.5, "cup", 0.8, false)

	dist := obs1.DistanceToObstacle(obs2)
	expected := 2.0 - 0.5 - 0.5

	if math.Abs(dist-expected) > 0.001 {
		t.Errorf("DistanceToObstacle() = %v, want %v", dist, expected)
	}
}

func TestDynamicObstacle_ToRobotState(t *testing.T) {
	obs := NewDynamicObstacle(1.0, 2.0, 0.1, 0.2, 0.5, "person", 0.95, false)

	state := obs.ToRobotState()

	if state.Position[0] != 1.0 {
		t.Errorf("Position[0] = %v, want 1.0", state.Position[0])
	}
	if state.Position[1] != 2.0 {
		t.Errorf("Position[1] = %v, want 2.0", state.Position[1])
	}
	if state.Velocity.VX != 0.1 {
		t.Errorf("VX = %v, want 0.1", state.Velocity.VX)
	}
	if state.Velocity.VY != 0.2 {
		t.Errorf("VY = %v, want 0.2", state.Velocity.VY)
	}
	if state.Diameter != 1.0 {
		t.Errorf("Diameter = %v, want 1.0", state.Diameter)
	}
	if state.RobotID != -1 {
		t.Errorf("RobotID = %v, want -1", state.RobotID)
	}
}
