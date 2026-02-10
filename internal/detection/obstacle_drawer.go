//go:build gocv

package detection

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
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

func (d *ObstacleDrawer) DrawObstacles(imgData []byte, width, height int, obstacles []Obstacle) []byte {
	if len(imgData) == 0 || len(obstacles) == 0 {
		// TODO: Show this log message in verbose mode.
		// log.Printf("OBSTACLE_DRAWER: Early exit - empty data or obstacles")
		return imgData
	}

	reader := bytes.NewReader(imgData)
	img, _, err := image.Decode(reader)
	if err != nil {
		// TODO: Show this log message in verbose mode.
		// log.Printf("OBSTACLE_DRAWER: Failed to decode image: %v", err)
		return imgData
	}

	rgba, ok := img.(*image.RGBA)
	if !ok {
		b := img.Bounds()
		newImg := image.NewRGBA(b)
		draw.Draw(newImg, b, img, b.Min, draw.Src)
		rgba = newImg
	}

	borderColor := color.RGBA{255, 107, 107, 255}

	drawnCount := 0
	for _, obs := range obstacles {
		x1 := obs.PixelTopLeft[0]
		y1 := obs.PixelTopLeft[1]
		x2 := obs.PixelBottomRight[0]
		y2 := obs.PixelBottomRight[1]

		if x1 >= width || y1 >= height || x2 <= 0 || y2 <= 0 {
			// TODO: Show this log message in verbose mode.
			// log.Printf("OBSTACLE_DRAWER: Skipping '%s' - out of bounds", obs.ID)
			continue
		}

		clipX1 := max(0, min(x1, width))
		clipY1 := max(0, min(y1, height))
		clipX2 := max(0, min(x2, width))
		clipY2 := max(0, min(y2, height))

		lineWidth := 2
		drawLine(rgba, image.Point{X: clipX1, Y: clipY1}, image.Point{X: clipX2, Y: clipY1}, borderColor, lineWidth)
		drawLine(rgba, image.Point{X: clipX2, Y: clipY1}, image.Point{X: clipX2, Y: clipY2}, borderColor, lineWidth)
		drawLine(rgba, image.Point{X: clipX2, Y: clipY2}, image.Point{X: clipX1, Y: clipY2}, borderColor, lineWidth)
		drawLine(rgba, image.Point{X: clipX1, Y: clipY2}, image.Point{X: clipX1, Y: clipY1}, borderColor, lineWidth)

		drawnCount++
	}

	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, rgba, &jpeg.Options{Quality: 85}); err != nil {
		// TODO: Show this log message in verbose mode.
		// log.Printf("OBSTACLE_DRAWER: Failed to encode output: %v", err)
		return imgData
	}

	return buf.Bytes()
}
