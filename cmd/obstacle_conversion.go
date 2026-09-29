package main

import (
	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
)

// obstacleClearance is the extra clearance (metres) the detection pipeline
// keeps around each user-drawn obstacle.
const obstacleClearance = 0.05

// convertPlanningObstaclesToDetection converts the operator-drawn obstacles the
// planner uses into the form the detection pipeline masks out.
func convertPlanningObstaclesToDetection(planningObstacles []planning.Obstacle) []detection.Obstacle {
	result := make([]detection.Obstacle, len(planningObstacles))
	for i, obs := range planningObstacles {
		result[i] = detection.Obstacle{
			ID:               obs.Name,
			PixelTopLeft:     obs.PixelsTopLeft,
			PixelBottomRight: obs.PixelsBottomRight,
			WorldTopLeft:     obs.WorldTopLeft,
			WorldBottomRight: obs.WorldBottomRight,
			Clearance:        obstacleClearance,
		}
	}
	return result
}
