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

	avoidanceVel := desiredVel

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
			avoidanceVel = p.computeCollisionAvoidance(robot, desiredVel, obs, combinedRadius, timeHorizon)
			continue
		}

		relVx := obs.Velocity.VX - robot.Velocity.VX
		relVy := obs.Velocity.VY - robot.Velocity.VY

		tau := dist / math.Sqrt(relVx*relVx+relVy*relVy)
		if tau > timeHorizon {
			continue
		}

		vo := p.computeVO(robot.Position, combinedRadius, obs.Position, [2]float64{obs.Velocity.VX, obs.Velocity.VY}, timeHorizon)

		if p.velocityInVO(desiredVel, vo) {
			avoidanceVel = p.computeBestAvoidanceVelocity(robot, desiredVel, obstacles, vo)
		}
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

func (p *LocalPlanner) computeBestAvoidanceVelocity(robot RobotState, desiredVel [2]float64, obstacles []RobotState, vo [][2]float64) [2]float64 {
	bestVel := desiredVel
	bestScore := -1.0

	candidateVels := p.generateCandidateVelocities(desiredVel)

	for _, cand := range candidateVels {
		if !p.velocityInVO(cand, vo) {
			score := p.evaluateVelocity(cand, desiredVel, obstacles)
			if score > bestScore {
				bestScore = score
				bestVel = cand
			}
		}
	}

	if bestScore < 0 {
		bestVel = [2]float64{0, 0}
	}

	return bestVel
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

func (p *LocalPlanner) computeCollisionAvoidance(robot RobotState, desiredVel [2]float64, obs RobotState, combinedRadius, timeHorizon float64) [2]float64 {
	dx := obs.Position[0] - robot.Position[0]
	dy := obs.Position[1] - robot.Position[1]
	dist := math.Sqrt(dx*dx + dy*dy)

	if dist < 0.01 {
		return [2]float64{0, 0}
	}

	avoidX := -dx / dist * p.config.MaxVelocity * 0.8
	avoidY := -dy / dist * p.config.MaxVelocity * 0.8

	return [2]float64{avoidX, avoidY}
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
