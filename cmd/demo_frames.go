//go:build gocv

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/config"
	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
	"github.com/chrisl8/robot_tracker_go/internal/ui"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// demoCameraName is the camera name used for calibration files while running
// on synthetic demo data, so a demo run (including Playwright's) can never read
// or overwrite the real camera's calibration.
const demoCameraName = "demo"

func (rs *RobotSystem) initDemoMode() {
	rs.web.webServer = ui.NewWebServer(":9086")

	tagConfig := detection.AprilTagConfig{
		Family:       "tag36h11",
		QuadDecimate: 2.0,
	}
	rs.detection.detectionPipe = detection.NewDetectionPipeline(tagConfig)
	utils.Logf("Demo mode: Detection pipeline initialized")

	rs.web.webServer.Start()
	utils.Log(getWebUIURLs("9086"))

	rs.registerDemoCallbacks()
}

// registerDemoCallbacks wires the web server for the no-config demo fallback.
// That path builds no planner, tracker or config, so the callbacks must
// tolerate a nil planner.
func (rs *RobotSystem) registerDemoCallbacks() {
	rs.web.webServer.Callbacks.OnObstaclesChanged = func(obstacles []planning.Obstacle) {
		utils.Debugf("DEBUG: OnObstaclesChanged callback triggered with %d obstacles", len(obstacles))
		if rs.planning.planner != nil {
			rs.planning.planner.SetObstacles(obstacles)
		}

		detectionObstacles := convertPlanningObstaclesToDetection(obstacles)
		rs.detection.detectionPipe.SetObstacles(detectionObstacles)
		utils.Debugf("DEBUG: SetObstacles called with %d detection obstacles", len(detectionObstacles))
	}

	rs.web.webServer.Callbacks.OnDestinationSet = func(robotID int, pixelPos [2]float64) error {
		utils.Logf("Demo mode: Destination set for robot %d at pixel(%d,%d)",
			robotID, int(pixelPos[0]), int(pixelPos[1]))
		return nil
	}
}

// demoTagCount is how many synthetic robots the demo generates; their tag IDs
// are 1..demoTagCount. registerDemoRobots makes them real configured robots so
// the demo goes through exactly the same rules as a real run.
const demoTagCount = 3

// demoRobotDiameter is the body size (metres) given to registered demo robots.
const demoRobotDiameter = 0.30

// registerDemoRobots adds the demo's synthetic robots to the configuration,
// leaving any robot the config already defines (e.g. tag 1) untouched. Without
// this the demo's tags are "unconfigured" and, correctly, cannot be given goals.
func (rs *RobotSystem) registerDemoRobots() {
	if rs.cfg == nil {
		return
	}
	for id := 1; id <= demoTagCount; id++ {
		rs.cfg.AddRobotIfMissing(config.RobotConfig{
			TagID:    id,
			Name:     fmt.Sprintf("demo_robot_%d", id),
			Diameter: demoRobotDiameter,
		})
	}
}

func generateTestPattern(width, height int, frameNum int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bgColor := color.RGBA{20, 20, 40, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	gridColor := color.RGBA{50, 50, 70, 255}
	for x := 0; x < width; x += 50 {
		for y := 0; y < height; y++ {
			img.Set(x, y, gridColor)
		}
	}
	for y := 0; y < height; y += 50 {
		for x := 0; x < width; x++ {
			img.Set(x, y, gridColor)
		}
	}

	numRobots := 3
	for i := 0; i < numRobots; i++ {
		radius := 100.0
		cx := float64(width)/2 + float64(i-1)*80
		cy := float64(height) / 2
		x := int(cx + radius*float64(i)*0.3*float64(frameNum)*0.01)
		y := int(cy + radius*float64(i)*0.5*float64(frameNum)*0.01)

		// #nosec G115
		robotColor := color.RGBA{uint8(78 + i*50), uint8(204 - i*30), 163, 255}
		for dy := -20; dy <= 20; dy++ {
			for dx := -20; dx <= 20; dx++ {
				if dx*dx+dy*dy <= 400 {
					img.Set(x+dx, y+dy, robotColor)
				}
			}
		}

		for tx := x - 25; tx <= x+25; tx++ {
			if tx >= 0 && tx < width && y-35 >= 0 && y-35 < height {
				img.Set(tx, y-35, color.RGBA{0, 0, 0, 200})
			}
		}
	}

	timeStr := time.Now().Format("15:04:05")
	for x := 0; x < 7*10; x++ {
		for y := 20; y < 35; y++ {
			if x < len(timeStr)*10 {
				img.Set(width-100+x, y, color.RGBA{0, 0, 0, 200})
			}
		}
	}
	for x := 0; x < len(timeStr)*10; x++ {
		for y := 20; y < 35; y++ {
			img.Set(width-100+x, y+1, color.White)
		}
	}

	return img
}

func generateDemoTags(width, height int, frameNum int) []detection.AprilTag {
	tags := []detection.AprilTag{}

	for i := 0; i < demoTagCount; i++ {
		angle := float64(frameNum+i*100) * 0.01
		radius := 100.0 + float64(i)*30
		cx := float64(width)/2 + radius*float64(i-1)*0.2*float64(frameNum)*0.01
		cy := float64(height)/2 + radius*float64(i)*0.3*float64(frameNum)*0.01

		size := 60.0
		corners := [4][2]float64{
			{cx - size, cy - size},
			{cx + size, cy - size},
			{cx + size, cy + size},
			{cx - size, cy + size},
		}

		tags = append(tags, detection.AprilTag{
			TagID:    i + 1,
			Family:   "tag36h11",
			Corners:  corners,
			CenterX:  cx,
			CenterY:  cy,
			Size:     size * 2,
			Rotation: angle,
		})
	}

	return tags
}

func drawDemoTagsOnImage(img *image.RGBA, tags []detection.AprilTag) *image.RGBA {
	borderColor := color.RGBA{0, 255, 0, 255}
	bgColor := color.RGBA{0, 255, 0, 200}
	centerColor := color.RGBA{255, 255, 255, 255}

	for _, tag := range tags {
		points := make([]image.Point, 4)
		for j := 0; j < 4; j++ {
			points[j] = image.Point{
				X: int(tag.Corners[j][0]),
				Y: int(tag.Corners[j][1]),
			}
		}

		lineWidth := 3
		for j := 0; j < 4; j++ {
			drawLineOnRGBA(img, points[j], points[(j+1)%4], borderColor, lineWidth)
		}

		cx := int(tag.CenterX)
		cy := int(tag.CenterY) - 25

		fontSize := 20
		boxWidth := fontSize
		boxHeight := fontSize

		boxRect := image.Rect(cx-boxWidth/2, cy, cx+boxWidth/2, cy+boxHeight)
		draw.Draw(img, boxRect, &image.Uniform{bgColor}, image.Point{}, draw.Src)

		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				if cx+dx >= 0 && cx+dx < img.Rect.Max.X && cy+dy >= 0 && cy+dy < img.Rect.Max.Y {
					img.Set(cx+dx, cy+dy, centerColor)
				}
			}
		}
	}

	return img
}

func drawLineOnRGBA(img *image.RGBA, p1, p2 image.Point, c color.RGBA, width int) {
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y

	// A zero-length segment would divide by dy == 0 below.
	if dx == 0 && dy == 0 {
		drawCircleOnRGBA(img, p1.X, p1.Y, width/2, c)
		return
	}

	if utils.Abs(dx) > utils.Abs(dy) {
		if p1.X > p2.X {
			p1, p2 = p2, p1
		}
		for x := p1.X; x <= p2.X; x++ {
			y := p1.Y + dy*(x-p1.X)/dx
			drawCircleOnRGBA(img, x, y, width/2, c)
		}
	} else {
		if p1.Y > p2.Y {
			p1, p2 = p2, p1
		}
		for y := p1.Y; y <= p2.Y; y++ {
			x := p1.X + dx*(y-p1.Y)/dy
			drawCircleOnRGBA(img, x, y, width/2, c)
		}
	}
}

func drawCircleOnRGBA(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				x := cx + dx
				y := cy + dy
				if x >= 0 && x < img.Rect.Max.X && y >= 0 && y < img.Rect.Max.Y {
					img.Set(x, y, c)
				}
			}
		}
	}
}

func (rs *RobotSystem) ProcessDemoFrame(img *image.RGBA, frameNum int, demoTags []detection.AprilTag) {
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
	timestamp := float64(frameStart.UnixNano()) / 1e9

	rs.updateFPS(frameStart)

	width := img.Rect.Max.X
	height := img.Rect.Max.Y
	rs.noteFrameSize(width, height)

	result := &detection.DetectionResult{
		Tags:            demoTags,
		FusedDetections: []detection.FusedDetection{},
		Timestamp:       timestamp,
		FrameIdx:        rs.stats.frameNum,
	}

	for _, tag := range demoTags {
		bbox := detection.BoundingBox{
			X1: int(tag.Corners[0][0]),
			Y1: int(tag.Corners[0][1]),
			X2: int(tag.Corners[2][0]),
			Y2: int(tag.Corners[2][1]),
		}
		tagID := tag.TagID
		result.FusedDetections = append(result.FusedDetections, detection.FusedDetection{
			DetectionType: detection.DetectionTypeAprilTag,
			Bbox:          &bbox,
			TagID:         &tagID,
			Confidence:    1.0,
			Corners:       tag.Corners,
			Source:        "april_tag",
		})
	}

	if rs.tracking.tracker != nil {
		trackingDetections := rs.convertFusedToTrackingDetections(result.FusedDetections)
		trackingResult := rs.tracking.tracker.Update(trackingDetections, timestamp, rs.stats.frameNum)

		for i := range trackingResult.Tracks {
			track := &trackingResult.Tracks[i]
			if track.State == tracking.TrackStateConfirmed && track.TagID != nil {
				if rs.position.positionEst != nil {
					// Mirror ProcessFrame's per-track wiring: register the
					// robot with the planner and compute its heading, so a
					// demo run behaves identically to a real camera run for
					// path planning and autonomous control (previously demo
					// tracks were drawn/broadcast but never registered with
					// the planner, so they never got a path, a heading, or
					// autonomous driving -- see docs/archived/code-review-2026-09-27.md
					// tech-debt #1).
					worldPos, robotDiameter := rs.updateTrackWorldPosition(track, true)
					if rs.planning.planner != nil {
						rs.planning.planner.AddRobot(*track.TagID, [2]float64{worldPos.X, worldPos.Y}, robotDiameter)
					}
					rs.computeTrackHeading(track, demoTags)
				}
			}
		}

		var robots []config.RobotConfig
		if rs.cfg != nil {
			robots = rs.cfg.Robots
		}
		rs.executeAutonomousControl(trackingResult.Tracks)

		// After autonomy, so the motion state sent is this frame's, not last frame's.
		rs.web.webServer.BroadcastTracks(trackingResult.Tracks, robots, rs.io.robotCommands)
	}

	if rs.detection.detectionPipe != nil && rs.web.webServer != nil && rs.web.webServer.CalibrationViewActive() {
		rs.web.webServer.PushFrame(img)
	} else if rs.web.webServer != nil {
		// The demo tags are already drawn into img, and the UI canvas draws the
		// obstacles, so the frame goes out as it is.
		rs.web.webServer.PushFrame(img)
	}

	rs.broadcastFrameStats(len(demoTags), len(demoTags), demoTags, width, height, demoCalibrationTagInfos(width, height))
}
