package planning

import "math"

type VelocityObstacleConfig struct {
	TimeHorizon     float64
	SafetyMargin    float64
	MaxVelocity     float64
	MaxAcceleration float64
	TimeStep        float64
}

type LocalPlanner struct {
	config *VelocityObstacleConfig
}

func NewLocalPlanner(config *VelocityObstacleConfig) *LocalPlanner {
	if config == nil {
		config = &VelocityObstacleConfig{
			TimeHorizon:     2.0,
			SafetyMargin:    0.15,
			MaxVelocity:     0.5,
			MaxAcceleration: 0.3,
			TimeStep:        0.1,
		}
	}
	return &LocalPlanner{config: config}
}

func (p *LocalPlanner) ComputeVelocity(robot RobotState, goal [2]float64, obstacles []RobotState) ([2]float64, bool) {
	desiredVel := p.computeDesiredVelocity(robot, goal)
	safeVel := p.applyVelocityObstacles(robot, desiredVel, obstacles)
	return safeVel, true
}

func (p *LocalPlanner) computeDesiredVelocity(robot RobotState, goal [2]float64) [2]float64 {
	dx := goal[0] - robot.Position[0]
	dy := goal[1] - robot.Position[1]
	dist := math.Sqrt(dx*dx + dy*dy)

	if dist < 0.01 {
		return [2]float64{0, 0}
	}

	maxSpeed := robot.Diameter
	if maxSpeed > p.config.MaxVelocity {
		maxSpeed = p.config.MaxVelocity
	}

	desiredSpeed := maxSpeed
	if dist < 0.5 {
		desiredSpeed = maxSpeed * dist / 0.5
	}

	velX := (dx / dist) * desiredSpeed
	velY := (dy / dist) * desiredSpeed

	return [2]float64{velX, velY}
}

func (p *LocalPlanner) applyVelocityObstacles(robot RobotState, desiredVel [2]float64, obstacles []RobotState) [2]float64 {
	robotRadius := robot.Diameter / 2
	timeHorizon := p.config.TimeHorizon
	safetyMargin := p.config.SafetyMargin

	// Collect every obstacle we're currently penetrating and every velocity
	// obstacle (VO) triggered by a predicted future collision. We must
	// account for all of them together — reacting to only the
	// last-processed obstacle can leave the chosen velocity still inside an
	// earlier obstacle's danger zone.
	var penetrating []RobotState
	var vos [][][2]float64
	needsVOAvoidance := false

	for _, obs := range obstacles {
		if obs.Diameter <= 0 {
			continue
		}

		obsRadius := obs.Diameter / 2
		combinedRadius := robotRadius + obsRadius + safetyMargin

		dx := obs.Position[0] - robot.Position[0]
		dy := obs.Position[1] - robot.Position[1]
		dist := math.Sqrt(dx*dx + dy*dy)

		if dist < combinedRadius {
			penetrating = append(penetrating, obs)
			continue
		}

		relVx := obs.Velocity.VX - robot.Velocity.VX
		relVy := obs.Velocity.VY - robot.Velocity.VY

		tau := dist / math.Sqrt(relVx*relVx+relVy*relVy)
		if tau > timeHorizon {
			continue
		}

		vo := p.computeVO(robot.Position, combinedRadius, obs.Position, [2]float64{obs.Velocity.VX, obs.Velocity.VY}, timeHorizon)
		vos = append(vos, vo)

		if p.velocityInVO(desiredVel, vo) {
			needsVOAvoidance = true
		}
	}

	avoidanceVel := desiredVel

	if len(penetrating) > 0 {
		// Already inside one or more safety margins: escape combines every
		// penetrating obstacle's push-away direction, weighted by how deep
		// the penetration is, rather than only the last one seen.
		avoidanceVel = p.computeCombinedCollisionAvoidance(robot, penetrating, robotRadius, safetyMargin)
	} else if needsVOAvoidance {
		// Not yet penetrating anything, but the desired velocity would enter
		// a predicted collision cone. Pick a candidate velocity that clears
		// every obstacle's VO simultaneously, not just the one that
		// triggered avoidance.
		avoidanceVel = p.computeBestAvoidanceVelocity(robot, desiredVel, obstacles, vos)
	}

	velMag := math.Sqrt(avoidanceVel[0]*avoidanceVel[0] + avoidanceVel[1]*avoidanceVel[1])
	if velMag > p.config.MaxVelocity {
		avoidanceVel[0] = (avoidanceVel[0] / velMag) * p.config.MaxVelocity
		avoidanceVel[1] = (avoidanceVel[1] / velMag) * p.config.MaxVelocity
	}

	return avoidanceVel
}

func (p *LocalPlanner) computeVO(robotPos [2]float64, radius float64, obsPos, obsVel [2]float64, tau float64) [][2]float64 {
	apex := robotPos

	discCenterX := obsPos[0] + obsVel[0]*tau - robotPos[0]
	discCenterY := obsPos[1] + obsVel[1]*tau - robotPos[1]

	discRadius := radius * tau

	alpha := math.Asin(discRadius / (math.Sqrt(discCenterX*discCenterX+discCenterY*discCenterY) + 0.001))

	theta := math.Atan2(discCenterY, discCenterX)

	halfAngle := alpha

	ray1Angle := theta + halfAngle
	ray2Angle := theta - halfAngle

	ray1 := [2]float64{math.Cos(ray1Angle), math.Sin(ray1Angle)}
	ray2 := [2]float64{math.Cos(ray2Angle), math.Sin(ray2Angle)}

	return [][2]float64{apex, ray1, ray2}
}

func (p *LocalPlanner) velocityInVO(vel [2]float64, vo [][2]float64) bool {
	apex := vo[0]
	ray1 := vo[1]
	ray2 := vo[2]

	dx := vel[0] - apex[0]
	dy := vel[1] - apex[1]

	cross1 := dx*ray1[1] - dy*ray1[0]
	cross2 := dx*ray2[1] - dy*ray2[0]

	return cross1 >= 0 && cross2 <= 0
}

func (p *LocalPlanner) computeBestAvoidanceVelocity(robot RobotState, desiredVel [2]float64, obstacles []RobotState, vos [][][2]float64) [2]float64 {
	bestVel := [2]float64{0, 0}
	bestScore := -1.0
	found := false

	candidateVels := p.generateCandidateVelocities(desiredVel)

	for _, cand := range candidateVels {
		if p.velocityInAnyVO(cand, vos) {
			continue
		}
		score := p.evaluateVelocity(cand, desiredVel, obstacles)
		if score > bestScore {
			bestScore = score
			bestVel = cand
			found = true
		}
	}

	if !found {
		bestVel = [2]float64{0, 0}
	}

	return bestVel
}

// velocityInAnyVO reports whether vel falls inside any of the given velocity
// obstacles.
func (p *LocalPlanner) velocityInAnyVO(vel [2]float64, vos [][][2]float64) bool {
	for _, vo := range vos {
		if p.velocityInVO(vel, vo) {
			return true
		}
	}
	return false
}

func (p *LocalPlanner) generateCandidateVelocities(desired [2]float64) [][2]float64 {
	speeds := []float64{0.1, 0.2, 0.3}
	angles := [8]float64{0, math.Pi / 4, math.Pi / 2, 3 * math.Pi / 4, math.Pi, -math.Pi / 4, -math.Pi / 2, -3 * math.Pi / 4}
	candidates := make([][2]float64, 0, len(speeds)*len(angles)+1)

	for _, angle := range angles {
		for _, speed := range speeds {
			candidates = append(candidates, [2]float64{
				math.Cos(angle) * speed,
				math.Sin(angle) * speed,
			})
		}
	}

	candidates = append(candidates, [2]float64{0, 0})

	return candidates
}

func (p *LocalPlanner) evaluateVelocity(vel [2]float64, desired [2]float64, obstacles []RobotState) float64 {
	desiredMag := math.Sqrt(desired[0]*desired[0] + desired[1]*desired[1])
	if desiredMag < 0.001 {
		return 0
	}

	alignment := (vel[0]*desired[0] + vel[1]*desired[1]) / (desiredMag*math.Sqrt(vel[0]*vel[0]+vel[1]*vel[1]) + 0.001)

	speed := math.Sqrt(vel[0]*vel[0] + vel[1]*vel[1])

	return alignment*0.7 + (speed/p.config.MaxVelocity)*0.3
}

// computeCombinedCollisionAvoidance produces a single escape velocity that
// accounts for every obstacle whose safety margin the robot is currently
// inside, weighted by how deep each penetration is. This avoids the bug
// where handling obstacles one at a time in a loop lets the last obstacle
// processed silently override an escape direction that was safe for an
// earlier, still-penetrated obstacle.
func (p *LocalPlanner) computeCombinedCollisionAvoidance(robot RobotState, obstacles []RobotState, robotRadius, safetyMargin float64) [2]float64 {
	var sumX, sumY float64

	for _, obs := range obstacles {
		obsRadius := obs.Diameter / 2
		combinedRadius := robotRadius + obsRadius + safetyMargin

		dx := obs.Position[0] - robot.Position[0]
		dy := obs.Position[1] - robot.Position[1]
		dist := math.Sqrt(dx*dx + dy*dy)

		if dist < 0.01 {
			// Coincident positions: push away in an arbitrary but
			// consistent direction rather than dividing by ~zero.
			dist = 0.01
			dx, dy = 0.01, 0
		}

		penetration := combinedRadius - dist
		if penetration < 0 {
			penetration = 0
		}

		// Weight the escape contribution by penetration depth (plus a
		// small floor so even a barely-penetrating obstacle still counts)
		// so the deepest intrusion dominates the combined direction.
		weight := penetration + 0.01
		sumX += (-dx / dist) * weight
		sumY += (-dy / dist) * weight
	}

	mag := math.Sqrt(sumX*sumX + sumY*sumY)
	if mag < 1e-9 {
		return [2]float64{0, 0}
	}

	return [2]float64{
		(sumX / mag) * p.config.MaxVelocity * 0.8,
		(sumY / mag) * p.config.MaxVelocity * 0.8,
	}
}

func (p *LocalPlanner) IsCollisionFree(robot RobotState, velocity [2]float64, obstacles []RobotState, duration float64) bool {
	newX := robot.Position[0] + velocity[0]*duration
	newY := robot.Position[1] + velocity[1]*duration

	robotRadius := robot.Diameter / 2

	for _, obs := range obstacles {
		obsRadius := obs.Diameter / 2
		combinedRadius := robotRadius + obsRadius + p.config.SafetyMargin

		obsX := obs.Position[0] + obs.Velocity.VX*duration
		obsY := obs.Position[1] + obs.Velocity.VY*duration

		dx := newX - obsX
		dy := newY - obsY
		dist := math.Sqrt(dx*dx + dy*dy)

		if dist < combinedRadius {
			return false
		}
	}

	return true
}

func (p *LocalPlanner) ComputeVelocityWithObstacles(
	robot RobotState,
	goal [2]float64,
	robotObstacles []RobotState,
	dynamicObstacles []*DynamicObstacle,
	minConfidence float64,
) ([2]float64, bool) {
	desiredVel := p.computeDesiredVelocity(robot, goal)

	filteredObstacles := make([]RobotState, 0, len(robotObstacles))
	for _, obs := range robotObstacles {
		if obs.Diameter > 0 {
			filteredObstacles = append(filteredObstacles, obs)
		}
	}

	for _, dyn := range dynamicObstacles {
		if dyn.Confidence >= minConfidence && dyn.Radius > 0 {
			filteredObstacles = append(filteredObstacles, dyn.ToRobotState())
		}
	}

	safeVel := p.applyVelocityObstacles(robot, desiredVel, filteredObstacles)
	return safeVel, true
}

func (p *LocalPlanner) ComputeVelocityToWaypoint(
	robot RobotState,
	waypoint [2]float64,
	staticObstacles []Obstacle,
	dynamicObstacles []*DynamicObstacle,
	minConfidence float64,
) ([2]float64, bool) {
	dx := waypoint[0] - robot.Position[0]
	dy := waypoint[1] - robot.Position[1]
	dist := math.Sqrt(dx*dx + dy*dy)

	if dist < 0.05 {
		return [2]float64{0, 0}, true
	}

	maxVel := p.config.MaxVelocity
	vx := (dx / dist) * maxVel
	vy := (dy / dist) * maxVel

	desiredVel := [2]float64{vx, vy}

	filteredObstacles := make([]RobotState, 0)
	for _, obs := range staticObstacles {
		obsState := RobotState{
			Position: [2]float64{(obs.WorldTopLeft[0] + obs.WorldBottomRight[0]) / 2, (obs.WorldTopLeft[1] + obs.WorldBottomRight[1]) / 2},
			Diameter: obs.WorldBottomRight[0] - obs.WorldTopLeft[0],
		}
		filteredObstacles = append(filteredObstacles, obsState)
	}

	for _, dyn := range dynamicObstacles {
		if dyn.Confidence >= minConfidence && dyn.Radius > 0 {
			filteredObstacles = append(filteredObstacles, dyn.ToRobotState())
		}
	}

	safeVel := p.applyVelocityObstacles(robot, desiredVel, filteredObstacles)
	return safeVel, true
}
