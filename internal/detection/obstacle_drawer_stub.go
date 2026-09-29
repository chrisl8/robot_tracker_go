//go:build !gocv

package detection

import "image"

type Obstacle struct {
	ID               string
	PixelTopLeft     [2]int
	PixelBottomRight [2]int
	WorldTopLeft     [2]float64
	WorldBottomRight [2]float64
	Clearance        float64
}

type ObstacleDrawer struct{}

func NewObstacleDrawer() *ObstacleDrawer {
	return &ObstacleDrawer{}
}

func (d *ObstacleDrawer) DrawObstaclesOn(dst *image.RGBA, obstacles []Obstacle) {}
