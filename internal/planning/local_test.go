package planning

import (
	"math"
	"testing"
)

// TestLocalPlanner_MultiObstaclePenetration_CombinesEscapeDirections verifies
// that when the robot is inside the safety margin of two obstacles on
// opposite sides, the escape velocity accounts for both rather than only the
// last obstacle processed. Regression test for the "last obstacle wins" bug
// (code review finding #4).
func TestLocalPlanner_MultiObstaclePenetration_CombinesEscapeDirections(t *testing.T) {
	lp := NewLocalPlanner(&VelocityObstacleConfig{
		TimeHorizon:     2.0,
		SafetyMargin:    0.15,
		MaxVelocity:     0.5,
		MaxAcceleration: 0.3,
		TimeStep:        0.1,
	})

	robot := RobotState{
		Position: [2]float64{0, 0},
		Diameter: 0.3,
	}

	// Two obstacles penetrating the safety margin from the +X side, one
	// deeper than the other. A "last obstacle wins" implementation would
	// react only to whichever obstacle happens to be last in the slice,
	// ignoring how deep the other one penetrates. The combined escape should
	// point away from both (i.e. -X), regardless of slice order or which one
	// penetrates deepest.
	shallow := RobotState{Position: [2]float64{0.28, 0}, Diameter: 0.3} // barely penetrating
	deep := RobotState{Position: [2]float64{0.15, 0}, Diameter: 0.3}    // deeply penetrating

	velShallowFirst := lp.applyVelocityObstacles(robot, [2]float64{0.3, 0}, []RobotState{shallow, deep})
	velDeepFirst := lp.applyVelocityObstacles(robot, [2]float64{0.3, 0}, []RobotState{deep, shallow})

	for _, vel := range [][2]float64{velShallowFirst, velDeepFirst} {
		velMag := math.Sqrt(vel[0]*vel[0] + vel[1]*vel[1])
		if velMag < 1e-6 {
			t.Fatalf("expected a non-zero escape velocity, got %v", vel)
		}
		if vel[0] >= 0 {
			t.Errorf("expected escape velocity to point away from both +X obstacles (-X), got vel=%v", vel)
		}
	}

	const eps = 1e-9
	if math.Abs(velShallowFirst[0]-velDeepFirst[0]) > eps || math.Abs(velShallowFirst[1]-velDeepFirst[1]) > eps {
		t.Errorf("escape velocity depends on obstacle order (last-obstacle-wins bug): shallow-first=%v deep-first=%v", velShallowFirst, velDeepFirst)
	}
}

// TestLocalPlanner_MultiObstaclePenetration_OrderIndependent verifies the
// combined escape velocity doesn't depend on the order obstacles are given
// in, which the old "overwrite each iteration" implementation was
// susceptible to.
func TestLocalPlanner_MultiObstaclePenetration_OrderIndependent(t *testing.T) {
	lp := NewLocalPlanner(nil)

	robot := RobotState{Position: [2]float64{0, 0}, Diameter: 0.3}

	obsA := RobotState{Position: [2]float64{0.1, 0.05}, Diameter: 0.3}
	obsB := RobotState{Position: [2]float64{-0.05, 0.1}, Diameter: 0.3}

	velAB := lp.applyVelocityObstacles(robot, [2]float64{0.2, 0.2}, []RobotState{obsA, obsB})
	velBA := lp.applyVelocityObstacles(robot, [2]float64{0.2, 0.2}, []RobotState{obsB, obsA})

	const eps = 1e-9
	if math.Abs(velAB[0]-velBA[0]) > eps || math.Abs(velAB[1]-velBA[1]) > eps {
		t.Errorf("escape velocity depends on obstacle order: [A,B]=%v vs [B,A]=%v", velAB, velBA)
	}
}
