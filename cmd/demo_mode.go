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
)

var demoObstacles = []DemoObstacle{
	{Name: "Person 1", ClassName: "person", X: 400, Y: 300, Width: 80, Height: 120, Confidence: 0.92, Moving: true, VX: 2, VY: 1},
	{Name: "Cup", ClassName: "cup", X: 800, Y: 200, Width: 40, Height: 40, Confidence: 0.88, Moving: false, VX: 0, VY: 0},
	{Name: "Chair", ClassName: "chair", X: 200, Y: 500, Width: 100, Height: 100, Confidence: 0.95, Moving: false, VX: 0, VY: 0},
	{Name: "Laptop", ClassName: "laptop", X: 600, Y: 400, Width: 70, Height: 50, Confidence: 0.78, Moving: true, VX: -1, VY: 0},
}

func updateDemoObstacles() {
	for i := range demoObstacles {
		if demoObstacles[i].Moving {
			demoObstacles[i].X += demoObstacles[i].VX
			demoObstacles[i].Y += demoObstacles[i].VY
			if demoObstacles[i].X < 50 || demoObstacles[i].X > 1190 {
				demoObstacles[i].VX *= -1
			}
			if demoObstacles[i].Y < 50 || demoObstacles[i].Y > 670 {
				demoObstacles[i].VY *= -1
			}
		}
	}
}

func demoObstaclesToYOLO() []detection.YOLODetection {
	detections := make([]detection.YOLODetection, 0, len(demoObstacles))
	for _, obs := range demoObstacles {
		detections = append(detections, detection.YOLODetection{
			Bbox: &detection.BoundingBox{
				X1: obs.X - obs.Width/2,
				Y1: obs.Y - obs.Height/2,
				X2: obs.X + obs.Width/2,
				Y2: obs.Y + obs.Height/2,
			},
			ClassName:  obs.ClassName,
			Confidence: obs.Confidence,
		})
	}
	return detections
}

func drawDemoObstaclesOnImage(img *image.RGBA, obstacles []DemoObstacle) {
	colors := map[string]color.RGBA{
		"person": {255, 0, 0, 255},
		"cup":    {0, 255, 0, 255},
		"chair":  {0, 0, 255, 255},
		"laptop": {255, 165, 0, 255},
	}

	for _, obs := range obstacles {
		x1 := obs.X - obs.Width/2
		y1 := obs.Y - obs.Height/2
		x2 := obs.X + obs.Width/2
		y2 := obs.Y + obs.Height/2

		rect := image.Rect(x1, y1, x2, y2)
		c := colors[obs.ClassName]
		draw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, draw.Src)

		for x := x1; x <= x2; x++ {
			img.Set(x, y1, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			img.Set(x, y2, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
		for y := y1; y <= y2; y++ {
			img.Set(x1, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			img.Set(x2, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
}

func RunSelfTest(rs *RobotSystem) {
	fmt.Println("=== Dynamic Obstacle Self-Test ===")
	fmt.Println()

	if rs.cfg == nil {
		rs.cfg = &config.Config{
			LocalPlanning: config.LocalPlanningConfig{
				ObstacleClasses: []string{"person", "cup", "chair", "laptop", "keyboard", "bottle"},
				MinConfidence:   0.5,
			},
		}
	}

	fmt.Println("Test 1: Creating fake YOLO detections...")
	fakeDetections := []detection.YOLODetection{
		{Bbox: &detection.BoundingBox{X1: 100, Y1: 200, X2: 200, Y2: 300}, ClassName: "person", Confidence: 0.85},
		{Bbox: &detection.BoundingBox{X1: 400, Y1: 150, X2: 480, Y2: 230}, ClassName: "cup", Confidence: 0.72},
		{Bbox: &detection.BoundingBox{X1: 600, Y1: 400, X2: 750, Y2: 550}, ClassName: "chair", Confidence: 0.91},
	}
	fmt.Printf("  Created %d fake YOLO detections\n", len(fakeDetections))

	fmt.Println()
	fmt.Println("Test 2: Converting to dynamic obstacles...")
	relevantClasses := classesToMap(rs.cfg.LocalPlanning.ObstacleClasses)
	obstacles := detection.YOLODetectionsToDynamicObstacles(
		fakeDetections, nil, relevantClasses, rs.cfg.LocalPlanning.MinConfidence)

	fmt.Printf("  Created %d dynamic obstacles\n", len(obstacles))
	for i, obs := range obstacles {
		fmt.Printf("  [%d] %s at (%.2f, %.2f) radius=%.2f confidence=%.2f\n",
			i, obs.ClassName, obs.X, obs.Y, obs.Radius, obs.Confidence)
	}

	fmt.Println()
	fmt.Println("Test 3: Testing planner integration...")

	if rs.planner == nil {
		plannerConfig := &planning.PlannerConfig{
			AStarConfig: &planning.AStarConfig{
				GridWidth:     100,
				GridHeight:    100,
				Resolution:    0.05,
				MaxIterations: 10000,
			},
			VelocityObstacleConfig: &planning.VelocityObstacleConfig{
				TimeHorizon:     2.0,
				SafetyMargin:    0.15,
				MaxVelocity:     0.15,
				MaxAcceleration: 0.3,
				TimeStep:        0.1,
			},
			CollisionMargin: 0.02,
		}
		rs.planner = planning.NewPlanner(plannerConfig)
		fmt.Println("  Created new planner for testing")
	}

	rs.planner.AddRobot(1, [2]float64{0.5, 0.5}, 0.30)
	goal := [2]float64{1.0, 1.0}

	velocity, shouldPause := rs.planner.ComputeVelocityWithDynamicObstacles(
		1, goal, obstacles, rs.cfg.LocalPlanning.MinConfidence)

	fmt.Printf("  Computed velocity: (%.4f, %.4f)\n", velocity[0], velocity[1])
	fmt.Printf("  Should pause: %v\n", shouldPause)

	fmt.Println()
	fmt.Println("Test 4: Testing demo obstacles (visual mode)...")
	fmt.Println("  Demo obstacles available:")
	for _, obs := range demoObstacles {
		fmt.Printf("    - %s (%s) at (%d, %d) moving=%v\n",
			obs.Name, obs.ClassName, obs.X, obs.Y, obs.Moving)
	}

	fmt.Println()
	fmt.Println("Test 5: Testing demo-to-YOLO conversion...")
	demoYOLO := demoObstaclesToYOLO()
	fmt.Printf("  Converted %d demo obstacles to YOLO format\n", len(demoYOLO))

	fmt.Println()
	fmt.Println("Test 6: Testing DynamicObstacle methods...")
	testObs := obstacles[0]
	fmt.Printf("  ContainsPoint(center): %v\n", testObs.ContainsPoint(testObs.X, testObs.Y))
	fmt.Printf("  ContainsPoint(outside): %v\n", testObs.ContainsPoint(testObs.X+100, testObs.Y+100))
	fmt.Printf("  DistanceTo(1,1): %.4f\n", testObs.DistanceTo(1.0, 1.0))

	if len(obstacles) >= 2 {
		dist := obstacles[0].DistanceToObstacle(obstacles[1])
		fmt.Printf("  Distance between obs[0] and obs[1]: %.4f\n", dist)
	}

	fmt.Println()
	fmt.Println("Test 7: Testing obstacle filtering by class...")
	personClasses := map[string]bool{"person": true}
	personOnly := detection.YOLODetectionsToDynamicObstacles(
		fakeDetections, nil, personClasses, 0.5)
	fmt.Printf("  Filtered to 'person' only: %d obstacles\n", len(personOnly))

	highConf := detection.YOLODetectionsToDynamicObstacles(
		fakeDetections, nil, relevantClasses, 0.9)
	fmt.Printf("  Filtered by confidence > 0.9: %d obstacles\n", len(highConf))

	fmt.Println()
	if velocity[0] != 0 || velocity[1] != 0 {
		fmt.Println("PASS: Non-zero velocity computed (obstacles influencing path)")
	} else {
		fmt.Println("NOTE: Zero velocity - may indicate immediate collision or specific path conditions")
	}

	fmt.Println()
	fmt.Println("=== Self-Test Complete ===")
	fmt.Println()
	fmt.Println("Demo mode with YOLO obstacles can be run with:")
	fmt.Println("  ./robot_tracker.exe --demo-yolo")
	fmt.Println()
	fmt.Println("This will display moving obstacles on the demo video stream.")
}

func RunDemoYOLOMode(rs *RobotSystem) {
	fmt.Println("Demo YOLO mode: Generating test pattern with dynamic obstacles...")

	width, height := 1280, 720
	frameNum := 0

	if rs.cfg == nil {
		rs.cfg = &config.Config{
			LocalPlanning: config.LocalPlanningConfig{
				ObstacleClasses: []string{"person", "cup", "chair", "laptop", "keyboard", "bottle"},
				MinConfidence:   0.5,
			},
		}
	}

	_ = rs.StartCamera()

	for {
		frame := generateTestPattern(width, height, frameNum)
		demoTags := generateDemoTags(width, height, frameNum)

		rgbaImg, ok := frame.(*image.RGBA)
		if !ok {
			rgbaImg = image.NewRGBA(frame.Bounds())
			draw.Draw(rgbaImg, frame.Bounds(), frame, frame.Bounds().Min, draw.Src)
		}

		updateDemoObstacles()
		rgbaWithTags := drawDemoTagsOnImage(rgbaImg, demoTags)
		drawDemoObstaclesOnImage(rgbaWithTags, demoObstacles)

		detectionObstacles := convertDemoObstaclesToDetection(demoObstacles)
		if rs.detectionPipe != nil {
			rs.detectionPipe.SetObstacles(detectionObstacles)
		}

		fakeYOLO := demoObstaclesToYOLO()

		relevantClasses := map[string]bool{
			"person": true, "cup": true, "chair": true,
			"laptop": true, "keyboard": true, "bottle": true,
		}
		rs.DynamicObstacles = detection.YOLODetectionsToDynamicObstacles(
			fakeYOLO, rs.positionEst, relevantClasses, 0.5)

		demoTagsResult := &detection.DetectionResult{
			Tags:            demoTags,
			YOLODetections:  fakeYOLO,
			FusedDetections: []detection.FusedDetection{},
			Timestamp:       float64(frameNum) / 30.0,
			FrameIdx:        frameNum,
		}

		trackingDetections := rs.convertFusedToTrackingDetections(demoTagsResult.FusedDetections)
		timestamp := float64(time.Now().UnixNano()) / 1e9
		trackingResult := rs.tracker.Update(trackingDetections, timestamp, frameNum)

		for _, track := range trackingResult.Tracks {
			if track.State == tracking.TrackStateConfirmed && track.TagID != nil {
				rs.CurrentRobotID = *track.TagID
				if rs.positionEst != nil {
					px, py := (track.Bbox[0]+track.Bbox[2])/2, (track.Bbox[1]+track.Bbox[3])/2
					worldPos := rs.positionEst.PixelToWorld(px, py)
					rs.positionEst.UpdatePosition(track.TrackID, worldPos.X, worldPos.Y)
					robotDiameter := 0.30 // fallback
				if robotConfig := rs.cfg.GetRobotByTagID(*track.TagID); robotConfig != nil {
					robotDiameter = robotConfig.Diameter
				}
				rs.planner.AddRobot(track.TrackID, [2]float64{worldPos.X, worldPos.Y}, robotDiameter)
				}
			}
		}

		for _, track := range trackingResult.Tracks {
			if track.State != tracking.TrackStateConfirmed || track.TagID == nil {
				continue
			}

			robotID := *track.TagID

			if _, hasPath := rs.planner.GetNextWaypoint(robotID); hasPath {
				if rs.positionEst == nil {
					continue
				}

				px, py := (track.Bbox[0]+track.Bbox[2])/2, (track.Bbox[1]+track.Bbox[3])/2
				worldPos := rs.positionEst.PixelToWorld(px, py)

				// Advance past any reached or overshot waypoints
				if !rs.planner.AdvancePastWaypoints(robotID, [2]float64{worldPos.X, worldPos.Y}, rs.waypointThreshold) {
					fmt.Printf("Demo: Robot %d reached final waypoint, stopping\n", robotID)
					continue
				}

				// Get updated waypoint after advancing
				waypoint, stillHasPath := rs.planner.GetNextWaypoint(robotID)
				if !stillHasPath {
					continue
				}

				robotState := planning.RobotState{
					Position: [2]float64{worldPos.X, worldPos.Y},
					Velocity: planning.Velocity{VX: 0, VY: 0},
					RobotID:  robotID,
					Diameter: 0.18,
				}

				velocity, _ := rs.planner.LocalPlanner().ComputeVelocityToWaypoint(
					robotState, waypoint, rs.StaticObstacles, rs.DynamicObstacles, 0.5)

				if rs.pathExecutor != nil {
					cmd := rs.pathExecutor.VelocityToCommand(velocity[0], velocity[1])
					fmt.Printf("Demo path exec: robot %d -> cmd %c (vel %.3f, %.3f)\n", robotID, cmd, velocity[0], velocity[1])
				}

				rs.planner.UpdateRobotState(robotID, [2]float64{worldPos.X, worldPos.Y},
					[2]float64{velocity[0], velocity[1]})
			}
		}

		if frameNum%30 == 0 {
			line := fmt.Sprintf("Demo YOLO: %d det, %d obs", len(fakeYOLO), len(rs.DynamicObstacles))
			for _, obs := range demoObstacles {
				if obs.Moving {
					line += fmt.Sprintf(" | %s@(%d,%d)", obs.ClassName, obs.X, obs.Y)
				}
			}
			fmt.Println(line)
		}

		overlay := rs.detectionPipe.DrawResults(rgbaImg.Pix, width, height, demoTagsResult)
		var overlayImg image.Image
		if overlay != nil {
			overlayImg = decodeToImage(overlay, width, height)
		}
		switch {
		case rs.webServer.CalibrationViewActive():
			rs.webServer.PushFrame(rgbaImg)
		case overlayImg != nil:
			rs.webServer.PushFrame(overlayImg)
		default:
			rs.webServer.PushFrame(rgbaWithTags)
		}

		rs.webServer.UpdateStats(len(demoTags), len(fakeYOLO))
		if frameNum%15 == 0 {
			rs.broadcastDemoTempObstacles()
		}

		detectedTags := make([]ui.DetectedTagInfo, 0, len(demoTags))
		for _, tag := range demoTags {
			detectedTags = append(detectedTags, ui.DetectedTagInfo{
				ID:      tag.TagID,
				Center:  [2]float64{tag.CenterX, tag.CenterY},
				Corners: tag.Corners,
			})
		}
		detectedTags = append(detectedTags, demoCalibrationTagInfos(width, height)...)
		rs.webServer.UpdateDetectedTags(detectedTags, width, height)

		frameNum++
		time.Sleep(33 * time.Millisecond)
	}
}

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

// broadcastDemoTempObstacles shows the demo obstacles as temporary obstacles in
// the UI, so the overlay and controls can be exercised without a camera. Demo
// frames are 640x480 and the demo has no real floor mapping, so world
// coordinates are just pixels / 100.
func (rs *RobotSystem) broadcastDemoTempObstacles() {
	msgs := make([]ui.TempObstacleResponse, 0, len(demoObstacles))
	for i, obs := range demoObstacles {
		x0, y0 := obs.X-obs.Width/2, obs.Y-obs.Height/2
		x1, y1 := obs.X+obs.Width/2, obs.Y+obs.Height/2
		msgs = append(msgs, ui.TempObstacleResponse{
			ID:               fmt.Sprintf("temp_%d", i+1),
			PixelTopLeft:     [2]int{x0, y0},
			PixelBottomRight: [2]int{x1, y1},
			WorldTopLeft:     [2]float64{float64(x0) / 100, float64(y0) / 100},
			WorldBottomRight: [2]float64{float64(x1) / 100, float64(y1) / 100},
		})
	}
	rs.webServer.BroadcastTempObstacles(ui.TempObstaclesMessage{Obstacles: msgs, Enabled: true})
}
