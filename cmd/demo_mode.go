//go:build gocv

package main

import (
	"github.com/chrisl8/robot_tracker_go/internal/ui"
)

func demoCalibrationTagInfos(width, height int) []ui.DetectedTagInfo {
	captures := demoTargetCaptures(width, height)
	infos := make([]ui.DetectedTagInfo, 0, len(captures))
	for _, c := range captures {
		var info ui.DetectedTagInfo
		info.ID = c.ID
		for i, p := range c.Corners {
			info.Corners[i] = [2]float64{p.X, p.Y}
			info.Center[0] += p.X / 4
			info.Center[1] += p.Y / 4
		}
		infos = append(infos, info)
	}
	return infos
}
