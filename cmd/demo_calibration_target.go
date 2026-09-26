package main

import (
	"math"

	"github.com/chrisl8/robot_tracker_go/internal/position"
)

// demoTargetPerspective maps world metres (Center-tag origin, +y down) to
// pixels for a 1280x720 frame with visible perspective, so demo mode can show
// the calibration wizard a realistic scattering of tags.
var demoTargetPerspective = [3][3]float64{
	{600, 30, 640},
	{20, 550, 360},
	{0.0002, 0.0005, 1},
}

// demoTargetLayout places each target tag on the floor near its guide box but
// deliberately off-square and rotated, as if dropped by hand; the calibration
// solver must not depend on precise placement.
var demoTargetLayout = map[int]struct{ x, y, deg float64 }{
	100: {0, 0, 0},
	101: {-0.63, -0.37, 12},
	102: {0.58, -0.33, -25},
	103: {0.62, 0.36, 40},
	104: {-0.57, 0.33, -8},
}

// demoTargetCaptures returns the calibration target tags as a camera would
// see them, scaled to the frame size.
func demoTargetCaptures(frameWidth, frameHeight int) []position.TargetCapture {
	sx := float64(frameWidth) / 1280
	sy := float64(frameHeight) / 720

	h := position.TargetTagSize / 2
	local := [4][2]float64{{-h, -h}, {h, -h}, {h, h}, {-h, h}}

	tags := position.TargetTags()
	captures := make([]position.TargetCapture, 0, len(tags))
	for _, tag := range tags {
		place := demoTargetLayout[tag.ID]
		s, c := math.Sincos(place.deg * math.Pi / 180)
		capture := position.TargetCapture{ID: tag.ID}
		for i, l := range local {
			wx := c*l[0] - s*l[1] + place.x
			wy := s*l[0] + c*l[1] + place.y
			m := demoTargetPerspective
			x := m[0][0]*wx + m[0][1]*wy + m[0][2]
			y := m[1][0]*wx + m[1][1]*wy + m[1][2]
			d := m[2][0]*wx + m[2][1]*wy + m[2][2]
			capture.Corners[i] = position.Point2D{X: x / d * sx, Y: y / d * sy}
		}
		captures = append(captures, capture)
	}
	return captures
}
