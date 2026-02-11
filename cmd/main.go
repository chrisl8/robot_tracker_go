//go:build gocv

package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"robot_tracker_go/internal/camera"
	"robot_tracker_go/internal/config"
	"robot_tracker_go/internal/controller"
	"robot_tracker_go/internal/detection"
	"robot_tracker_go/internal/planning"
	"robot_tracker_go/internal/position"
	"robot_tracker_go/internal/tracking"
	"robot_tracker_go/internal/ui"
	"robot_tracker_go/internal/utils"
)

type RobotSystem struct {
	cfg               *config.Config
	cam               camera.Camera
	detectionPipe     *detection.DetectionPipeline
	tracker           tracking.Tracker
	planner           *planning.Planner
	positionEst       *position.PositionEstimator
	arduino           *controller.ArduinoController
	commandQueue      *controller.CommandQueue
	pathExecutor      *controller.PathExecutor
	webServer         *ui.WebServer
	cameraRunning     bool
	frameNum          int
	DynamicObstacles  []*planning.DynamicObstacle
	StaticObstacles   []planning.Obstacle
	CurrentRobotID    int
	CurrentGoal       [2]float64
	waypointThreshold float64
}

func NewRobotSystem(cfg *config.Config) *RobotSystem {
	return &RobotSystem{
		cfg:           cfg,
		cameraRunning: false,
		frameNum:      0,
		CurrentGoal:   [2]float64{0, 0},
	}
}

func getLocalIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "unknown"
	}

	preferredRanges := []string{"192.168.", "10."}
	skipPrefixes := []string{"docker0", "br-", "veth", "vmnet", "virbr"}

	var preferredIP string
	var fallbackIP string

	for _, iface := range interfaces {
		skip := false
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(iface.Name, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				ipStr := ipNet.IP.String()
				if ipNet.IP.IsLoopback() {
					continue
				}

				isPrivate := false
				for _, pref := range preferredRanges {
					if strings.HasPrefix(ipStr, pref) {
						isPrivate = true
						break
					}
				}

				if isPrivate {
					if strings.HasPrefix(ipStr, "192.168.") {
						return ipStr
					}
					if preferredIP == "" {
						preferredIP = ipStr
					}
				} else if fallbackIP == "" {
					fallbackIP = ipStr
				}
			}
		}
	}

	if preferredIP != "" {
		return preferredIP
	}
	if fallbackIP != "" {
		return fallbackIP
	}
	return "unknown"
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func getWebUIURLs(port string) string {
	localIP := getLocalIP()
	hostname := getHostname()
	return fmt.Sprintf("\n\n---------------------------------\n--     Web UI started at:\n--     http://localhost:%s\n--     http://%s:%s\n--     http://%s:%s\n---------------------------------\n\n", port, localIP, port, hostname, port)
}

func (rs *RobotSystem) initDemoMode() {
	rs.webServer = ui.NewWebServer(":9086")

	tagConfig := detection.AprilTagConfig{
		Family:       "tag36h11",
		QuadDecimate: 2.0,
	}
	rs.detectionPipe = detection.NewDetectionPipeline(nil, tagConfig)
	utils.Logf("Demo mode: Detection pipeline initialized (YOLO disabled)")

	rs.webServer.Start()
	utils.Log(getWebUIURLs("9086"))

	rs.webServer.OnObstaclesChanged = func(obstacles []planning.Obstacle) {
		utils.Logf("DEBUG: OnObstaclesChanged callback triggered with %d obstacles", len(obstacles))
		rs.planner.SetObstacles(obstacles)

		detectionObstacles := make([]detection.Obstacle, len(obstacles))
		for i, obs := range obstacles {
			utils.Logf("DEBUG: Converting obstacle '%s': pixels [%d,%d] to [%d,%d]",
				obs.Name, obs.PixelsTopLeft[0], obs.PixelsTopLeft[1], obs.PixelsBottomRight[0], obs.PixelsBottomRight[1])
			detectionObstacles[i] = detection.Obstacle{
				ID:               obs.Name,
				PixelTopLeft:     [2]int{obs.PixelsTopLeft[0], obs.PixelsTopLeft[1]},
				PixelBottomRight: [2]int{obs.PixelsBottomRight[0], obs.PixelsBottomRight[1]},
				WorldTopLeft:     obs.WorldTopLeft,
				WorldBottomRight: obs.WorldBottomRight,
				Clearance:        0.05,
			}
		}
		rs.detectionPipe.SetObstacles(detectionObstacles)
		utils.Logf("DEBUG: SetObstacles called with %d detection obstacles", len(detectionObstacles))
	}

	rs.webServer.OnDestinationSet = func(robotID int, pixelPos [2]float64) {
		utils.Logf("Demo mode: Destination set for robot %d at pixel(%d,%d)",
			robotID, int(pixelPos[0]), int(pixelPos[1]))
	}
}

func classesToMap(classes []string) map[string]bool {
	m := make(map[string]bool)
	for _, c := range classes {
		m[c] = true
	}
	return m
}

func (rs *RobotSystem) Initialize() error {
	tagConfig := detection.AprilTagConfig{
		Family:       rs.cfg.AprilTags.Family,
		QuadDecimate: rs.cfg.AprilTags.QuadDecimate,
	}

	yoloConfig := &detection.YOLOConfig{
		ModelPath:       rs.cfg.YOLO.ModelPath,
		InputSize:       rs.cfg.YOLO.InputSize,
		ConfThres:       rs.cfg.YOLO.ConfThres,
		IOUThres:        rs.cfg.YOLO.IOUThres,
		Device:          rs.cfg.YOLO.Device,
		MinObstacleSize: rs.cfg.YOLO.MinObstacleSize,
		PixelsPerMeter:  rs.cfg.YOLO.PixelsPerMeter,
	}

	rs.detectionPipe = detection.NewDetectionPipeline(yoloConfig, tagConfig)
	utils.Logf("Detection pipeline initialized, YOLO enabled: %v", rs.detectionPipe.IsYOLOEnabled())

	trackConfig := &tracking.ByteTrackConfig{
		TrackThresh: rs.cfg.Tracking.TrackThresh,
		TrackBuffer: rs.cfg.Tracking.TrackBuffer,
		MatchThresh: rs.cfg.Tracking.MatchThresh,
		FrameRate:   rs.cfg.Tracking.FrameRate,
		MinBoxArea:  rs.cfg.Tracking.MinBoxArea,
		MOT20:       rs.cfg.Tracking.MOT20,
	}
	rs.tracker = tracking.NewByteTrack(trackConfig)
	utils.Logf("ByteTrack initialized")

	plannerConfig := &planning.PlannerConfig{
		AStarConfig:            nil,
		VelocityObstacleConfig: nil,
		CollisionMargin:        0.05,
	}
	rs.planner = planning.NewPlanner(plannerConfig)
	utils.Logf("Planner initialized")

	if primaryCam := rs.cfg.GetPrimaryCamera(); primaryCam != nil {
		camConfig := camera.CameraConfig{
			Type:     primaryCam.Type,
			Name:     primaryCam.Name,
			CameraID: primaryCam.CameraID,
			URL:      primaryCam.URL,
			Width:    primaryCam.Width,
			Height:   primaryCam.Height,
			FPS:      primaryCam.FPS,
		}
		cam, err := camera.NewCamera(camConfig)
		if err != nil {
			utils.Logf("Warning: Could not initialize camera: %v", err)
			rs.cam = nil
		} else {
			rs.cam = cam
			utils.Logf("Camera initialized: %s", rs.cam.GetName())
		}
	} else {
		utils.Logf("No camera configured, using demo mode")
	}

	calibrationPath := "config/calibration_default.yaml"
	if rs.cam != nil {
		calibrationPath = ui.GetCalibrationFilename(rs.cam.GetName())
		utils.Logf("Using calibration file: %s", calibrationPath)
	}
	obstaclesPath := ""
	if rs.cfg != nil && rs.cfg.Obstacles.GetPath() != "" {
		obstaclesPath = rs.cfg.Obstacles.GetPath()
	}
	posEst, err := position.NewPositionEstimator(calibrationPath, obstaclesPath, rs.cfg.Position.Smoothing, rs.cfg.Position.SmoothingAlpha)
	if err != nil {
		utils.Logf("Warning: Position estimator initialization failed: %v", err)
		rs.positionEst = nil
	} else {
		rs.positionEst = posEst
		utils.Logf("Position estimator initialized")
	}

	rs.arduino = controller.NewArduinoController("auto", controller.BaudRate)
	if err := rs.arduino.Connect(); err != nil {
		utils.Logf("Warning: Could not connect to Arduino: %v", err)
	} else {
		utils.Logf("Connected to Arduino on %s", rs.arduino.GetPort())
	}

	rs.commandQueue = controller.NewCommandQueue(rs.arduino, controller.CommandIntervalMs)
	rs.commandQueue.Start()
	utils.Logf("Command queue started")

	if rs.cfg != nil && rs.cfg.PathExecution.MaxSpeed > 0 {
		rs.pathExecutor = controller.NewPathExecutor(rs.cfg.PathExecution.MaxSpeed, rs.cfg.PathExecution.TurnSpeed)
		rs.waypointThreshold = rs.cfg.PathExecution.WaypointThreshold
	} else {
		rs.pathExecutor = controller.NewPathExecutor(0.15, 0.5)
		rs.waypointThreshold = 0.1
	}
	utils.Logf("Path executor initialized: max_speed=%.3f, turn_speed=%.3f, waypoint_threshold=%.3f",
		rs.pathExecutor.MaxSpeed(), rs.pathExecutor.TurnSpeed(), rs.waypointThreshold)

	rs.webServer = ui.NewWebServer(":9086")

	rs.webServer.OnObstaclesChanged = func(obstacles []planning.Obstacle) {
		utils.Logf("DEBUG: Initialize() OnObstaclesChanged callback triggered with %d obstacles", len(obstacles))
		rs.planner.SetObstacles(obstacles)

		detectionObstacles := make([]detection.Obstacle, len(obstacles))
		for i, obs := range obstacles {
			utils.Logf("DEBUG: Initialize() converting obstacle '%s': pixels [%d,%d] to [%d,%d]",
				obs.Name, obs.PixelsTopLeft[0], obs.PixelsTopLeft[1], obs.PixelsBottomRight[0], obs.PixelsBottomRight[1])
			detectionObstacles[i] = detection.Obstacle{
				ID:               obs.Name,
				PixelTopLeft:     [2]int{obs.PixelsTopLeft[0], obs.PixelsTopLeft[1]},
				PixelBottomRight: [2]int{obs.PixelsBottomRight[0], obs.PixelsBottomRight[1]},
				WorldTopLeft:     obs.WorldTopLeft,
				WorldBottomRight: obs.WorldBottomRight,
				Clearance:        0.05,
			}
		}
		rs.detectionPipe.SetObstacles(detectionObstacles)
		utils.Logf("DEBUG: Initialize() SetObstacles called with %d detection obstacles", len(detectionObstacles))
	}

	rs.webServer.OnDestinationSet = func(robotID int, pixelPos [2]float64) {
		if rs.positionEst == nil || !rs.positionEst.IsCalibrated() {
			utils.Logf("Cannot set destination: not calibrated")
			return
		}
		worldPos := rs.positionEst.PixelToWorld(int(pixelPos[0]), int(pixelPos[1]))
		utils.Logf("DEST: pixel(%d,%d) -> world(%.2f,%.2f) BEFORE SetGoal",
			int(pixelPos[0]), int(pixelPos[1]), worldPos.X, worldPos.Y)
		rs.planner.SetGoal(robotID, [2]float64{worldPos.X, worldPos.Y})
		utils.Logf("Destination set for robot %d: pixel(%d,%d) -> world(%.2f,%.2f)",
			robotID, int(pixelPos[0]), int(pixelPos[1]), worldPos.X, worldPos.Y)
	}

	rs.webServer.OnCalibrationComplete = func(calibFile string) {
		utils.Logf("Calibration complete, reloading from %s", calibFile)
		if rs.positionEst != nil {
			if err := rs.positionEst.LoadCalibration(calibFile); err != nil {
				utils.Logf("Failed to reload calibration: %v", err)
				return
			}
			rs.webServer.SetPositionEstimator(rs.positionEst)
			utils.Logf("Calibration reloaded: IsCalibrated=%v", rs.positionEst.IsCalibrated())
		}
	}

	rs.webServer.OnPathsChanged = func() map[int][][2]float64 {
		paths := rs.planner.GetPathsWithGoals()
		for rid, path := range paths {
			utils.Logf("  Robot %d: %d waypoints", rid, len(path))
			if len(path) > 0 {
				utils.Logf("    First: (%.2f, %.2f), Last: (%.2f, %.2f)",
					path[0][0], path[0][1], path[len(path)-1][0], path[len(path)-1][1])
			}
		}
		return paths
	}

	if rs.cam != nil {
		rs.webServer.SetCameraName(rs.cam.GetName())
	}
	if rs.positionEst != nil && rs.positionEst.IsCalibrated() {
		rs.webServer.SetPositionEstimator(rs.positionEst)
		utils.Logf("PATH VIS: PositionEstimator set on WebServer (calibrated=%v)",
			rs.positionEst.IsCalibrated())
		rs.webServer.SetCalibrationState("calibrated", "Calibration loaded", calibrationPath, 0.15)
		utils.Logf("Calibration loaded from %s", calibrationPath)
	} else {
		utils.Logf("PATH VIS: WARNING - PositionEstimator NOT set! IsCalibrated()=%v",
			rs.positionEst != nil && rs.positionEst.IsCalibrated())
	}
	rs.webServer.Start()
	utils.Log(getWebUIURLs("9086"))

	rs.loadStaticObstacles()

	return nil
}

func (rs *RobotSystem) loadStaticObstacles() {
	obstaclesPath := ""
	if rs.cfg != nil && rs.cfg.Obstacles.GetPath() != "" {
		configPath := rs.cfg.Obstacles.GetPath()
		if _, err := os.Stat(configPath); err == nil {
			obstaclesPath = configPath
			utils.Logf("Obstacles path from config: %s", obstaclesPath)
		} else {
			utils.Logf("Config obstacles file not found: %s", configPath)
		}
	}

	if obstaclesPath == "" && rs.webServer != nil {
		obstaclesPath = rs.webServer.GetObstaclesPath()
		utils.Logf("Obstacles path from webserver: %s", obstaclesPath)
	}
	if obstaclesPath == "" {
		obstaclesPath = "config/obstacles.yaml"
		utils.Logf("Using default obstacles path: %s", obstaclesPath)
	}

	utils.Logf("Loading obstacles from: %s", obstaclesPath)
	data, err := os.ReadFile(obstaclesPath)
	if err != nil {
		utils.Logf("No obstacles file found at %s", obstaclesPath)
		return
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		utils.Logf("Warning: Failed to parse obstacles file: %v", err)
		return
	}

	obstaclesData, ok := config["obstacles"]
	if !ok {
		return
	}

	obstaclesList, ok := obstaclesData.([]interface{})
	if !ok {
		return
	}

	for _, obsData := range obstaclesList {
		obs, ok := obsData.(map[string]interface{})
		if !ok {
			continue
		}

		pixels, ok := obs["pixels"].(map[string]interface{})
		if !ok {
			continue
		}

		world, _ := obs["world"].(map[string]interface{})

		var pixelsTL [2]int
		var pixelsBR [2]int
		var worldTL [2]float64
		var worldBR [2]float64

		// #nosec G602
		if tl, ok := pixels["top_left"].([]interface{}); ok && len(tl) >= 2 {
			pixelsTL[0] = int(toFloat64(tl[0]))
			pixelsTL[1] = int(toFloat64(tl[1]))
		}
		// #nosec G602
		if br, ok := pixels["bottom_right"].([]interface{}); ok && len(br) >= 2 {
			pixelsBR[0] = int(toFloat64(br[0]))
			pixelsBR[1] = int(toFloat64(br[1]))
		}

		if world != nil {
			// #nosec G602
			if tl, ok := world["top_left"].([]interface{}); ok && len(tl) >= 2 {
				worldTL[0] = toFloat64(tl[0])
				worldTL[1] = toFloat64(tl[1])
			}
			// #nosec G602
			if br, ok := world["bottom_right"].([]interface{}); ok && len(br) >= 2 {
				worldBR[0] = toFloat64(br[0])
				worldBR[1] = toFloat64(br[1])
			}
		}

		name := "obstacle"
		if n, ok := obs["name"].(string); ok {
			name = n
		}

		rs.StaticObstacles = append(rs.StaticObstacles, planning.Obstacle{
			Name:              name,
			WorldTopLeft:      worldTL,
			WorldBottomRight:  worldBR,
			PixelsTopLeft:     pixelsTL,
			PixelsBottomRight: pixelsBR,
		})
	}

	if len(rs.StaticObstacles) > 0 {
		utils.Logf("Loaded %d static obstacles from %s, calling SetObstacles", len(rs.StaticObstacles), obstaclesPath)
		rs.webServer.SetObstacles(rs.StaticObstacles)
	}
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

func (rs *RobotSystem) StartCamera() error {
	utils.Logf("Starting camera...")
	if rs.cam == nil {
		utils.Logf("No camera available")
		rs.cameraRunning = false
		return nil
	}
	if err := rs.cam.Start(); err != nil {
		utils.Logf("Failed to start camera: %v", err)
		rs.cameraRunning = false
		return err
	}
	rs.cameraRunning = true
	utils.Logf("Camera started: %s", rs.cam.GetName())
	return nil
}

func (rs *RobotSystem) Stop() {
	utils.Logf("Stopping system...")
	rs.cameraRunning = false
	if rs.cam != nil {
		rs.cam.Stop()
	}
	if rs.commandQueue != nil {
		rs.commandQueue.Stop()
	}
	if rs.arduino != nil {
		_ = rs.arduino.Disconnect()
	}
	if rs.webServer != nil {
		rs.webServer.Stop()
	}
	utils.Logf("System stopped")
}

func (rs *RobotSystem) convertFusedToTrackingDetections(fused []detection.FusedDetection) []tracking.Detection {
	detections := make([]tracking.Detection, 0, len(fused))
	for _, f := range fused {
		bbox := f.Bbox
		det := tracking.Detection{
			Bbox:       [4]int{bbox.X1, bbox.Y1, bbox.X2, bbox.Y2},
			Confidence: f.Confidence,
		}
		if f.TagID != nil {
			det.TagID = f.TagID
		}
		if f.ClassName != "" {
			classID := int(f.Confidence * 100)
			det.ClassID = classID
		}
		detections = append(detections, det)
	}
	return detections
}

func (rs *RobotSystem) ProcessFrame(img image.Image, frameData []byte) {
	if img == nil {
		return
	}

	rs.frameNum++
	timestamp := float64(time.Now().UnixNano()) / 1e9

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	rgbaImg, ok := img.(*image.RGBA)
	if rgbaImg == nil {
		rgbaImg = image.NewRGBA(bounds)
		draw.Draw(rgbaImg, bounds, img, bounds.Min, draw.Src)
	}
	_ = ok

	detectionResult := rs.detectionPipe.Detect(frameData, width, height, timestamp, rs.frameNum)

	relevantClasses := classesToMap(rs.cfg.LocalPlanning.ObstacleClasses)
	minConfidence := rs.cfg.LocalPlanning.MinConfidence
	rs.DynamicObstacles = detection.YOLODetectionsToDynamicObstacles(
		detectionResult.YOLODetections,
		rs.positionEst,
		relevantClasses,
		minConfidence,
	)

	trackingDetections := rs.convertFusedToTrackingDetections(detectionResult.FusedDetections)
	trackingResult := rs.tracker.Update(trackingDetections, timestamp, rs.frameNum)

	for i := range trackingResult.Tracks {
		track := &trackingResult.Tracks[i]
		if track.State == tracking.TrackStateConfirmed && track.TagID != nil {
			rs.CurrentRobotID = *track.TagID
			if rs.positionEst != nil {
				px, py := (track.Bbox[0]+track.Bbox[2])/2, (track.Bbox[1]+track.Bbox[3])/2
				worldPos := rs.positionEst.PixelToWorld(px, py)
				rs.positionEst.UpdatePosition(track.TrackID, worldPos.X, worldPos.Y)
				track.WorldPos = [2]float64{worldPos.X, worldPos.Y}
				if robotConfig := rs.cfg.GetRobotByTagID(*track.TagID); robotConfig != nil {
					track.PixelRadius = (robotConfig.Diameter / 2) * rs.cfg.YOLO.PixelsPerMeter
				}
				rs.planner.AddRobot(*track.TagID, [2]float64{worldPos.X, worldPos.Y}, 0.18)
			}
		}
	}

	rs.webServer.BroadcastTracks(trackingResult.Tracks)

	for i := range trackingResult.Tracks {
		track := &trackingResult.Tracks[i]
		if track.State != tracking.TrackStateConfirmed || track.TagID == nil {
			continue
		}

		robotID := *track.TagID

		if waypoint, hasPath := rs.planner.GetNextWaypoint(robotID); hasPath {
			if rs.positionEst == nil {
				continue
			}

			px, py := (track.Bbox[0]+track.Bbox[2])/2, (track.Bbox[1]+track.Bbox[3])/2
			worldPos := rs.positionEst.PixelToWorld(px, py)

			robotDiameter := 0.18
			if robotConfig := rs.cfg.GetRobotByTagID(robotID); robotConfig != nil {
				robotDiameter = robotConfig.Diameter
			}

			robotState := planning.RobotState{
				Position: [2]float64{worldPos.X, worldPos.Y},
				Velocity: planning.Velocity{VX: 0, VY: 0},
				RobotID:  robotID,
				Diameter: robotDiameter,
			}

			velocity, _ := rs.planner.LocalPlanner().ComputeVelocityToWaypoint(
				robotState, waypoint, rs.StaticObstacles, rs.DynamicObstacles, minConfidence)

			dx := waypoint[0] - worldPos.X
			dy := waypoint[1] - worldPos.Y
			distToWaypoint := math.Sqrt(dx*dx + dy*dy)
			if distToWaypoint < rs.waypointThreshold {
				rs.planner.AdvanceWaypoint(robotID)
				utils.Logf("Robot %d reached waypoint, advancing to next", robotID)
				continue
			}

			if rs.pathExecutor != nil && rs.commandQueue != nil {
				cmd := rs.pathExecutor.VelocityToCommand(velocity[0], velocity[1])
				rs.commandQueue.Enqueue(cmd)
				utils.Logf("Path exec robot %d: waypoint (%.2f,%.2f) -> vel (%.3f,%.3f) -> cmd %c",
					robotID, waypoint[0], waypoint[1], velocity[0], velocity[1], cmd)
			}

			rs.planner.UpdateRobotState(robotID, [2]float64{worldPos.X, worldPos.Y},
				[2]float64{velocity[0], velocity[1]})
		}
	}

	overlay := rs.detectionPipe.DrawResults(frameData, width, height, detectionResult)
	if len(overlay) > 0 && len(overlay) < width*height*3 {
		rs.webServer.PushRawJPEG(overlay)
	} else if img != nil {
		rs.webServer.PushFrame(img)
	}

	rs.webServer.UpdateStats(len(detectionResult.Tags), len(detectionResult.YOLODetections))

	detectedTags := make([]ui.DetectedTagInfo, 0, len(detectionResult.Tags))
	for _, tag := range detectionResult.Tags {
		detectedTags = append(detectedTags, ui.DetectedTagInfo{
			ID:      tag.TagID,
			Center:  [2]float64{tag.CenterX, tag.CenterY},
			Corners: tag.Corners,
		})
	}
	rs.webServer.UpdateDetectedTags(detectedTags)

	if rs.frameNum%10 == 0 && rs.webServer != nil {
		paths := rs.planner.GetPaths()
		numPaths := len(paths)
		totalWaypoints := 0
		for _, path := range paths {
			totalWaypoints += len(path)
		}
		if numPaths > 0 {
			utils.Logf("PATH DEBUG: %d robots with paths, %d total waypoints", numPaths, totalWaypoints)
		}
		rs.webServer.BroadcastPaths()
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

	tagCount := 3
	for i := 0; i < tagCount; i++ {
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
	if img == nil {
		return
	}

	rs.frameNum++
	timestamp := float64(time.Now().UnixNano()) / 1e9

	width := img.Rect.Max.X
	height := img.Rect.Max.Y

	result := &detection.DetectionResult{
		Tags:            demoTags,
		YOLODetections:  []detection.YOLODetection{},
		FusedDetections: []detection.FusedDetection{},
		Timestamp:       timestamp,
		FrameIdx:        rs.frameNum,
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

	if rs.tracker != nil {
		trackingDetections := rs.convertFusedToTrackingDetections(result.FusedDetections)
		trackingResult := rs.tracker.Update(trackingDetections, timestamp, rs.frameNum)

		for i := range trackingResult.Tracks {
			track := &trackingResult.Tracks[i]
			if track.State == tracking.TrackStateConfirmed && track.TagID != nil {
				rs.CurrentRobotID = *track.TagID
				if rs.positionEst != nil {
					px, py := (track.Bbox[0]+track.Bbox[2])/2, (track.Bbox[1]+track.Bbox[3])/2
					worldPos := rs.positionEst.PixelToWorld(px, py)
					rs.positionEst.UpdatePosition(track.TrackID, worldPos.X, worldPos.Y)
					track.WorldPos = [2]float64{worldPos.X, worldPos.Y}
					if robotConfig := rs.cfg.GetRobotByTagID(*track.TagID); robotConfig != nil {
						track.PixelRadius = (robotConfig.Diameter / 2) * rs.cfg.YOLO.PixelsPerMeter
					}
				}
			}
		}

		rs.webServer.BroadcastTracks(trackingResult.Tracks)
	}

	if rs.detectionPipe != nil && rs.webServer != nil {
		overlay := rs.detectionPipe.DrawResults(img.Pix, width, height, result)
		if overlay != nil {
			overlayImg := decodeToImage(overlay, width, height)
			if overlayImg != nil {
				rs.webServer.PushFrame(overlayImg)
			} else {
				rs.webServer.PushFrame(img)
			}
		} else {
			rs.webServer.PushFrame(img)
		}
	}

	if rs.webServer != nil {
		rs.webServer.UpdateStats(len(demoTags), 0)

		detectedTags := make([]ui.DetectedTagInfo, 0, len(demoTags))
		for _, tag := range demoTags {
			detectedTags = append(detectedTags, ui.DetectedTagInfo{
				ID:      tag.TagID,
				Center:  [2]float64{tag.CenterX, tag.CenterY},
				Corners: tag.Corners,
			})
		}
		rs.webServer.UpdateDetectedTags(detectedTags)
	}
}

//gocyclo:ignore
func main() {
	configPath := flag.String("config", "config/tracking_config.yaml", "Path to configuration file")
	listPorts := flag.Bool("list-ports", false, "List available serial ports")
	flag.String("web-port", ":9086", "Web server port")
	demoMode := flag.Bool("demo", false, "Run demo mode with test pattern")
	selfTestMode := flag.Bool("self-test", false, "Run self-test for dynamic obstacle pipeline")
	demoYOLOMode := flag.Bool("demo-yolo", false, "Run demo mode with YOLO obstacles visualization")
	quiet := flag.Bool("quiet", false, "Suppress all logging output")
	flag.Parse()

	if *quiet {
		utils.SetQuietMode(true)
	}

	if *listPorts {
		arduino := controller.NewArduinoController("auto", controller.BaudRate)
		ports := arduino.ListPorts()
		utils.Log("Available serial ports:")
		for _, p := range ports {
			utils.Logf("  - %s", p)
		}
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		utils.Logf("Warning: Could not load config: %v", err)
		utils.Logf("Running with demo mode only")
		*demoMode = true
	}

	rs := NewRobotSystem(cfg)
	if cfg != nil {
		if err := rs.Initialize(); err != nil {
			utils.Logf("Warning: Failed to initialize robot system: %v", err)
		}
	} else {
		rs.initDemoMode()
	}
	defer rs.Stop()

	utils.Log("Press Ctrl+C to exit.")

	if *selfTestMode {
		rs := NewRobotSystem(cfg)
		if cfg != nil {
			if err := rs.Initialize(); err != nil {
				utils.Logf("Warning: Failed to initialize robot system: %v", err)
			}
		}
		RunSelfTest(rs)
		return
	}

	if *demoYOLOMode {
		rs := NewRobotSystem(cfg)
		if cfg != nil {
			if err := rs.Initialize(); err != nil {
				utils.Logf("Warning: Failed to initialize robot system: %v", err)
			}
		}
		RunDemoYOLOMode(rs)
		return
	}

	if rs.cam != nil && !*demoMode {
		utils.Log("Starting real camera capture...")
		if err := rs.StartCamera(); err != nil {
			utils.Logf("Failed to start camera: %v, falling back to demo mode", err)
			*demoMode = true
		} else {
			utils.Logf("Starting real camera capture...")
			frameNum := 0
			for rs.cameraRunning {
				startTime := time.Now()
				frame, err := rs.cam.GetFrame()
				if err != nil {
					utils.Logf("Failed to get frame: %v", err)
					time.Sleep(100 * time.Millisecond)
					continue
				}
				if frame == nil || len(frame.Data) == 0 {
					utils.Logf("Empty frame received")
					time.Sleep(100 * time.Millisecond)
					continue
				}
				img := cameraFrameToImage(frame)
				if img == nil {
					utils.Logf("Failed to convert frame to image")
					time.Sleep(100 * time.Millisecond)
					continue
				}
				rs.ProcessFrame(img, frame.Data)
				frameNum++
				elapsed := time.Since(startTime)
				if elapsed < 33*time.Millisecond {
					time.Sleep(33*time.Millisecond - elapsed)
				}
			}
		}
	}

	for *demoMode {
		utils.Log("Demo mode: Generating test pattern with AprilTag visualization...")
		_ = rs.StartCamera()
		frameNum := 0
		for {
			frame := generateTestPattern(640, 480, frameNum)
			demoTags := generateDemoTags(640, 480, frameNum)
			rgbaImg, ok := frame.(*image.RGBA)
			if !ok {
				rgbaImg = image.NewRGBA(frame.Bounds())
				draw.Draw(rgbaImg, frame.Bounds(), frame, frame.Bounds().Min, draw.Src)
			}
			rgbaWithTags := drawDemoTagsOnImage(rgbaImg, demoTags)
			rs.ProcessDemoFrame(rgbaWithTags, frameNum, demoTags)
			frameNum++
			time.Sleep(33 * time.Millisecond)
		}
	}

	if cfg != nil {
		executor := controller.NewPathExecutor(0.15, 1.0)
		testCommands := []controller.Command{
			controller.CommandForward,
			controller.CommandLeft,
			controller.CommandRight,
			controller.CommandStop,
		}

		for _, cmd := range testCommands {
			utils.Logf("Sending command: %c", cmd)
			rs.commandQueue.Enqueue(cmd)
			vel := executor.CommandToVelocity(cmd)
			utils.Logf("  Velocity: (%.2f, %.2f)", vel.VX, vel.VY)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	utils.Log("\nShutting down...")
}
