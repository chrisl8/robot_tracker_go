//go:build gocv

package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/ui"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

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
			// Obstacles are stored in pixels; their floor coordinates depend on
			// the calibration that was just replaced.
			rs.web.webServer.RecomputeObstacleWorld()
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
		// The file's world coordinates date from whenever it was saved; redo
		// them from the pixel boxes under the calibration now loaded.
		rs.web.webServer.RecomputeObstacleWorld()
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
