package main

import (
	"github.com/chrisl8/robot_tracker_go/internal/position"
)

// demoTargetPerspective maps world metres (Center-tag origin, +y down) to
// pixels for a 1280x720 frame with visible perspective, so demo mode can show
// the calibration wizard a realistic laid-out target.
var demoTargetPerspective = [3][3]float64{
	{600, 30, 640},
	{20, 550, 360},
	{0.0002, 0.0005, 1},
}

// demoTargetCaptures returns the calibration target tags as a camera would
// see them, laid out at the default width x depth, scaled to the frame size.
func demoTargetCaptures(frameWidth, frameHeight int) []position.TargetCapture {
	sx := float64(frameWidth) / 1280
	sy := float64(frameHeight) / 720

	tags := position.TargetTags()
	captures := make([]position.TargetCapture, 0, len(tags))
	for _, tag := range tags {
		capture := position.TargetCapture{ID: tag.ID}
		for i, w := range tag.WorldCorners(position.DefaultTargetWidth, position.DefaultTargetDepth) {
			m := demoTargetPerspective
			x := m[0][0]*w.X + m[0][1]*w.Y + m[0][2]
			y := m[1][0]*w.X + m[1][1]*w.Y + m[1][2]
			d := m[2][0]*w.X + m[2][1]*w.Y + m[2][2]
			capture.Corners[i] = position.Point2D{X: x / d * sx, Y: y / d * sy}
		}
		captures = append(captures, capture)
	}
	return captures
}
