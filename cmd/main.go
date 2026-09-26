//go:build gocv

package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/chrisl8/robot_tracker_go/internal/camera"
	"github.com/chrisl8/robot_tracker_go/internal/config"
	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
	"github.com/chrisl8/robot_tracker_go/internal/ui"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

type ControlMode int

const (
	ControlModeIdle       ControlMode = iota
	ControlModeManual
	ControlModeAutonomous
)

func (m ControlMode) String() string {
	switch m {
	case ControlModeManual:
		return "manual"
	case ControlModeAutonomous:
		return "autonomous"
	default:
		return "hold"
	}
}

func ParseControlMode(s string) ControlMode {
	switch s {
	case "manual":
		return ControlModeManual
	case "autonomous":
		return ControlModeAutonomous
	case "hold":
		return ControlModeIdle
	default:
		return ControlModeIdle
	}
}

// demoCameraName is the camera name used for calibration files while running
// on synthetic demo data, so a demo run (including Playwright's) can never read
// or overwrite the real camera's calibration.
const demoCameraName = "demo"

type RobotSystem struct {
	demoMode          bool
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
	controlMode       ControlMode
	emergencyStopped  bool
	controlMu         sync.RWMutex
	lastHeading        map[int]float64
	headingDelta       map[int]float64
	smoothedHeading    map[int]float64
	headingRejectCount map[int]int
	headingLostCount    map[int]int
	robotCommands       map[int]string // tag_id -> current motion state
	lastCommandTime     time.Time
	trackingLostTimeout time.Duration
	lastFrameTime       time.Time
	smoothedFPS         float64
	startTime           time.Time
	cameraConfig        *camera.CameraConfig // stored for retry if initial open fails
	lastReplanTime      map[int]time.Time    // robotID -> last proximity replan time
}

func NewRobotSystem(cfg *config.Config) *RobotSystem {
	return &RobotSystem{
		cfg:           cfg,
		cameraRunning: false,
		frameNum:      0,
		CurrentGoal:   [2]float64{0, 0},
		lastHeading:        make(map[int]float64),
		headingDelta:       make(map[int]float64),
		smoothedHeading:    make(map[int]float64),
		headingRejectCount: make(map[int]int),
		headingLostCount:   make(map[int]int),
		robotCommands:      make(map[int]string),
		lastReplanTime:     make(map[int]time.Time),
		startTime:          time.Now(),
	}
}

func (rs *RobotSystem) GetControlMode() ControlMode {
	rs.controlMu.RLock()
	defer rs.controlMu.RUnlock()
	return rs.controlMode
}

func (rs *RobotSystem) SetControlMode(mode ControlMode) {
	rs.controlMu.Lock()
	defer rs.controlMu.Unlock()
	if rs.emergencyStopped {
		return
	}
	prev := rs.controlMode
	rs.controlMode = mode
	utils.Logf("Control mode changed to: %s", mode)

	// When leaving Manual or Autonomous, clear active command and stop the robot
	if prev != mode && (prev == ControlModeManual || prev == ControlModeAutonomous) {
		if rs.commandQueue != nil {
			rs.commandQueue.ClearActiveCommand()
			rs.commandQueue.Enqueue(controller.CommandStop)
		}
	}
}

func (rs *RobotSystem) IsEmergencyStopped() bool {
	rs.controlMu.RLock()
	defer rs.controlMu.RUnlock()
	return rs.emergencyStopped
}

func (rs *RobotSystem) EmergencyStop() {
	rs.controlMu.Lock()
	rs.emergencyStopped = true
	rs.controlMode = ControlModeIdle
	rs.controlMu.Unlock()

	// Send stop directly to Arduino, bypassing queue for reliability
	if rs.arduino != nil {
		_ = rs.arduino.SendCommand(controller.CommandStop)
	}
	if rs.commandQueue != nil {
		rs.commandQueue.EmergencyStop()
	}
	utils.Logf("EMERGENCY STOP activated")
}

func (rs *RobotSystem) ClearEmergencyStop() {
	rs.controlMu.Lock()
	rs.emergencyStopped = false
	rs.controlMu.Unlock()

	// Restart the command queue so it can accept commands again
	if rs.commandQueue != nil {
		rs.commandQueue.Start()
	}
	utils.Logf("Emergency stop cleared")
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
		utils.Debugf("DEBUG: OnObstaclesChanged callback triggered with %d obstacles", len(obstacles))
		rs.planner.SetObstacles(obstacles)

		detectionObstacles := make([]detection.Obstacle, len(obstacles))
		for i, obs := range obstacles {
			utils.Debugf("DEBUG: Converting obstacle '%s': pixels [%d,%d] to [%d,%d]",
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
		utils.Debugf("DEBUG: SetObstacles called with %d detection obstacles", len(detectionObstacles))
	}

	rs.webServer.OnDestinationSet = func(robotID int, pixelPos [2]float64) {
		utils.Logf("Demo mode: Destination set for robot %d at pixel(%d,%d)",
			robotID, int(pixelPos[0]), int(pixelPos[1]))
	}
}

// cameraDisplayName names the configured camera even before it has opened
// (e.g. while macOS is still waiting on camera permission), so the calibration
// file used for loading and saving never depends on camera start-up timing.
func (rs *RobotSystem) cameraDisplayName() string {
	if rs.demoMode {
		return demoCameraName
	}
	if rs.cam != nil {
		return rs.cam.GetName()
	}
	if rs.cameraConfig != nil {
		return camera.DisplayName(rs.cameraConfig.URL, rs.cameraConfig.CameraID)
	}
	return ""
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
		CollisionMargin:        0.08,
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
		rs.cameraConfig = &camConfig
		cam, err := camera.NewCamera(camConfig)
		if err != nil {
			utils.Logf("Warning: Could not initialize camera: %v (will retry)", err)
			rs.cam = nil
		} else {
			rs.cam = cam
			utils.Logf("Camera initialized: %s", rs.cam.GetName())
		}
	} else {
		utils.Logf("No camera configured, using demo mode")
	}

	calibrationPath := "config/calibration_default.yaml"
	if name := rs.cameraDisplayName(); name != "" {
		calibrationPath = ui.GetCalibrationFilename(name)
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

	// Initialize Arduino controller using config values
	serialPort := "auto"
	serialBaud := controller.BaudRate
	commandIntervalMs := controller.CommandIntervalMs
	if rs.cfg.Controller.Serial.Port != "" {
		serialPort = rs.cfg.Controller.Serial.Port
	}
	if rs.cfg.Controller.Serial.BaudRate > 0 {
		serialBaud = rs.cfg.Controller.Serial.BaudRate
	}
	controllerEnabled := rs.cfg.Controller.Enabled
	if rs.cfg.Controller.CommandInterval > 0 {
		commandIntervalMs = int(rs.cfg.Controller.CommandInterval * 1000)
	}

	rs.arduino = controller.NewArduinoController(serialPort, serialBaud)
	if controllerEnabled {
		if err := rs.arduino.Connect(); err != nil {
			utils.Logf("Warning: Could not connect to Arduino: %v", err)
		} else {
			utils.Logf("Connected to Arduino on %s at %d baud", rs.arduino.GetPort(), serialBaud)
		}
	} else {
		utils.Logf("Controller disabled in config, skipping Arduino connection")
	}

	rs.commandQueue = controller.NewCommandQueue(rs.arduino, commandIntervalMs)
	rs.commandQueue.Start()
	if rs.arduino.IsConnected() {
		utils.Logf("Command queue started (Arduino connected)")
	} else {
		utils.Logf("WARNING: Command queue started but Arduino is NOT connected — commands will fail")
	}

	if rs.cfg != nil && rs.cfg.PathExecution.MaxSpeed > 0 {
		rs.pathExecutor = controller.NewPathExecutor(rs.cfg.PathExecution.MaxSpeed, rs.cfg.PathExecution.TurnSpeed)
		rs.waypointThreshold = rs.cfg.PathExecution.WaypointThreshold
		if rs.cfg.PathExecution.SpinThresholdDeg > 0 {
			rs.pathExecutor.SpinThresholdDeg = rs.cfg.PathExecution.SpinThresholdDeg
		}
		if rs.cfg.PathExecution.BurstFrames > 0 {
			rs.pathExecutor.BurstFrames = rs.cfg.PathExecution.BurstFrames
		}
		if rs.cfg.PathExecution.MaxWaitFrames > 0 {
			rs.pathExecutor.MaxWaitFrames = rs.cfg.PathExecution.MaxWaitFrames
		}
		if rs.cfg.PathExecution.ForwardThresholdDeg > 0 {
			rs.pathExecutor.ForwardThresholdDeg = rs.cfg.PathExecution.ForwardThresholdDeg
		}
	} else {
		rs.pathExecutor = controller.NewPathExecutor(0.15, 0.5)
		rs.waypointThreshold = 0.1
	}
	rs.trackingLostTimeout = 3 * time.Second
	if rs.cfg != nil && rs.cfg.PathExecution.TrackingLostTimeoutS > 0 {
		rs.trackingLostTimeout = time.Duration(rs.cfg.PathExecution.TrackingLostTimeoutS * float64(time.Second))
	}
	utils.Logf("Path executor: speed=%.3f turn=%.3f waypoint=%.3f tracking_timeout=%.1fs",
		rs.pathExecutor.MaxSpeed(), rs.pathExecutor.TurnSpeed(), rs.waypointThreshold,
		rs.trackingLostTimeout.Seconds())
	utils.Logf("Path executor: spin=%.1f° burst=%d wait=%d forward=%.1f°",
		rs.pathExecutor.SpinThresholdDeg, rs.pathExecutor.BurstFrames, rs.pathExecutor.MaxWaitFrames,
		rs.pathExecutor.ForwardThresholdDeg)

	rs.webServer = ui.NewWebServer(":9086")

	rs.webServer.OnObstaclesChanged = func(obstacles []planning.Obstacle) {
		utils.Debugf("DEBUG: Initialize() OnObstaclesChanged callback triggered with %d obstacles", len(obstacles))
		rs.planner.SetObstacles(obstacles)

		detectionObstacles := make([]detection.Obstacle, len(obstacles))
		for i, obs := range obstacles {
			utils.Debugf("DEBUG: Initialize() converting obstacle '%s': pixels [%d,%d] to [%d,%d]",
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
		utils.Debugf("DEBUG: Initialize() SetObstacles called with %d detection obstacles", len(detectionObstacles))
	}

	rs.webServer.OnDestinationSet = func(robotID int, pixelPos [2]float64) {
		if rs.positionEst == nil || !rs.positionEst.IsCalibrated() {
			utils.Logf("Cannot set destination: not calibrated")
			return
		}
		worldPos := rs.positionEst.PixelToWorld(int(pixelPos[0]), int(pixelPos[1]))
		utils.Debugf("DEST: pixel(%d,%d) -> world(%.2f,%.2f) BEFORE SetGoal",
			int(pixelPos[0]), int(pixelPos[1]), worldPos.X, worldPos.Y)
		rs.planner.SetGoal(robotID, [2]float64{worldPos.X, worldPos.Y})
		utils.Logf("Destination set for robot %d: pixel(%d,%d) -> world(%.2f,%.2f)",
			robotID, int(pixelPos[0]), int(pixelPos[1]), worldPos.X, worldPos.Y)
	}

	rs.webServer.OnDestinationClear = func(robotID int) {
		utils.Logf("Destination cleared for robot %d", robotID)
		rs.planner.CompletePath(robotID)
		if rs.commandQueue != nil {
			rs.commandQueue.Enqueue(controller.CommandStop)
		}
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
			utils.Debugf("  Robot %d: %d waypoints", rid, len(path))
			if len(path) > 0 {
				utils.Debugf("    First: (%.2f, %.2f), Last: (%.2f, %.2f)",
					path[0][0], path[0][1], path[len(path)-1][0], path[len(path)-1][1])
			}
		}
		return paths
	}

	rs.webServer.OnCommand = func(cmdStr string) error {
		if rs.IsEmergencyStopped() {
			return fmt.Errorf("emergency stop is active")
		}
		if rs.GetControlMode() != ControlModeManual {
			return fmt.Errorf("not in manual mode (current: %s)", rs.GetControlMode())
		}
		var cmd controller.Command
		switch cmdStr {
		case "F":
			cmd = controller.CommandForward
		case "B":
			cmd = controller.CommandBackward
		case "L":
			cmd = controller.CommandLeft
		case "R":
			cmd = controller.CommandRight
		case "S":
			cmd = controller.CommandStop
		default:
			return fmt.Errorf("unknown command: %s", cmdStr)
		}
		if rs.commandQueue != nil {
			rs.commandQueue.Enqueue(cmd)
		}
		return nil
	}

	rs.webServer.OnModeChange = func(mode string) error {
		if rs.IsEmergencyStopped() {
			return fmt.Errorf("cannot change mode while emergency stop is active")
		}
		rs.SetControlMode(ParseControlMode(mode))
		return nil
	}

	rs.webServer.OnEmergencyStop = func() {
		rs.EmergencyStop()
	}

	rs.webServer.OnClearEmergencyStop = func() error {
		rs.ClearEmergencyStop()
		return nil
	}

	rs.webServer.OnGetControlState = func() (string, bool) {
		return rs.GetControlMode().String(), rs.IsEmergencyStopped()
	}

	if name := rs.cameraDisplayName(); name != "" {
		rs.webServer.SetCameraName(name)
	}
	if rs.positionEst != nil && rs.positionEst.IsCalibrated() {
		rs.webServer.SetPositionEstimator(rs.positionEst)
		utils.Debugf("PATH VIS: PositionEstimator set on WebServer (calibrated=%v)",
			rs.positionEst.IsCalibrated())
		rs.webServer.SetCalibrationState("calibrated", "Calibration loaded", calibrationPath, 0.15)
		utils.Logf("Calibration loaded from %s", calibrationPath)
	} else {
		utils.Debugf("PATH VIS: WARNING - PositionEstimator NOT set! IsCalibrated()=%v",
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

// tryOpenCamera retries camera initialization with exponential backoff.
// This handles the macOS case where TCC permission is granted after the process starts
// (e.g., the user clicks "Allow" on the camera permission dialog).
func (rs *RobotSystem) tryOpenCamera(maxDuration time.Duration) error {
	if rs.cameraConfig == nil {
		return fmt.Errorf("no camera configuration available")
	}
	backoff := 2 * time.Second
	maxBackoff := 30 * time.Second
	deadline := time.Now().Add(maxDuration)

	for {
		cam, err := camera.NewCamera(*rs.cameraConfig)
		if err == nil {
			rs.cam = cam
			utils.Logf("Camera initialized: %s", rs.cam.GetName())
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("camera not available after %v: %w", maxDuration, err)
		}

		utils.Logf("Camera not available, retrying in %v (waiting for permission?)...", backoff)
		time.Sleep(backoff)
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
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
			Corners:    f.Corners,
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
	frameStart := time.Now()
	timestamp := float64(frameStart.UnixNano()) / 1e9

	// Compute smoothed FPS via exponential moving average
	if !rs.lastFrameTime.IsZero() {
		dt := frameStart.Sub(rs.lastFrameTime).Seconds()
		if dt > 0 {
			instantFPS := 1.0 / dt
			alpha := 0.1 // EMA smoothing factor
			if rs.smoothedFPS == 0 {
				rs.smoothedFPS = instantFPS
			} else {
				rs.smoothedFPS = alpha*instantFPS + (1-alpha)*rs.smoothedFPS
			}
		}
	}
	rs.lastFrameTime = frameStart

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	rgbaImg, ok := img.(*image.RGBA)
	if rgbaImg == nil {
		rgbaImg = image.NewRGBA(bounds)
		draw.Draw(rgbaImg, bounds, img, bounds.Min, draw.Src)
	}
	_ = ok

	if rs.positionEst != nil {
		rs.positionEst.SetFrameSize(width, height)
	}

	detectionResult := rs.detectionPipe.Detect(frameData, width, height, timestamp, rs.frameNum)
	detectTime := time.Since(frameStart)

	nonRobotYOLO := nonRobotYOLODetections(detectionResult)
	nonRobotYOLO = excludeYOLONearKnownRobots(nonRobotYOLO, rs.positionEst, rs.planner.GetAllRobotStates())

	relevantClasses := classesToMap(rs.cfg.LocalPlanning.ObstacleClasses)
	minConfidence := rs.cfg.LocalPlanning.MinConfidence
	rs.DynamicObstacles = detection.YOLODetectionsToDynamicObstacles(
		nonRobotYOLO,
		rs.positionEst,
		relevantClasses,
		minConfidence,
	)

	// Feed YOLO-detected obstacles into the A* global planner
	if rs.positionEst != nil && rs.positionEst.IsCalibrated() {
		var plannerObstacles []planning.Obstacle
		for _, det := range nonRobotYOLO {
			if det.Bbox == nil {
				continue
			}
			tl := rs.positionEst.PixelToWorld(det.Bbox.X1, det.Bbox.Y1)
			br := rs.positionEst.PixelToWorld(det.Bbox.X2, det.Bbox.Y2)
			// Normalize so TopLeft has smaller coords and BottomRight has larger
			minX, maxX := math.Min(tl.X, br.X), math.Max(tl.X, br.X)
			minY, maxY := math.Min(tl.Y, br.Y), math.Max(tl.Y, br.Y)
			plannerObstacles = append(plannerObstacles, planning.Obstacle{
				Name:             det.ClassName,
				WorldTopLeft:     [2]float64{minX, minY},
				WorldBottomRight: [2]float64{maxX, maxY},
			})
		}
		rs.planner.SetDynamicObstacles(plannerObstacles)
	}

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
				robotDiameter := 0.30 // fallback
				if robotConfig := rs.cfg.GetRobotByTagID(*track.TagID); robotConfig != nil {
					// Compute pixel radius by projecting world-space footprint through homography
					worldRadius := robotConfig.Diameter / 2
					edgePx, edgePy := rs.positionEst.WorldToPixel(position.Point2D{
						X: worldPos.X + worldRadius, Y: worldPos.Y,
					})
					dxR := float64(edgePx - px)
					dyR := float64(edgePy - py)
					track.PixelRadius = math.Sqrt(dxR*dxR + dyR*dyR)
					robotDiameter = robotConfig.Diameter
					// Apply center offset if configured (compensates for tag-to-robot-center distance / parallax)
					if robotConfig.CenterOffsetX != 0 || robotConfig.CenterOffsetY != 0 {
						if heading, ok := rs.lastHeading[*track.TagID]; ok {
							cosH := math.Cos(heading)
							sinH := math.Sin(heading)
							worldPos.X += robotConfig.CenterOffsetX*cosH - robotConfig.CenterOffsetY*sinH
							worldPos.Y += robotConfig.CenterOffsetX*sinH + robotConfig.CenterOffsetY*cosH
						}
					}
				}
				track.WorldPos = [2]float64{worldPos.X, worldPos.Y}
				rs.planner.AddRobot(*track.TagID, [2]float64{worldPos.X, worldPos.Y}, robotDiameter)

				rs.computeTrackHeading(track, detectionResult.Tags)
				utils.Debugf("TRACKPOS: robot=%d pos=(%.3f,%.3f) heading=%.1f° tagSeen=%v cmd=%s",
					*track.TagID, worldPos.X, worldPos.Y, track.Heading*180/math.Pi,
					rs.headingLostCount[*track.TagID] == 0, rs.robotCommands[*track.TagID])
			}
		}
	}

	rs.webServer.BroadcastTracks(trackingResult.Tracks, rs.cfg.Robots, rs.robotCommands)

	rs.executeAutonomousControl(trackingResult.Tracks)

	if rs.webServer.CalibrationViewActive() {
		// Calibration wizard open: send the clean camera view, no detection overlay.
		if img != nil {
			rs.webServer.PushFrame(img)
		}
	} else if overlay := rs.detectionPipe.DrawResults(frameData, width, height, detectionResult); len(overlay) > 0 && len(overlay) < width*height*3 {
		rs.webServer.PushRawJPEG(overlay)
	} else if img != nil {
		rs.webServer.PushFrame(img)
	}

	rs.webServer.UpdateStats(len(detectionResult.Tags), len(detectionResult.YOLODetections))

	// Broadcast Arduino status via WebSocket every ~1 second (30 frames)
	if rs.frameNum%30 == 0 {
		rs.webServer.SetArduinoConnected(rs.arduino != nil && rs.arduino.IsConnected())
		rs.webServer.BroadcastStatus(len(trackingResult.Tracks), rs.smoothedFPS, time.Since(rs.startTime).Seconds())
	}

	detectedTags := make([]ui.DetectedTagInfo, 0, len(detectionResult.Tags))
	for _, tag := range detectionResult.Tags {
		detectedTags = append(detectedTags, ui.DetectedTagInfo{
			ID:      tag.TagID,
			Center:  [2]float64{tag.CenterX, tag.CenterY},
			Corners: tag.Corners,
		})
	}
	rs.webServer.UpdateDetectedTags(detectedTags, width, height)

	if rs.frameNum%10 == 0 && rs.webServer != nil {
		paths := rs.planner.GetPaths()
		totalWaypoints := 0
		for _, path := range paths {
			totalWaypoints += len(path)
		}
		rs.webServer.BroadcastPaths()
	}

	if rs.frameNum%30 == 0 {
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
		wBot = rs.positionEst.PixelToWorldFloat(botMidX, botMidY)
		wTop = rs.positionEst.PixelToWorldFloat(topMidX, topMidY)
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
		if cached, ok := rs.lastHeading[tagID]; ok {
			track.Heading = cached
			utils.Debugf("HEADING: tag %d using cached heading=%.2f°", tagID, cached*180/math.Pi)
		}
		rs.headingDelta[tagID] = 0
		rs.headingLostCount[tagID]++
		if rs.headingLostCount[tagID] > 5 {
			delete(rs.smoothedHeading, tagID)
			delete(rs.headingRejectCount, tagID)
		}
	} else {
		rs.headingLostCount[tagID] = 0
		if prev, ok := rs.lastHeading[tagID]; ok {
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
			rs.headingDelta[tagID] = delta
		} else {
			rs.headingDelta[tagID] = 0
		}
		rs.lastHeading[tagID] = track.Heading
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
	prev, ok := rs.smoothedHeading[tagID]
	if !ok {
		rs.smoothedHeading[tagID] = track.Heading
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
		rs.headingRejectCount[tagID]++
		utils.Debugf("HEADING REJECT: tag %d raw=%.1f° smoothed=%.1f° diff=%.1f° count=%d",
			tagID, track.Heading*180/math.Pi, prev*180/math.Pi, diff*180/math.Pi, rs.headingRejectCount[tagID])
		if rs.headingRejectCount[tagID] >= 10 {
			// Too many consecutive rejections — accept with EMA to converge
			track.Heading = normalizeAngle(prev + alpha*diff)
			utils.Debugf("HEADING RESET: tag %d after 10 rejections, converging to %.1f°",
				tagID, track.Heading*180/math.Pi)
			rs.headingRejectCount[tagID] = 0
		} else {
			track.Heading = prev // keep previous smoothed heading
		}
	} else {
		// Reasonable change — apply EMA
		track.Heading = normalizeAngle(prev + alpha*diff)
		rs.headingRejectCount[tagID] = 0
	}
	rs.smoothedHeading[tagID] = track.Heading
}

// executeAutonomousControl handles path-following for all tracked robots.
func (rs *RobotSystem) executeAutonomousControl(tracks []tracking.Track) {
	if rs.GetControlMode() != ControlModeAutonomous || rs.IsEmergencyStopped() {
		return
	}

	commandIssued := false
	for i := range tracks {
		track := &tracks[i]
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

			// Stop and replan when dangerously close to an obstacle
			if clearance := rs.planner.GetClearance(robotID); clearance < 0.08 {
				utils.Logf("PROXIMITY WARNING: Robot %d clearance=%.3fm — stopping and replanning", robotID, clearance)
				if rs.commandQueue != nil {
					rs.commandQueue.Enqueue(controller.CommandStop)
					commandIssued = true
				}
				rs.robotCommands[robotID] = "stopped"
				// Replan at most once every 3 seconds to avoid thrashing
				if lastReplan, ok := rs.lastReplanTime[robotID]; !ok || time.Since(lastReplan) > 3*time.Second {
					if goal, hasGoal := rs.planner.GetGoal(robotID); hasGoal {
						rs.planner.ClearPathOnly(robotID)
						pos := [2]float64{worldPos.X, worldPos.Y}
						if newPath, ok := rs.planner.PlanPath(robotID, pos, goal); ok {
							rs.planner.SetPath(robotID, newPath)
							utils.Logf("Robot %d replanned: %d waypoints from (%.2f,%.2f)", robotID, len(newPath), pos[0], pos[1])
						} else {
							utils.Logf("Robot %d replan FAILED from (%.2f,%.2f) to (%.2f,%.2f)", robotID, pos[0], pos[1], goal[0], goal[1])
						}
						rs.lastReplanTime[robotID] = time.Now()
					}
				}
				continue
			}

			// Check if robot is close to final destination
			if goal, hasGoal := rs.planner.GetGoal(robotID); hasGoal {
				dx := worldPos.X - goal[0]
				dy := worldPos.Y - goal[1]
				distToGoal := math.Sqrt(dx*dx + dy*dy)
				if distToGoal < rs.waypointThreshold {
					rs.planner.CompletePath(robotID)
					rs.webServer.ClearDestination(robotID)
					utils.Logf("Robot %d reached goal (%.2fm away), stopping", robotID, distToGoal)
					if rs.commandQueue != nil {
						rs.commandQueue.Enqueue(controller.CommandStop)
						commandIssued = true
					}
					rs.robotCommands[robotID] = "stopped"
					continue
				}
			}

			// Advance past any reached or overshot waypoints
			if !rs.planner.AdvancePastWaypoints(robotID, [2]float64{worldPos.X, worldPos.Y}, rs.waypointThreshold) {
				utils.Logf("Robot %d reached final waypoint, stopping", robotID)
				if rs.commandQueue != nil {
					rs.commandQueue.Enqueue(controller.CommandStop)
					commandIssued = true
				}
				rs.robotCommands[robotID] = "stopped"
				continue
			}

			// Get updated waypoint after advancing
			waypoint, stillHasPath := rs.planner.GetNextWaypoint(robotID)
			if !stillHasPath {
				continue
			}

			// Heading-based steering: turn to face waypoint, then drive forward
			dx := waypoint[0] - worldPos.X
			dy := waypoint[1] - worldPos.Y
			bearingToWaypoint := math.Atan2(dy, dx)

			if rs.pathExecutor != nil && rs.commandQueue != nil {
				delta := rs.headingDelta[robotID]
				cmd := rs.pathExecutor.BearingToCommand(track.Heading, bearingToWaypoint, delta)
				rs.commandQueue.Enqueue(cmd)
				commandIssued = true
				switch cmd {
				case controller.CommandForward:
					rs.robotCommands[robotID] = "forward"
				case controller.CommandBackward:
					rs.robotCommands[robotID] = "backward"
				case controller.CommandLeft:
					rs.robotCommands[robotID] = "rotating_left"
				case controller.CommandRight:
					rs.robotCommands[robotID] = "rotating_right"
				case controller.CommandStop:
					rs.robotCommands[robotID] = "stopped"
				}
			}

			rs.planner.UpdateRobotState(robotID, [2]float64{worldPos.X, worldPos.Y},
				[2]float64{0, 0})
		} else if rs.commandQueue != nil && rs.commandQueue.IsRunning() {
			rs.commandQueue.ClearActiveCommand()
		}
	}

	// Safety: stop re-sending stale commands when tracking is lost for too long.
	if commandIssued {
		rs.lastCommandTime = time.Now()
	} else if rs.commandQueue != nil && time.Since(rs.lastCommandTime) > rs.trackingLostTimeout {
		rs.commandQueue.ClearActiveCommand()
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
	frameStart := time.Now()
	timestamp := float64(frameStart.UnixNano()) / 1e9

	// Compute smoothed FPS via exponential moving average
	if !rs.lastFrameTime.IsZero() {
		dt := frameStart.Sub(rs.lastFrameTime).Seconds()
		if dt > 0 {
			instantFPS := 1.0 / dt
			alpha := 0.1
			if rs.smoothedFPS == 0 {
				rs.smoothedFPS = instantFPS
			} else {
				rs.smoothedFPS = alpha*instantFPS + (1-alpha)*rs.smoothedFPS
			}
		}
	}
	rs.lastFrameTime = frameStart

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
						worldRadius := robotConfig.Diameter / 2
						edgePx, edgePy := rs.positionEst.WorldToPixel(position.Point2D{
							X: worldPos.X + worldRadius, Y: worldPos.Y,
						})
						dxR := float64(edgePx - px)
						dyR := float64(edgePy - py)
						track.PixelRadius = math.Sqrt(dxR*dxR + dyR*dyR)
					}
				}
			}
		}

		var robots []config.RobotConfig
		if rs.cfg != nil {
			robots = rs.cfg.Robots
		}
		rs.webServer.BroadcastTracks(trackingResult.Tracks, robots, rs.robotCommands)
	}

	if rs.detectionPipe != nil && rs.webServer != nil && rs.webServer.CalibrationViewActive() {
		rs.webServer.PushFrame(img)
	} else if rs.detectionPipe != nil && rs.webServer != nil {
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

		// Broadcast Arduino status via WebSocket every ~1 second (30 frames)
		if rs.frameNum%30 == 0 {
			rs.webServer.SetArduinoConnected(rs.arduino != nil && rs.arduino.IsConnected())
			rs.webServer.BroadcastStatus(len(demoTags), rs.smoothedFPS, time.Since(rs.startTime).Seconds())
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
	}
}

//gocyclo:ignore
func main() {
	configPath := flag.String("config", "config/tracking_config.yaml", "Path to configuration file")
	listPorts := flag.Bool("list-ports", false, "List available serial ports")
	listCamerasFlag := flag.Bool("list-cameras", false, "List available cameras")
	testCameraID := flag.Int("test-camera", -1, "Test specific camera by ID")
	flag.String("web-port", ":9086", "Web server port")
	demoMode := flag.Bool("demo", false, "Run demo mode with test pattern")
	selfTestMode := flag.Bool("self-test", false, "Run self-test for dynamic obstacle pipeline")
	demoYOLOMode := flag.Bool("demo-yolo", false, "Run demo mode with YOLO obstacles visualization")
	quiet := flag.Bool("quiet", false, "Suppress all logging output")
	logFile := flag.String("log-file", "", "Log to file with rotation (default: log to stderr)")
	flag.Parse()

	if *logFile != "" {
		f, err := utils.SetupLogFile(*logFile, 3)
		if err != nil {
			log.Fatalf("Failed to set up log file: %v", err)
		}
		defer func() { _ = f.Close() }()
	}

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

	if *listCamerasFlag {
		listCameras()
		return
	}

	if *testCameraID >= 0 {
		testCamera(*testCameraID)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		utils.Logf("Warning: Could not load config: %v", err)
		utils.Logf("Running with demo mode only")
		*demoMode = true
	}

	rs := NewRobotSystem(cfg)
	rs.demoMode = *demoMode
	if cfg != nil {
		if err := rs.Initialize(); err != nil {
			utils.Logf("Warning: Failed to initialize robot system: %v", err)
		}
	} else {
		rs.initDemoMode()
	}
	defer rs.Stop()

	utils.Log("Press Ctrl+C to exit.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		utils.Log("\nShutting down...")
		rs.Stop()
		os.Exit(0)
	}()

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

	// If camera isn't available yet (e.g., macOS permission dialog pending),
	// retry with backoff before falling back to demo mode.
	if rs.cam == nil && !*demoMode && rs.cameraConfig != nil {
		utils.Log("Camera not available at startup, retrying (waiting for permission?)...")
		if err := rs.tryOpenCamera(2 * time.Minute); err != nil {
			utils.Logf("Camera unavailable after retries: %v, falling back to demo mode", err)
			*demoMode = true
		}
	}
	// If camera still isn't available and not in demo mode, fall back
	if rs.cam == nil && !*demoMode {
		utils.Log("No camera available, falling back to demo mode")
		*demoMode = true
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

	if *demoMode {
		// Also covers falling back to demo because the camera never opened:
		// calibrating on synthetic frames must not overwrite the real file.
		rs.demoMode = true
		if rs.webServer != nil {
			rs.webServer.SetCameraName(demoCameraName)
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

}
