package main

import (
	"robot_tracker_go/internal/detection"
	"robot_tracker_go/internal/planning"
)

func convertDemoObstaclesToDetection(demoObstacles []DemoObstacle) []detection.Obstacle {
	result := make([]detection.Obstacle, len(demoObstacles))
	for i, obs := range demoObstacles {
		result[i] = detection.Obstacle{
			ID:               obs.name,
			PixelTopLeft:     [2]int{obs.x - obs.width/2, obs.y - obs.height/2},
			PixelBottomRight: [2]int{obs.x + obs.width/2, obs.y + obs.height/2},
			Clearance:        0.05,
		}
	}
	return result
}

func convertPlanningObstaclesToDetection(planningObstacles []planning.Obstacle) []detection.Obstacle {
	result := make([]detection.Obstacle, len(planningObstacles))
	for i, obs := range planningObstacles {
		result[i] = detection.Obstacle{
			ID:               obs.Name,
			PixelTopLeft:     obs.PixelsTopLeft,
			PixelBottomRight: obs.PixelsBottomRight,
			WorldTopLeft:     obs.WorldTopLeft,
			WorldBottomRight: obs.WorldBottomRight,
			Clearance:        0.05,
		}
	}
	return result
}
