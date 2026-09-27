package main

import (
	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
)

func convertDemoObstaclesToDetection(demoObstacles []DemoObstacle) []detection.Obstacle {
	result := make([]detection.Obstacle, len(demoObstacles))
	for i, obs := range demoObstacles {
		result[i] = detection.Obstacle{
			ID:               obs.Name,
			PixelTopLeft:     [2]int{obs.X - obs.Width/2, obs.Y - obs.Height/2},
			PixelBottomRight: [2]int{obs.X + obs.Width/2, obs.Y + obs.Height/2},
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
