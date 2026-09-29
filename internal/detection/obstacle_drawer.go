//go:build gocv

package detection

import (
	"image"
	"image/color"
)

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

// DrawObstaclesOn outlines each obstacle's pixel box onto dst, clipped to the
// image. Boxes wholly outside the image are skipped.
func (d *ObstacleDrawer) DrawObstaclesOn(dst *image.RGBA, obstacles []Obstacle) {
	width, height := dst.Rect.Dx(), dst.Rect.Dy()
	borderColor := color.RGBA{255, 107, 107, 255}

	for _, obs := range obstacles {
		x1 := obs.PixelTopLeft[0]
		y1 := obs.PixelTopLeft[1]
		x2 := obs.PixelBottomRight[0]
		y2 := obs.PixelBottomRight[1]

		if x1 >= width || y1 >= height || x2 <= 0 || y2 <= 0 {
			continue
		}

		clipX1 := max(0, min(x1, width))
		clipY1 := max(0, min(y1, height))
		clipX2 := max(0, min(x2, width))
		clipY2 := max(0, min(y2, height))

		lineWidth := 2
		drawLine(dst, image.Point{X: clipX1, Y: clipY1}, image.Point{X: clipX2, Y: clipY1}, borderColor, lineWidth)
		drawLine(dst, image.Point{X: clipX2, Y: clipY1}, image.Point{X: clipX2, Y: clipY2}, borderColor, lineWidth)
		drawLine(dst, image.Point{X: clipX2, Y: clipY2}, image.Point{X: clipX1, Y: clipY2}, borderColor, lineWidth)
		drawLine(dst, image.Point{X: clipX1, Y: clipY2}, image.Point{X: clipX1, Y: clipY1}, borderColor, lineWidth)
	}
}
