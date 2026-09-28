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

// TestLocalPlanner_CollisionAvoidance_BlendsGoalDirection verifies that the
// escape velocity slides sideways toward the goal instead of retreating
// straight back the way the robot came with no regard for desiredVel.
// Regression test for the oscillation bug (code review finding #10): the old
// implementation ignored desiredVel entirely and always retreated directly
// away from the obstacle at a fixed 80% max speed, so the robot would bounce
// back into the same obstacle the instant it cleared the safety margin.
func TestLocalPlanner_CollisionAvoidance_BlendsGoalDirection(t *testing.T) {
	lp := NewLocalPlanner(&VelocityObstacleConfig{
		TimeHorizon:     2.0,
		SafetyMargin:    0.15,
		MaxVelocity:     0.5,
		MaxAcceleration: 0.3,
		TimeStep:        0.1,
	})

	robot := RobotState{Position: [2]float64{0, 0}, Diameter: 0.3}

	// Obstacle directly ahead on +X; goal is beyond it, offset in +Y, so the
	// desired velocity has a sideways component the robot should be able to
	// use to go around the obstacle rather than just backing away from it.
	obs := RobotState{Position: [2]float64{0.2, 0}, Diameter: 0.3} // shallow penetration
	desiredVel := [2]float64{0.3, 0.3}

	vel := lp.applyVelocityObstacles(robot, desiredVel, []RobotState{obs})

	if vel[0] >= 0 {
		t.Errorf("expected escape velocity to retreat in -X away from the obstacle, got vel=%v", vel)
	}
	if vel[1] <= 0 {
		t.Errorf("expected escape velocity to retain a +Y component toward the goal (sliding around the obstacle), got vel=%v", vel)
	}

	// A deep penetration should lean further on pure retreat (smaller
	// sideways component relative to escape speed) than a shallow one.
	deepObs := RobotState{Position: [2]float64{0.05, 0}, Diameter: 0.3}
	velDeep := lp.applyVelocityObstacles(robot, desiredVel, []RobotState{deepObs})

	shallowRatio := math.Abs(vel[1] / vel[0])
	deepRatio := math.Abs(velDeep[1] / velDeep[0])
	if deepRatio >= shallowRatio {
		t.Errorf("expected deeper penetration to reduce the sideways-to-retreat ratio: shallow=%v (ratio %v) deep=%v (ratio %v)", vel, shallowRatio, velDeep, deepRatio)
	}
}
