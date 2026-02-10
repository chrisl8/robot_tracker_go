package detection

import (
	"robot_tracker_go/internal/planning"
	"robot_tracker_go/internal/position"
)

func YOLODetectionsToDynamicObstacles(
	detections []YOLODetection,
	positionEst *position.PositionEstimator,
	relevantClasses map[string]bool,
	minConfidence float64,
) []*planning.DynamicObstacle {
	if len(detections) == 0 {
		return []*planning.DynamicObstacle{}
	}

	obstacles := make([]*planning.DynamicObstacle, 0, len(detections))

	for _, det := range detections {
		if det.Bbox == nil {
			continue
		}

		if det.Confidence < minConfidence {
			continue
		}

		if relevantClasses != nil {
			if _, ok := relevantClasses[det.ClassName]; !ok {
				continue
			}
		}

		cx, cy := det.Bbox.Center()
		bboxWidth := det.Bbox.Width()
		bboxHeight := det.Bbox.Height()

		var worldX, worldY float64
		if positionEst != nil && positionEst.IsCalibrated() {
			worldPos := positionEst.PixelToWorld(cx, cy)
			worldX = worldPos.X
			worldY = worldPos.Y
		} else {
			worldX = float64(cx)
			worldY = float64(cy)
		}

		maxDim := float64(max(bboxWidth, bboxHeight))
		radius := maxDim / 2.0

		obstacle := planning.NewDynamicObstacle(
			worldX,
			worldY,
			0.0,
			0.0,
			radius,
			det.ClassName,
			det.Confidence,
			false,
		)

		obstacles = append(obstacles, obstacle)
	}

	return obstacles
}
