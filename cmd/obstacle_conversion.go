package main

import (
	"math"

	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/position"
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

// nonRobotYOLODetections returns the YOLO detections that the fusion step did
// NOT match to a robot's AprilTag (DetectionTypeYOLO in FusedDetections means
// "no matching tag"). Feeding raw, unfiltered YOLO detections into obstacle
// avoidance lets a robot whose own body gets classified as one of the
// obstacle classes (e.g. "chair") be treated as its own obstacle, permanently
// blocking it in place.
func nonRobotYOLODetections(result *detection.DetectionResult) []detection.YOLODetection {
	nonRobot := make([]detection.YOLODetection, 0, len(result.FusedDetections))
	for _, fd := range result.FusedDetections {
		if fd.DetectionType == detection.DetectionTypeYOLO {
			nonRobot = append(nonRobot, detection.YOLODetection{
				Bbox:       fd.Bbox,
				Confidence: fd.Confidence,
				ClassName:  fd.ClassName,
			})
		}
	}
	return nonRobot
}

// excludeYOLONearKnownRobots drops YOLO detections whose bbox center lands on
// top of a currently-tracked robot's last known position. nonRobotYOLODetections
// only excludes a detection matched to a robot's AprilTag in that exact frame;
// when the tag briefly fails to detect (common — occlusion, angle, motion
// blur), the robot's own misclassified body (e.g. YOLO calling it a "chair")
// slips back through as a phantom obstacle sitting on the robot itself,
// permanently reporting negative clearance and blocking all movement.
func excludeYOLONearKnownRobots(
	detections []detection.YOLODetection,
	positionEst *position.PositionEstimator,
	robots map[int]planning.RobotState,
) []detection.YOLODetection {
	if positionEst == nil || !positionEst.IsCalibrated() || len(robots) == 0 {
		return detections
	}

	const clearanceMargin = 0.10 // meters, beyond the robot's own radius

	filtered := make([]detection.YOLODetection, 0, len(detections))
	for _, det := range detections {
		if det.Bbox == nil {
			filtered = append(filtered, det)
			continue
		}
		cx, cy := det.Bbox.Center()
		world := positionEst.PixelToWorld(cx, cy)

		nearRobot := false
		for _, robot := range robots {
			dx := world.X - robot.Position[0]
			dy := world.Y - robot.Position[1]
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < robot.Diameter/2+clearanceMargin {
				nearRobot = true
				break
			}
		}
		if !nearRobot {
			filtered = append(filtered, det)
		}
	}
	return filtered
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
