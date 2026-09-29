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
	"runtime"
	"strings"
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
	ControlModeIdle ControlMode = iota
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

// RobotSystem owns and orchestrates every subsystem in the tracking/control
// pipeline. Its fields are grouped into named sub-structs (declared in
// robot_system_types.go) by the concern that owns them:
//
//   - capture   — camera capture
//   - detection — AprilTag + foreground/obstacle detection
//   - tracking  — multi-object tracker
//   - planning  — path planner, current goal
//   - obstacles — static/dynamic obstacle lists
//   - position  — pixel<->world calibration
//   - io        — Arduino serial, command queue, path executor
//   - web       — HTTP/WebSocket/MJPEG server
//   - control   — operator control mode + e-stop
//   - heading   — per-robot heading smoothing
//   - stats     — frame timing, watchdog, perf window
//
// Initialize() sets each group up via its own init<Group>() method, in the
// order listed above (see Initialize() for the exact call sequence).
type RobotSystem struct {
	demoMode bool
	cfg      *config.Config

	capture   cameraSubsystem
	detection detectionSubsystem
	tracking  trackingSubsystem
	planning  planningSubsystem
	obstacles obstaclesSubsystem
	position  positionSubsystem
	io        controlIOSubsystem
	web       webSubsystem
	control   controlStateSubsystem
	heading   headingSubsystem
	stats     frameTimingSubsystem
}

func NewRobotSystem(cfg *config.Config) *RobotSystem {
	return &RobotSystem{
		cfg: cfg,
		planning: planningSubsystem{
			lastReplanTime: make(map[int]time.Time),
		},
		io: controlIOSubsystem{
			robotCommands: make(map[int]string),
		},
		heading: headingSubsystem{
			lastHeading:        make(map[int]float64),
			headingDelta:       make(map[int]float64),
			smoothedHeading:    make(map[int]float64),
			headingRejectCount: make(map[int]int),
			headingLostCount:   make(map[int]int),
		},
		stats: frameTimingSubsystem{startTime: time.Now()},
	}
}

func (rs *RobotSystem) GetControlMode() ControlMode {
	rs.control.controlMu.RLock()
	defer rs.control.controlMu.RUnlock()
	return rs.control.controlMode
}

func (rs *RobotSystem) SetControlMode(mode ControlMode) {
	rs.control.controlMu.Lock()
	defer rs.control.controlMu.Unlock()
	if rs.control.emergencyStopped {
		return
	}
	prev := rs.control.controlMode
	rs.control.controlMode = mode
	utils.Logf("Control mode changed to: %s", mode)

	// When leaving Manual or Autonomous, clear active command and stop the robot
	if prev != mode && (prev == ControlModeManual || prev == ControlModeAutonomous) {
		if rs.io.commandQueue != nil {
			rs.io.commandQueue.ClearActiveCommand()
			rs.io.commandQueue.Enqueue(controller.CommandStop)
		}
	}
}

func (rs *RobotSystem) IsEmergencyStopped() bool {
	rs.control.controlMu.RLock()
	defer rs.control.controlMu.RUnlock()
	return rs.control.emergencyStopped
}

func (rs *RobotSystem) EmergencyStop() {
	rs.control.controlMu.Lock()
	rs.control.emergencyStopped = true
	rs.control.controlMode = ControlModeIdle
	rs.control.controlMu.Unlock()

	// Send stop directly to Arduino, bypassing queue for reliability
	if rs.io.arduino != nil {
		_ = rs.io.arduino.SendCommand(controller.CommandStop)
	}
	if rs.io.commandQueue != nil {
		rs.io.commandQueue.EmergencyStop()
	}
	utils.Logf("EMERGENCY STOP activated")
}

func (rs *RobotSystem) ClearEmergencyStop() {
	rs.control.controlMu.Lock()
	rs.control.emergencyStopped = false
	rs.control.controlMu.Unlock()

	// Restart the command queue so it can accept commands again
	if rs.io.commandQueue != nil {
		rs.io.commandQueue.Start()
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

// cameraDisplayName names the configured camera even before it has opened
// (e.g. while macOS is still waiting on camera permission), so the calibration
// file used for loading and saving never depends on camera start-up timing.
func (rs *RobotSystem) cameraDisplayName() string {
	if rs.demoMode {
		return demoCameraName
	}
	if rs.capture.cam != nil {
		return rs.capture.cam.GetName()
	}
	if rs.capture.cameraConfig != nil {
		return camera.DisplayName(rs.capture.cameraConfig.URL, rs.capture.cameraConfig.CameraID)
	}
	return ""
}

// statusBroadcastDue reports whether a status update (FPS, Arduino state) should
// go to the UI now. It is time-based rather than every N frames so the FPS
// readout keeps refreshing about once a second even when the frame rate is
// very low, which is exactly when the UI needs to warn about it.
func (rs *RobotSystem) statusBroadcastDue() bool {
	if time.Since(rs.stats.lastStatusBroadcast) < time.Second {
		return false
	}
	rs.stats.lastStatusBroadcast = time.Now()
	return true
}

// Initialize wires up every subsystem in a fixed order and never returns a
// non-nil error today (every failure path logs a warning and continues in a
// degraded state instead) -- each init<Group>() helper below preserves that
// contract; none of them should be given error-propagating return values
// without also updating every caller, since callers currently rely on
// Initialize() always succeeding.
func (rs *RobotSystem) Initialize() error {
	rs.initDetectionPipeline()
	rs.initTracker()
	rs.initPlanner()
	rs.initCamera()

	// Calibration files are named after the camera (config/calibration_<name>.yaml,
	// see ui.GetCalibrationFilename); the "default" name below is only reached
	// when no camera name is available at all (no camera opened and no camera
	// config), which happens with an empty/misconfigured cameras list. There is
	// deliberately no config/calibration_default.yaml on disk -- initPositionEstimator
	// skips loading a calibration file that doesn't exist, so this just starts
	// uncalibrated rather than fail.
	calibrationPath := ui.GetCalibrationFilename("default")
	if name := rs.cameraDisplayName(); name != "" {
		calibrationPath = ui.GetCalibrationFilename(name)
	}
	utils.Logf("Using calibration file: %s", calibrationPath)
	rs.initPositionEstimator(calibrationPath)
	rs.initArduinoAndQueue()
	rs.initPathExecutor()

	rs.web.webServer = ui.NewWebServer(":9086")
	// Wire the configured obstacles-file override through to the web UI's
	// save/clear handlers so they honor the same override that loading does
	// (see ui.ResolveObstaclesPath); previously GetObstaclesPath() never
	// consulted cfg.Obstacles at all.
	rs.web.webServer.SetObstaclesPath(rs.cfg.Obstacles.GetPath())
	rs.registerWebServerCallbacks()
	rs.applyCalibrationStateToWebServer(calibrationPath)
	rs.initForeground()

	rs.web.webServer.Start()
	utils.Log(getWebUIURLs("9086"))

	rs.loadStaticObstacles()

	return nil
}

// initDetectionPipeline sets up AprilTag detection.
func (rs *RobotSystem) initDetectionPipeline() {
	tagConfig := detection.AprilTagConfig{
		Family:       rs.cfg.AprilTags.Family,
		QuadDecimate: rs.cfg.AprilTags.QuadDecimate,
	}

	rs.detection.detectionPipe = detection.NewDetectionPipeline(tagConfig)
	utils.Logf("Detection pipeline initialized")
}

// initTracker sets up the multi-object tracker.
func (rs *RobotSystem) initTracker() {
	trackConfig := &tracking.ByteTrackConfig{
		TrackThresh: rs.cfg.Tracking.TrackThresh,
		TrackBuffer: rs.cfg.Tracking.TrackBuffer,
		MatchThresh: rs.cfg.Tracking.MatchThresh,
		FrameRate:   rs.cfg.Tracking.FrameRate,
		MinBoxArea:  rs.cfg.Tracking.MinBoxArea,
		MOT20:       rs.cfg.Tracking.MOT20,
	}
	rs.tracking.tracker = tracking.NewByteTrack(trackConfig)
	utils.Logf("ByteTrack initialized")
}

// initPlanner sets up the path planner.
func (rs *RobotSystem) initPlanner() {
	plannerConfig := &planning.PlannerConfig{
		AStarConfig: nil,
	}
	rs.planning.planner = planning.NewPlanner(plannerConfig)
	utils.Logf("Planner initialized")
}

// initCamera opens the configured primary camera, if any. A failure here is
// non-fatal: rs.capture.cam is left nil and the caller falls back to demo mode
// or retries later via tryOpenCamera/StartCamera.
func (rs *RobotSystem) initCamera() {
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
		rs.capture.cameraConfig = &camConfig
		cam, err := camera.NewCamera(camConfig)
		if err != nil {
			utils.Logf("Warning: Could not initialize camera: %v (will retry)", err)
			rs.capture.cam = nil
		} else {
			rs.capture.cam = cam
			utils.Logf("Camera initialized: %s", rs.capture.cam.GetName())
		}
	} else {
		utils.Logf("No camera configured, using demo mode")
	}
}

// initPositionEstimator sets up pixel<->world calibration from calibrationPath.
func (rs *RobotSystem) initPositionEstimator(calibrationPath string) {
	posEst, err := position.NewPositionEstimator(calibrationPath)
	if err != nil {
		utils.Logf("Warning: Position estimator initialization failed: %v", err)
		rs.position.positionEst = nil
	} else {
		rs.position.positionEst = posEst
		utils.Logf("Position estimator initialized")
	}
}

// initArduinoAndQueue connects to the configured Arduino (if the controller
// is enabled) and starts the command queue regardless of whether that
// connection succeeded, matching the previous inline behavior.
func (rs *RobotSystem) initArduinoAndQueue() {
	serialPort := "auto"
	serialBaud := controller.BaudRate
	commandIntervalMs := controller.CommandIntervalMs
	heartbeatTimeoutMs := controller.HeartbeatTimeoutMs
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
	if rs.cfg.Controller.HeartbeatTimeout > 0 {
		heartbeatTimeoutMs = int(rs.cfg.Controller.HeartbeatTimeout * 1000)
	}

	rs.io.arduino = controller.NewArduinoController(serialPort, serialBaud)
	if controllerEnabled {
		if err := rs.io.arduino.Connect(); err != nil {
			utils.Logf("Warning: Could not connect to Arduino: %v", err)
		} else {
			utils.Logf("Connected to Arduino on %s at %d baud", rs.io.arduino.GetPort(), serialBaud)
		}
	} else {
		utils.Logf("Controller disabled in config, skipping Arduino connection")
	}

	rs.io.commandQueue = controller.NewCommandQueue(rs.io.arduino, commandIntervalMs, heartbeatTimeoutMs)
	rs.io.commandQueue.Start()
	if rs.io.arduino.IsConnected() {
		utils.Logf("Command queue started (Arduino connected)")
	} else {
		utils.Logf("WARNING: Command queue started but Arduino is NOT connected — commands will fail")
	}
}

// initPathExecutor sets up waypoint-following parameters from config,
// falling back to hardcoded defaults if PathExecution isn't configured.
func (rs *RobotSystem) initPathExecutor() {
	if rs.cfg != nil && rs.cfg.PathExecution.MaxSpeed > 0 {
		rs.io.pathExecutor = controller.NewPathExecutor(rs.cfg.PathExecution.MaxSpeed, rs.cfg.PathExecution.TurnSpeed)
		rs.io.waypointThreshold = rs.cfg.PathExecution.WaypointThreshold
		if rs.cfg.PathExecution.SpinThresholdDeg > 0 {
			rs.io.pathExecutor.SpinThresholdDeg = rs.cfg.PathExecution.SpinThresholdDeg
		}
		if rs.cfg.PathExecution.BurstFrames > 0 {
			rs.io.pathExecutor.BurstFrames = rs.cfg.PathExecution.BurstFrames
		}
		if rs.cfg.PathExecution.MaxWaitFrames > 0 {
			rs.io.pathExecutor.MaxWaitFrames = rs.cfg.PathExecution.MaxWaitFrames
		}
		if rs.cfg.PathExecution.ForwardThresholdDeg > 0 {
			rs.io.pathExecutor.ForwardThresholdDeg = rs.cfg.PathExecution.ForwardThresholdDeg
		}
	} else {
		rs.io.pathExecutor = controller.NewPathExecutor(0.15, 0.5)
		rs.io.waypointThreshold = 0.1
	}
	rs.io.trackingLostTimeout = 3 * time.Second
	if rs.cfg != nil && rs.cfg.PathExecution.TrackingLostTimeoutS > 0 {
		rs.io.trackingLostTimeout = time.Duration(rs.cfg.PathExecution.TrackingLostTimeoutS * float64(time.Second))
	}
	utils.Logf("Path executor: speed=%.3f turn=%.3f waypoint=%.3f tracking_timeout=%.1fs",
		rs.io.pathExecutor.MaxSpeed(), rs.io.pathExecutor.TurnSpeed(), rs.io.waypointThreshold,
		rs.io.trackingLostTimeout.Seconds())
	utils.Logf("Path executor: spin=%.1f° burst=%d wait=%d forward=%.1f°",
		rs.io.pathExecutor.SpinThresholdDeg, rs.io.pathExecutor.BurstFrames, rs.io.pathExecutor.MaxWaitFrames,
		rs.io.pathExecutor.ForwardThresholdDeg)
}

// registerWebServerCallbacks wires the operator UI's HTTP/WebSocket handlers
// (in internal/ui) to RobotSystem behavior via the WebServer's On* callback
// fields. Each closure captures rs, so it always sees the RobotSystem's
// current state at call time, not at registration time.
func (rs *RobotSystem) registerWebServerCallbacks() {
	rs.web.webServer.Callbacks.OnObstaclesChanged = func(obstacles []planning.Obstacle) {
		utils.Debugf("DEBUG: Initialize() OnObstaclesChanged callback triggered with %d obstacles", len(obstacles))
		rs.planning.planner.SetObstacles(obstacles)

		detectionObstacles := convertPlanningObstaclesToDetection(obstacles)
		rs.detection.detectionPipe.SetObstacles(detectionObstacles)
		utils.Debugf("DEBUG: Initialize() SetObstacles called with %d detection obstacles", len(detectionObstacles))
	}

	rs.web.webServer.Callbacks.OnDestinationSet = func(robotID int, pixelPos [2]float64) error {
		if rs.cfg == nil || rs.cfg.GetRobotByTagID(robotID) == nil {
			return fmt.Errorf("robot %d is not configured; add it under robots: in config/tracking_config.yaml", robotID)
		}
		if rs.position.positionEst == nil || !rs.position.positionEst.IsCalibrated() {
			utils.Logf("Cannot set destination: not calibrated")
			return fmt.Errorf("cannot set a destination: the camera is not calibrated")
		}
		worldPos := rs.position.positionEst.PixelToWorld(int(pixelPos[0]), int(pixelPos[1]))
		utils.Debugf("DEST: pixel(%d,%d) -> world(%.2f,%.2f) BEFORE SetGoal",
			int(pixelPos[0]), int(pixelPos[1]), worldPos.X, worldPos.Y)
		// One controller line drives one robot: the protocol has no robot
		// address, and there is a single command queue and path executor. So
		// only one robot may have a goal at a time; giving a new robot a
		// goal releases the others. (Multi-robot control will need a
		// controller per robot; see docs.) The UI likewise models a single
		// destination, so the web server's copy is not touched here.
		for _, other := range rs.planning.planner.RobotsWithGoals() {
			if other != robotID {
				utils.Logf("Releasing goal of robot %d: robot %d is now the controlled robot", other, robotID)
				rs.planning.planner.CompletePath(other)
			}
		}
		rs.planning.planner.SetGoal(robotID, [2]float64{worldPos.X, worldPos.Y})
		utils.Logf("Destination set for robot %d: pixel(%d,%d) -> world(%.2f,%.2f)",
			robotID, int(pixelPos[0]), int(pixelPos[1]), worldPos.X, worldPos.Y)
		return nil
	}

	rs.web.webServer.Callbacks.OnDestinationClear = func(robotID int) {
		utils.Logf("Destination cleared for robot %d", robotID)
		rs.planning.planner.CompletePath(robotID)
		if rs.io.commandQueue != nil {
			rs.io.commandQueue.Enqueue(controller.CommandStop)
		}
	}

	rs.web.webServer.Callbacks.OnCalibrationComplete = func(calibFile string) {
		utils.Logf("Calibration complete, reloading from %s", calibFile)
		if rs.position.positionEst != nil {
			if err := rs.position.positionEst.LoadCalibration(calibFile); err != nil {
				utils.Logf("Failed to reload calibration: %v", err)
				return
			}
			rs.web.webServer.SetPositionEstimator(rs.position.positionEst)
			utils.Logf("Calibration reloaded: IsCalibrated=%v", rs.position.positionEst.IsCalibrated())
			if rs.detection.fg != nil {
				// The floor mapping changed: relearn the background and forget obstacles.
				rs.detection.fg.requestReset()
			}
		}
	}

	rs.web.webServer.Callbacks.OnPathsChanged = func() map[int][][2]float64 {
		paths := rs.planning.planner.GetPathsWithGoals()
		for rid, path := range paths {
			utils.Debugf("  Robot %d: %d waypoints", rid, len(path))
			if len(path) > 0 {
				utils.Debugf("    First: (%.2f, %.2f), Last: (%.2f, %.2f)",
					path[0][0], path[0][1], path[len(path)-1][0], path[len(path)-1][1])
			}
		}
		return paths
	}

	rs.web.webServer.Callbacks.OnCommand = func(cmdStr string) error {
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
		if rs.io.commandQueue != nil {
			rs.io.commandQueue.Enqueue(cmd)
		}
		return nil
	}

	rs.web.webServer.Callbacks.OnModeChange = func(mode string) error {
		if rs.IsEmergencyStopped() {
			return fmt.Errorf("cannot change mode while emergency stop is active")
		}
		rs.SetControlMode(ParseControlMode(mode))
		return nil
	}

	rs.web.webServer.Callbacks.OnEmergencyStop = func() {
		rs.EmergencyStop()
	}

	rs.web.webServer.Callbacks.OnClearEmergencyStop = func() error {
		rs.ClearEmergencyStop()
		return nil
	}

	rs.web.webServer.Callbacks.OnGetControlState = func() (string, bool) {
		return rs.GetControlMode().String(), rs.IsEmergencyStopped()
	}
}

// applyCalibrationStateToWebServer pushes the current camera name and
// calibration status to the web server so the operator UI reflects them
// immediately on startup, without waiting for a calibration event.
func (rs *RobotSystem) applyCalibrationStateToWebServer(calibrationPath string) {
	if name := rs.cameraDisplayName(); name != "" {
		rs.web.webServer.SetCameraName(name)
	}
	if rs.position.positionEst != nil && rs.position.positionEst.IsCalibrated() {
		rs.web.webServer.SetPositionEstimator(rs.position.positionEst)
		utils.Debugf("PATH VIS: PositionEstimator set on WebServer (calibrated=%v)",
			rs.position.positionEst.IsCalibrated())
		rs.web.webServer.SetCalibrationState("calibrated", "Calibration loaded", calibrationPath, 0.15)
		utils.Logf("Calibration loaded from %s", calibrationPath)
	} else {
		utils.Debugf("PATH VIS: WARNING - PositionEstimator NOT set! IsCalibrated()=%v",
			rs.position.positionEst != nil && rs.position.positionEst.IsCalibrated())
	}
}

func (rs *RobotSystem) loadStaticObstacles() {
	configuredPath := ""
	if rs.cfg != nil {
		configuredPath = rs.cfg.Obstacles.GetPath()
	}
	obstaclesPath := ui.ResolveObstaclesPath(configuredPath, rs.cameraDisplayName())

	utils.Logf("Loading obstacles from: %s", obstaclesPath)
	// #nosec G304
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

		staticObs := planning.NewRectObstacle(name, worldTL, worldBR)
		staticObs.PixelsTopLeft = pixelsTL
		staticObs.PixelsBottomRight = pixelsBR
		rs.obstacles.StaticObstacles = append(rs.obstacles.StaticObstacles, staticObs)
	}

	if len(rs.obstacles.StaticObstacles) > 0 {
		utils.Logf("Loaded %d static obstacles from %s, calling SetObstacles", len(rs.obstacles.StaticObstacles), obstaclesPath)
		rs.web.webServer.SetObstacles(rs.obstacles.StaticObstacles)
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
	if rs.capture.cameraConfig == nil {
		return fmt.Errorf("no camera configuration available")
	}
	backoff := 2 * time.Second
	maxBackoff := 30 * time.Second
	deadline := time.Now().Add(maxDuration)

	for {
		cam, err := camera.NewCamera(*rs.capture.cameraConfig)
		if err == nil {
			rs.capture.cam = cam
			utils.Logf("Camera initialized: %s", rs.capture.cam.GetName())
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
	if rs.capture.cam == nil {
		utils.Logf("No camera available")
		rs.capture.cameraRunning.Store(false)
		return nil
	}
	if err := rs.capture.cam.Start(); err != nil {
		utils.Logf("Failed to start camera: %v", err)
		rs.capture.cameraRunning.Store(false)
		return err
	}
	rs.capture.cameraRunning.Store(true)
	utils.Logf("Camera started: %s", rs.capture.cam.GetName())
	if !rs.demoMode {
		rs.startFrameWatchdog()
	}
	return nil
}

const (
	// perfLogEveryTicks is how many watchdog ticks (seconds) between PERF lines.
	perfLogEveryTicks = 5
	// lowFPSForOffenders is the frame rate under which a PERF line also names
	// the busiest other processes.
	lowFPSForOffenders = 8.0
	offenderLogEvery   = 30 * time.Second
)

// logPerf writes one PERF line: frame rate and timings over the last window,
// this process's CPU use, machine load, memory, and control state. When the
// frame rate is low it also names the busiest other processes (rate-limited),
// so a slowdown caused by something else on the machine identifies itself.
func (rs *RobotSystem) logPerf() {
	s := rs.stats.perf.TakeSummary(time.Now())

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	line := s.Line() + fmt.Sprintf(" mem=%.0fMB mode=%s", float64(mem.Alloc)/1024/1024, rs.GetControlMode())

	now := time.Now()
	cpu := processCPUSeconds()
	if wall := now.Sub(rs.stats.lastCPUAt).Seconds(); wall > 0 && rs.stats.lastCPUSeconds > 0 {
		line += fmt.Sprintf(" proc_cpu=%.1fcores", (cpu-rs.stats.lastCPUSeconds)/wall)
	}
	rs.stats.lastCPUSeconds, rs.stats.lastCPUAt = cpu, now

	if rs.detection.fg != nil && rs.detection.fg.enabled.Load() {
		line += rs.detection.fg.perfSummary()
	}
	if load, ok := systemLoadAverage(); ok {
		line += fmt.Sprintf(" load1=%.2f", load)
	}
	if s.FPS < lowFPSForOffenders && time.Since(rs.stats.lastOffenderLog) > offenderLogEvery {
		if top := topCPUProcesses(4); len(top) > 0 {
			line += " LOW_FPS busiest_processes=" + strings.Join(top, ",")
			rs.stats.lastOffenderLog = now
		}
	}
	utils.Log(line)
}

// frameStallThreshold is how long without a processed frame counts as a
// stalled camera worth telling the UI about.
const frameStallThreshold = 2 * time.Second

// startFrameWatchdog broadcasts status once a second while no frames are being
// processed. The normal status update rides on the frame loop, so without this
// a dead camera would leave the UI silently showing the last good FPS.
func (rs *RobotSystem) startFrameWatchdog() {
	rs.stats.watchdogOnce.Do(func() {
		rs.stats.watchdogStop = make(chan struct{})
		stop := rs.stats.watchdogStop
		rs.stats.perf.TakeSummary(time.Now()) // start the first window now
		rs.stats.lastCPUAt = time.Now()
		rs.stats.lastCPUSeconds = processCPUSeconds()
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			ticks := 0
			stallHalted := false
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					ticks++
					if ticks%perfLogEveryTicks == 0 {
						rs.logPerf()
					}
					last := rs.stats.startTime
					if n := rs.stats.lastFrameNanos.Load(); n != 0 {
						last = time.Unix(0, n)
					}
					age := time.Since(last)
					if age > frameStallThreshold && rs.web.webServer != nil {
						rs.web.webServer.BroadcastCameraStalled(time.Since(rs.stats.startTime).Seconds(), age.Seconds())
					}
					// Autonomous control only runs from the frame loop, so with no
					// frames nothing would ever stop the robot. Halt it once per
					// stall (not every tick, so it can be driven again by hand).
					if age > frameStallThreshold && !stallHalted {
						stallHalted = true
						utils.Logf("Camera stalled for %.1fs: halting robot", age.Seconds())
						if rs.io.commandQueue != nil {
							rs.io.commandQueue.HaltMotion()
						}
					} else if age <= frameStallThreshold {
						stallHalted = false
					}
				}
			}
		}()
	})
}

// Stop shuts the system down. It is safe to call more than once, and safe to
// call concurrently: main() both defers a call and calls it from the SIGINT/
// SIGTERM handler goroutine, and the handler's os.Exit(0) only coincidentally
// prevents both from running today (see docs/archived/code-review-2026-09-27.md #15).
// stopOnce makes that safety an actual invariant rather than a side effect of
// process-exit timing, so a future refactor (e.g. removing the os.Exit) can't
// reintroduce a double-close panic.
func (rs *RobotSystem) Stop() {
	rs.stats.stopOnce.Do(func() {
		utils.Logf("Stopping system...")
		rs.stats.watchdogStopOnce.Do(func() {
			if rs.stats.watchdogStop != nil {
				close(rs.stats.watchdogStop)
			}
		})
		rs.capture.cameraRunning.Store(false)
		// Wait for any in-flight frame and reject later ones, so the saves and
		// Closes below never race the frame loop's use of the detector.
		rs.capture.frameMu.Lock()
		rs.capture.stopped = true
		rs.capture.frameMu.Unlock()
		if rs.detection.fg != nil {
			// A final, blocking save so a clean shutdown never has to wait for
			// the next periodic save to capture the current background.
			rs.detection.fg.det.SaveNow()
		}
		if rs.capture.cam != nil {
			rs.capture.cam.Stop()
		}
		if rs.detection.fg != nil {
			// Release the detector's OpenCV resources. The frame loop can no
			// longer be inside the detector (see frameMu above).
			rs.detection.fg.det.Close()
		}
		if rs.io.commandQueue != nil {
			rs.io.commandQueue.Stop()
		}
		if rs.io.arduino != nil {
			_ = rs.io.arduino.Disconnect()
		}
		if rs.web.webServer != nil {
			rs.web.webServer.Stop()
		}
		utils.Logf("System stopped")
	})
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

// executeAutonomousControl handles path-following for all tracked robots.
func (rs *RobotSystem) executeAutonomousControl(tracks []tracking.Track) {
	if rs.GetControlMode() != ControlModeAutonomous || rs.IsEmergencyStopped() {
		return
	}
	// Demo mode can fall back from a real camera that's temporarily
	// unavailable while a real Arduino stays connected (Initialize() wires
	// up Arduino/commandQueue regardless of demo vs. real-camera mode). Never
	// let synthetic demo-tag positions drive real hardware.
	if rs.demoMode && rs.io.arduino != nil && rs.io.arduino.IsConnected() {
		utils.Logf("Refusing autonomous control: demo mode is active with a real Arduino connected")
		return
	}

	commandIssued := false
	anyPath := false
	for i := range tracks {
		track := &tracks[i]
		if track.State != tracking.TrackStateConfirmed || track.TagID == nil {
			continue
		}

		robotID := *track.TagID

		if _, hasPath := rs.planning.planner.GetNextWaypoint(robotID); hasPath {
			anyPath = true
			if rs.position.positionEst == nil {
				continue
			}

			// Use the position ProcessFrame computed (it includes the robot's
			// center_offset correction and is what the planner was given), not
			// a fresh uncorrected projection of the bbox.
			worldPos := position.Point2D{X: track.WorldPos[0], Y: track.WorldPos[1]}

			// Stop and replan when dangerously close to an obstacle
			if clearance := rs.planning.planner.GetClearance(robotID); clearance < 0.08 {
				utils.Logf("PROXIMITY WARNING: Robot %d clearance=%.3fm — stopping and replanning", robotID, clearance)
				if rs.io.commandQueue != nil {
					rs.io.commandQueue.Enqueue(controller.CommandStop)
					commandIssued = true
				}
				rs.io.robotCommands[robotID] = "stopped"
				// Replan at most once every 3 seconds to avoid thrashing
				if lastReplan, ok := rs.planning.lastReplanTime[robotID]; !ok || time.Since(lastReplan) > 3*time.Second {
					if goal, hasGoal := rs.planning.planner.GetGoal(robotID); hasGoal {
						rs.planning.planner.ClearPathOnly(robotID)
						pos := [2]float64{worldPos.X, worldPos.Y}
						if newPath, ok := rs.planning.planner.PlanPath(robotID, pos, goal); ok {
							rs.planning.planner.SetPath(robotID, newPath)
							utils.Logf("Robot %d replanned: %d waypoints from (%.2f,%.2f)", robotID, len(newPath), pos[0], pos[1])
						} else {
							utils.Logf("Robot %d replan FAILED from (%.2f,%.2f) to (%.2f,%.2f)", robotID, pos[0], pos[1], goal[0], goal[1])
						}
						rs.planning.lastReplanTime[robotID] = time.Now()
					}
				}
				continue
			}

			// Check if robot is close to final destination
			if goal, hasGoal := rs.planning.planner.GetGoal(robotID); hasGoal {
				dx := worldPos.X - goal[0]
				dy := worldPos.Y - goal[1]
				distToGoal := math.Sqrt(dx*dx + dy*dy)
				if distToGoal < rs.io.waypointThreshold {
					rs.planning.planner.CompletePath(robotID)
					rs.web.webServer.ClearDestination(robotID)
					utils.Logf("Robot %d reached goal (%.2fm away), stopping", robotID, distToGoal)
					if rs.io.commandQueue != nil {
						rs.io.commandQueue.Enqueue(controller.CommandStop)
						commandIssued = true
					}
					rs.io.robotCommands[robotID] = "stopped"
					continue
				}
			}

			// Advance past any reached or overshot waypoints
			if !rs.planning.planner.AdvancePastWaypoints(robotID, [2]float64{worldPos.X, worldPos.Y}, rs.io.waypointThreshold) {
				utils.Logf("Robot %d reached final waypoint, stopping", robotID)
				if rs.io.commandQueue != nil {
					rs.io.commandQueue.Enqueue(controller.CommandStop)
					commandIssued = true
				}
				rs.io.robotCommands[robotID] = "stopped"
				continue
			}

			// Get updated waypoint after advancing
			waypoint, stillHasPath := rs.planning.planner.GetNextWaypoint(robotID)
			if !stillHasPath {
				continue
			}

			// Heading-based steering: turn to face waypoint, then drive forward
			dx := waypoint[0] - worldPos.X
			dy := waypoint[1] - worldPos.Y
			bearingToWaypoint := math.Atan2(dy, dx)

			if rs.io.pathExecutor != nil && rs.io.commandQueue != nil {
				delta := rs.heading.headingDelta[robotID]
				cmd := rs.io.pathExecutor.BearingToCommand(track.Heading, bearingToWaypoint, delta)
				rs.io.commandQueue.Enqueue(cmd)
				commandIssued = true
				switch cmd {
				case controller.CommandForward:
					rs.io.robotCommands[robotID] = "forward"
				case controller.CommandBackward:
					rs.io.robotCommands[robotID] = "backward"
				case controller.CommandLeft:
					rs.io.robotCommands[robotID] = "rotating_left"
				case controller.CommandRight:
					rs.io.robotCommands[robotID] = "rotating_right"
				case controller.CommandStop:
					rs.io.robotCommands[robotID] = "stopped"
				}
			}
		}
	}

	// Only clear the active command when no robot is following a path. Doing
	// it per robot let a path-less robot wipe the command just issued for the
	// robot that does have one.
	if !anyPath && rs.io.commandQueue != nil && rs.io.commandQueue.IsRunning() {
		rs.io.commandQueue.ClearActiveCommand()
	}

	// Safety: stop re-sending stale commands when tracking is lost for too long.
	if commandIssued {
		rs.io.lastCommandTime = time.Now()
	} else if rs.io.commandQueue != nil && time.Since(rs.io.lastCommandTime) > rs.io.trackingLostTimeout {
		rs.io.commandQueue.ClearActiveCommand()
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
		rs.web.webServer.BroadcastTracks(trackingResult.Tracks, robots, rs.io.robotCommands)

		rs.executeAutonomousControl(trackingResult.Tracks)
	}

	if rs.detection.detectionPipe != nil && rs.web.webServer != nil && rs.web.webServer.CalibrationViewActive() {
		rs.web.webServer.PushFrame(img)
	} else if rs.detection.detectionPipe != nil && rs.web.webServer != nil {
		overlay := rs.detection.detectionPipe.DrawResults(img.Pix, width, height, result)
		if overlay != nil {
			overlayImg := decodeToImage(overlay, width, height)
			if overlayImg != nil {
				rs.web.webServer.PushFrame(overlayImg)
			} else {
				rs.web.webServer.PushFrame(img)
			}
		} else {
			rs.web.webServer.PushFrame(img)
		}
	}

	rs.broadcastFrameStats(len(demoTags), len(demoTags), demoTags, width, height, demoCalibrationTagInfos(width, height))
}

//gocyclo:ignore
func main() {
	configPath := flag.String("config", "config/tracking_config.yaml", "Path to configuration file")
	listPorts := flag.Bool("list-ports", false, "List available serial ports")
	listCamerasFlag := flag.Bool("list-cameras", false, "List available cameras")
	testCameraID := flag.Int("test-camera", -1, "Test specific camera by ID")
	demoMode := flag.Bool("demo", false, "Run demo mode with test pattern")
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
	if rs.demoMode {
		rs.registerDemoRobots()
	}
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

	// If camera isn't available yet (e.g., macOS permission dialog pending),
	// retry with backoff before falling back to demo mode.
	if rs.capture.cam == nil && !*demoMode && rs.capture.cameraConfig != nil {
		utils.Log("Camera not available at startup, retrying (waiting for permission?)...")
		if err := rs.tryOpenCamera(2 * time.Minute); err != nil {
			utils.Logf("Camera unavailable after retries: %v, falling back to demo mode", err)
			*demoMode = true
		}
	}
	// If camera still isn't available and not in demo mode, fall back
	if rs.capture.cam == nil && !*demoMode {
		utils.Log("No camera available, falling back to demo mode")
		*demoMode = true
	}

	if rs.capture.cam != nil && !*demoMode {
		utils.Log("Starting real camera capture...")
		if err := rs.StartCamera(); err != nil {
			utils.Logf("Failed to start camera: %v, falling back to demo mode", err)
			*demoMode = true
		} else {
			utils.Logf("Starting real camera capture...")
			frameFailures := 0
			minFrameInterval := time.Second / time.Duration(rs.cfg.EffectiveMaxFPS())
			utils.Logf("Processing capped at %d fps", rs.cfg.EffectiveMaxFPS())
			for rs.capture.cameraRunning.Load() {
				startTime := time.Now()
				frame, err := rs.capture.cam.GetFrame()
				if err != nil {
					frameFailures++
					rs.stats.perf.RecordCameraFailure()
					if frameFailures == 1 || frameFailures%50 == 0 {
						utils.Logf("Failed to get frame (%d in a row): %v", frameFailures, err)
					}
					time.Sleep(100 * time.Millisecond)
					continue
				}
				if frameFailures > 0 {
					utils.Logf("Camera frames resumed after %d failed reads", frameFailures)
					frameFailures = 0
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
				if elapsed := time.Since(startTime); elapsed < minFrameInterval {
					time.Sleep(minFrameInterval - elapsed)
				}
			}
		}
	}

	if *demoMode {
		// Also covers falling back to demo because the camera never opened:
		// calibrating on synthetic frames must not overwrite the real file.
		rs.demoMode = true
		rs.registerDemoRobots()
		if rs.web.webServer != nil {
			rs.web.webServer.SetCameraName(demoCameraName)
		}
	}

	if *demoMode {
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
}
