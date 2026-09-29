//go:build gocv

package main

import (
	"image"
	"math"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/camera"
	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
	"github.com/chrisl8/robot_tracker_go/internal/ui"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

func (rs *RobotSystem) convertFusedToTrackingDetections(fused []detection.FusedDetection) []tracking.Detection {
	detections := make([]tracking.Detection, 0, len(fused))
	for _, f := range fused {
		bbox := f.Bbox
		det := tracking.Detection{
			Bbox:       [4]int{bbox.X1, bbox.Y1, bbox.X2, bbox.Y2},
			Confidence: f.Confidence,
			Corners:    f.Corners,
		}
		if f.TagID != nil {
			det.TagID = f.TagID
		}
		detections = append(detections, det)
	}
	return detections
}

// updateFPS advances the smoothed (EMA) FPS estimate in rs.stats using the
// gap since the previous frame's frameStart. Shared by ProcessFrame and
// ProcessDemoFrame so real and demo frame timing use identical math.
func (rs *RobotSystem) updateFPS(frameStart time.Time) {
	if !rs.stats.lastFrameTime.IsZero() {
		if dt := frameStart.Sub(rs.stats.lastFrameTime).Seconds(); dt > 0 {
			instantFPS := 1.0 / dt
			const alpha = 0.1 // EMA smoothing factor
			if rs.stats.smoothedFPS == 0 {
				rs.stats.smoothedFPS = instantFPS
			} else {
				rs.stats.smoothedFPS = alpha*instantFPS + (1-alpha)*rs.stats.smoothedFPS
			}
		}
	}
	rs.stats.lastFrameTime = frameStart
}

// updateTrackWorldPosition projects a confirmed track's pixel-space bbox
// center into world coordinates, updates the position estimator and the
// track's PixelRadius (by projecting the robot's configured world-space
// footprint back through the homography), and returns the world position
// plus the robot's configured diameter (falling back to 0.30m if the tag
// isn't in config). Shared by ProcessFrame and ProcessDemoFrame so both use
// identical per-track math.
//
// applyCenterOffset controls whether a configured CenterOffsetX/Y (which
// compensates for the AprilTag not being mounted at the robot's true
// rotational center) is applied to worldPos. It requires a smoothed heading
// to already be available in rs.heading.lastHeading, so it must stay false
// for a track this function computes heading for before computeTrackHeading
// runs.
func (rs *RobotSystem) updateTrackWorldPosition(track *tracking.Track, applyCenterOffset bool) (worldPos *position.Point2D, robotDiameter float64) {
	robotDiameter = 0.30 // fallback
	px, py := (track.Bbox[0]+track.Bbox[2])/2, (track.Bbox[1]+track.Bbox[3])/2
	worldPos = rs.position.positionEst.PixelToWorld(px, py)
	if robotConfig := rs.cfg.GetRobotByTagID(*track.TagID); robotConfig != nil {
		// Compute pixel radius by projecting world-space footprint through homography
		worldRadius := robotConfig.Diameter / 2
		edgePx, edgePy := rs.position.positionEst.WorldToPixel(position.Point2D{
			X: worldPos.X + worldRadius, Y: worldPos.Y,
		})
		dxR := float64(edgePx - px)
		dyR := float64(edgePy - py)
		track.PixelRadius = math.Sqrt(dxR*dxR + dyR*dyR)
		robotDiameter = robotConfig.Diameter
		// Apply center offset if configured (compensates for tag-to-robot-center distance / parallax)
		if applyCenterOffset && (robotConfig.CenterOffsetX != 0 || robotConfig.CenterOffsetY != 0) {
			if heading, ok := rs.heading.lastHeading[*track.TagID]; ok {
				cosH := math.Cos(heading)
				sinH := math.Sin(heading)
				worldPos.X += robotConfig.CenterOffsetX*cosH - robotConfig.CenterOffsetY*sinH
				worldPos.Y += robotConfig.CenterOffsetX*sinH + robotConfig.CenterOffsetY*cosH
			}
		}
	}
	track.WorldPos = [2]float64{worldPos.X, worldPos.Y}
	return worldPos, robotDiameter
}

// noteFrameSize tells the position estimator the size of the frames being
// processed. The calibration-resolution check and the web API's range checks
// (destinations, obstacles) depend on it. Shared by ProcessFrame and
// ProcessDemoFrame: the demo path used to skip it, which silently disabled the
// API's upper-bound checks in demo mode.
func (rs *RobotSystem) noteFrameSize(width, height int) {
	if rs.position.positionEst != nil {
		rs.position.positionEst.SetFrameSize(width, height)
	}
}

// broadcastFrameStats pushes per-frame tag/track counts and Arduino/FPS
// status to the web server, throttling the status/tag broadcast to roughly
// once a second via statusBroadcastDue(). Shared by ProcessFrame and
// ProcessDemoFrame; extraDetectedTags lets ProcessDemoFrame append its
// synthetic calibration-target tag markers.
func (rs *RobotSystem) broadcastFrameStats(tagCount, trackCount int, tags []detection.AprilTag, width, height int, extraDetectedTags []ui.DetectedTagInfo) {
	if rs.web.webServer == nil {
		return
	}
	rs.web.webServer.UpdateStats(tagCount)

	// Broadcast Arduino status via WebSocket every ~1 second (30 frames)
	if rs.statusBroadcastDue() {
		rs.web.webServer.SetArduinoConnected(rs.io.arduino != nil && rs.io.arduino.IsConnected())
		rs.web.webServer.BroadcastStatus(trackCount, rs.stats.smoothedFPS, time.Since(rs.stats.startTime).Seconds())
	}

	detectedTags := make([]ui.DetectedTagInfo, 0, len(tags)+len(extraDetectedTags))
	for _, tag := range tags {
		detectedTags = append(detectedTags, ui.DetectedTagInfo{
			ID:      tag.TagID,
			Center:  [2]float64{tag.CenterX, tag.CenterY},
			Corners: tag.Corners,
		})
	}
	detectedTags = append(detectedTags, extraDetectedTags...)
	rs.web.webServer.UpdateDetectedTags(detectedTags, width, height)
}

func (rs *RobotSystem) ProcessFrame(img image.Image, frameData []byte) {
	rs.capture.frameMu.Lock()
	defer rs.capture.frameMu.Unlock()
	if rs.capture.stopped {
		return
	}

	if img == nil {
		return
	}

	rs.stats.frameNum++
	frameStart := time.Now()
	rs.stats.lastFrameNanos.Store(frameStart.UnixNano())
	timestamp := float64(frameStart.UnixNano()) / 1e9

	rs.updateFPS(frameStart)

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	rs.noteFrameSize(width, height)

	detectionResult := rs.detection.detectionPipe.Detect(frameData, width, height, timestamp, rs.stats.frameNum)
	detectTime := time.Since(frameStart)

	rs.processForeground(frameData, width, height, frameStart, detectionResult)

	trackingDetections := rs.convertFusedToTrackingDetections(detectionResult.FusedDetections)
	trackingResult := rs.tracking.tracker.Update(trackingDetections, timestamp, rs.stats.frameNum)

	for i := range trackingResult.Tracks {
		track := &trackingResult.Tracks[i]
		if track.State == tracking.TrackStateConfirmed && track.TagID != nil {
			if rs.position.positionEst != nil {
				worldPos, robotDiameter := rs.updateTrackWorldPosition(track, true)
				rs.planning.planner.AddRobot(*track.TagID, [2]float64{worldPos.X, worldPos.Y}, robotDiameter)

				rs.computeTrackHeading(track, detectionResult.Tags)
				utils.Debugf("TRACKPOS: robot=%d pos=(%.3f,%.3f) heading=%.1f° tagSeen=%v cmd=%s",
					*track.TagID, worldPos.X, worldPos.Y, track.Heading*180/math.Pi,
					rs.heading.headingLostCount[*track.TagID] == 0, rs.io.robotCommands[*track.TagID])
			}
		}
	}

	rs.web.webServer.BroadcastTracks(trackingResult.Tracks, rs.cfg.Robots, rs.io.robotCommands)

	rs.executeAutonomousControl(trackingResult.Tracks)

	if rs.web.webServer.CalibrationViewActive() {
		// Calibration wizard open: send the clean camera view, no detection overlay.
		if img != nil {
			rs.web.webServer.PushFrame(img)
		}
	} else if overlay := rs.detection.detectionPipe.DrawResults(frameData, width, height, detectionResult); len(overlay) > 0 && len(overlay) < width*height*3 {
		rs.web.webServer.PushRawJPEG(overlay)
	} else if img != nil {
		rs.web.webServer.PushFrame(img)
	}

	rs.broadcastFrameStats(len(detectionResult.Tags), len(trackingResult.Tracks), detectionResult.Tags, width, height, nil)

	if rs.stats.frameNum%10 == 0 && rs.web.webServer != nil {
		rs.web.webServer.BroadcastPaths()
	}

	frameTotal := time.Since(frameStart)
	rs.stats.perf.Record(frameTotal, detectTime, frameTotal-detectTime)

	if rs.stats.frameNum%30 == 0 {
		hasConfirmedRobot := false
		for _, track := range trackingResult.Tracks {
			if track.State == tracking.TrackStateConfirmed && track.TagID != nil {
				hasConfirmedRobot = true
				break
			}
		}
		if hasConfirmedRobot {
			totalTime := time.Since(frameStart)
			trackPlanTime := totalTime - detectTime
			utils.Debugf("FRAME TIMING: detect=%dms track+plan=%dms total=%dms",
				detectTime.Milliseconds(), trackPlanTime.Milliseconds(), totalTime.Milliseconds())
		}
	}
}

func decodeToImage(data []byte, width, height int) image.Image {
	if len(data) == 0 {
		return nil
	}
	expectedLen := width * height * 3
	if len(data) != expectedLen {
		return nil
	}
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < width*height; i++ {
		b := data[i*3]
		g := data[i*3+1]
		r := data[i*3+2]
		rgba.Pix[i*4] = r
		rgba.Pix[i*4+1] = g
		rgba.Pix[i*4+2] = b
		rgba.Pix[i*4+3] = 255
	}
	return rgba
}

func cameraFrameToImage(frame *camera.Frame) image.Image {
	if frame == nil || len(frame.Data) == 0 {
		return nil
	}
	if frame.Width <= 0 || frame.Height <= 0 {
		return nil
	}

	// Handle based on channel count
	switch frame.Channels {
	case 3:
		width := frame.Width
		height := frame.Height
		rgba := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				srcIdx := y*frame.Width*3 + x*3
				dstIdx := (y*width + x) * 4
				rgba.Pix[dstIdx+0] = frame.Data[srcIdx+2] // R (from BGR)
				rgba.Pix[dstIdx+1] = frame.Data[srcIdx+1] // G
				rgba.Pix[dstIdx+2] = frame.Data[srcIdx+0] // B
				rgba.Pix[dstIdx+3] = 255                  // A
			}
		}
		return rgba
	case 4:
		rgba := &image.RGBA{
			Pix:    frame.Data,
			Stride: frame.Width * frame.Channels,
			Rect:   image.Rect(0, 0, frame.Width, frame.Height),
		}
		return rgba
	case 1:
		gray := &image.Gray{
			Pix:    frame.Data,
			Stride: frame.Width,
			Rect:   image.Rect(0, 0, frame.Width, frame.Height),
		}
		return gray
	default:
		return nil
	}
}
