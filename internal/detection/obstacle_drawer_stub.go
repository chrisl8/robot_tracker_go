//go:build !gocv

package detection

import "image"

// Obstacle is a marked static obstacle as the drawer needs it.
type Obstacle struct {
	ID               string
	PixelTopLeft     [2]int
	PixelBottomRight [2]int
	WorldTopLeft     [2]float64
	WorldBottomRight [2]float64
	Clearance        float64
}

// ObstacleDrawer is unavailable without OpenCV; it draws nothing.
type ObstacleDrawer struct{}

// NewObstacleDrawer returns an ObstacleDrawer.
func NewObstacleDrawer() *ObstacleDrawer {
	return &ObstacleDrawer{}
}

// DrawObstaclesOn draws nothing.
func (d *ObstacleDrawer) DrawObstaclesOn(dst *image.RGBA, obstacles []Obstacle) {}
