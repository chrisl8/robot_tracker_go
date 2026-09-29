//go:build gocv

package main

import (
	"math"

	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// computeTrackHeading computes and smooths the heading for a confirmed track
// using AprilTag corner geometry, EMA smoothing, and outlier rejection.
func (rs *RobotSystem) computeTrackHeading(track *tracking.Track, tags []detection.AprilTag) {
	tagID := *track.TagID

	// Find matching tag and compute raw heading from corners
	tagFound := false
	var wBot, wTop *position.Point2D
	for _, tag := range tags {
		if tag.TagID != tagID {
			continue
		}
		tagFound = true
		track.Corners = tag.Corners

		// Use bottom-center → top-center to get the tag's canonical forward direction
		botMidX := (tag.Corners[2][0] + tag.Corners[3][0]) / 2
		botMidY := (tag.Corners[2][1] + tag.Corners[3][1]) / 2
		topMidX := (tag.Corners[0][0] + tag.Corners[1][0]) / 2
		topMidY := (tag.Corners[0][1] + tag.Corners[1][1]) / 2
		wBot = rs.position.positionEst.PixelToWorldFloat(botMidX, botMidY)
		wTop = rs.position.positionEst.PixelToWorldFloat(topMidX, topMidY)
		track.Heading = math.Atan2(wTop.Y-wBot.Y, wTop.X-wBot.X)

		// Apply configurable mounting offset
		if robotConfig := rs.cfg.GetRobotByTagID(tagID); robotConfig != nil {
			offset := robotConfig.HeadingOffsetDegrees * math.Pi / 180
			track.Heading += offset
			track.HeadingOffset = offset
		}

		rs.applyHeadingSmoothing(track, tagID)
		break
	}

	if !tagFound {
		utils.Debugf("HEADING: tag %d not detected this frame", tagID)
	}

	// Track heading delta (angular velocity) and cache heading
	if !tagFound {
		if cached, ok := rs.heading.lastHeading[tagID]; ok {
			track.Heading = cached
			utils.Debugf("HEADING: tag %d using cached heading=%.2f°", tagID, cached*180/math.Pi)
		}
		rs.heading.headingDelta[tagID] = 0
		rs.heading.headingLostCount[tagID]++
		if rs.heading.headingLostCount[tagID] > 5 {
			delete(rs.heading.smoothedHeading, tagID)
			delete(rs.heading.headingRejectCount, tagID)
		}
	} else {
		rs.heading.headingLostCount[tagID] = 0
		if prev, ok := rs.heading.lastHeading[tagID]; ok {
			delta := track.Heading - prev
			delta = normalizeAngle(delta)
			if math.Abs(delta) > 15*math.Pi/180 {
				utils.Debugf("HEADING JUMP: tag %d delta=%.1f° wBot=(%.3f,%.3f) wTop=(%.3f,%.3f)",
					tagID, delta*180/math.Pi, wBot.X, wBot.Y, wTop.X, wTop.Y)
				utils.Debugf("  corners: TL=(%.0f,%.0f) TR=(%.0f,%.0f) BR=(%.0f,%.0f) BL=(%.0f,%.0f)",
					track.Corners[0][0], track.Corners[0][1],
					track.Corners[1][0], track.Corners[1][1],
					track.Corners[2][0], track.Corners[2][1],
					track.Corners[3][0], track.Corners[3][1])
			}
			rs.heading.headingDelta[tagID] = delta
		} else {
			rs.heading.headingDelta[tagID] = 0
		}
		rs.heading.lastHeading[tagID] = track.Heading
	}
}

// normalizeAngle wraps an angle to the range [-pi, pi].
func normalizeAngle(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

// applyHeadingSmoothing applies angle-aware EMA smoothing with outlier rejection.
func (rs *RobotSystem) applyHeadingSmoothing(track *tracking.Track, tagID int) {
	prev, ok := rs.heading.smoothedHeading[tagID]
	if !ok {
		rs.heading.smoothedHeading[tagID] = track.Heading
		return
	}

	diff := normalizeAngle(track.Heading - prev)

	alpha := rs.cfg.Position.HeadingSmoothingAlpha
	if alpha <= 0 {
		alpha = 1.0
	}

	maxRate := rs.cfg.Position.HeadingMaxRateDeg * math.Pi / 180
	if maxRate > 0 && math.Abs(diff) > maxRate {
		// Measurement too far from smoothed — likely noise, reject it
		rs.heading.headingRejectCount[tagID]++
		utils.Debugf("HEADING REJECT: tag %d raw=%.1f° smoothed=%.1f° diff=%.1f° count=%d",
			tagID, track.Heading*180/math.Pi, prev*180/math.Pi, diff*180/math.Pi, rs.heading.headingRejectCount[tagID])
		if rs.heading.headingRejectCount[tagID] >= 10 {
			// Too many consecutive rejections — accept with EMA to converge
			track.Heading = normalizeAngle(prev + alpha*diff)
			utils.Debugf("HEADING RESET: tag %d after 10 rejections, converging to %.1f°",
				tagID, track.Heading*180/math.Pi)
			rs.heading.headingRejectCount[tagID] = 0
		} else {
			track.Heading = prev // keep previous smoothed heading
		}
	} else {
		// Reasonable change — apply EMA
		track.Heading = normalizeAngle(prev + alpha*diff)
		rs.heading.headingRejectCount[tagID] = 0
	}
	rs.heading.smoothedHeading[tagID] = track.Heading
}
