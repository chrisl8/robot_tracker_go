//go:build gocv

package main

import (
	"fmt"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/camera"
	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
	"github.com/chrisl8/robot_tracker_go/internal/ui"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

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
