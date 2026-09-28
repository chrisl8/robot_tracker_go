package planning

import (
	"math"
	"testing"
)

// TestCoordinator_AdjustForConflict_CoincidentRobotsNoNaN verifies that
// adjustForConflict produces a finite, bounded velocity when two robots
// occupy (numerically) the exact same position — plausible after a tracking
// re-ID swap. Before the fix, dist == 0 satisfied the `dist < 0.5` guard and
// then divided by dist, producing NaN/Inf that corrupted the command.
// Regression test for code review finding #14.
func TestCoordinator_AdjustForConflict_CoincidentRobotsNoNaN(t *testing.T) {
	c := NewCoordinator(nil, nil)
	c.AddRobot(1, RobotState{Position: [2]float64{1.0, 1.0}, Diameter: 0.3})
	c.AddRobot(2, RobotState{Position: [2]float64{1.0, 1.0}, Diameter: 0.3})

	commands := map[int][2]float64{
		1: {0.2, 0},
		2: {-0.2, 0},
	}

	adjusted := c.adjustForConflict(1, commands[1], commands)

	if math.IsNaN(adjusted[0]) || math.IsNaN(adjusted[1]) {
		t.Fatalf("adjustForConflict produced NaN for coincident robots: %v", adjusted)
	}
	if math.IsInf(adjusted[0], 0) || math.IsInf(adjusted[1], 0) {
		t.Fatalf("adjustForConflict produced Inf for coincident robots: %v", adjusted)
	}

	velMag := math.Sqrt(adjusted[0]*adjusted[0] + adjusted[1]*adjusted[1])
	if velMag > 0.3+1e-9 {
		t.Fatalf("adjusted velocity magnitude %v exceeds the 0.3 cap", velMag)
	}
}

// TestCoordinator_AdjustForConflict_CoincidentRobotsDivergeDeterministically
// verifies the two coincident robots get opposing escape directions rather
// than both nudging the same way (which would never separate them).
func TestCoordinator_AdjustForConflict_CoincidentRobotsDivergeDeterministically(t *testing.T) {
	c := NewCoordinator(nil, nil)
	c.AddRobot(1, RobotState{Position: [2]float64{1.0, 1.0}, Diameter: 0.3})
	c.AddRobot(2, RobotState{Position: [2]float64{1.0, 1.0}, Diameter: 0.3})

	commands := map[int][2]float64{
		1: {0, 0},
		2: {0, 0},
	}

	adj1 := c.adjustForConflict(1, commands[1], commands)
	adj2 := c.adjustForConflict(2, commands[2], commands)

	if adj1[0] == adj2[0] && adj1[1] == adj2[1] {
		t.Fatalf("expected coincident robots to receive opposing escape directions, got %v and %v", adj1, adj2)
	}
}
