//go:build gocv

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/camera"
	"github.com/chrisl8/robot_tracker_go/internal/config"
	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

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

//gocyclo:ignore
func main() {
	configPath := flag.String("config", "config/tracking_config.yaml", "Path to configuration file")
	listPorts := flag.Bool("list-ports", false, "List available serial ports")
	listCamerasFlag := flag.Bool("list-cameras", false, "List available cameras")
	testCameraID := flag.Int("test-camera", -1, "Test specific camera by ID")
	demoMode := flag.Bool("demo", false, "Run demo mode with test pattern")
	quiet := flag.Bool("quiet", false, "Suppress all logging output")
	debug := flag.Bool("debug", false, "Log per-frame debug detail (also enabled by ROBOT_TRACKER_DEBUG=1)")
	logFile := flag.String("log-file", "", "Log to file with rotation (default: log to stderr)")
	flag.Parse()

	if *logFile != "" {
		f, err := utils.SetupLogFile(*logFile, 3)
		if err != nil {
			log.Fatalf("Failed to set up log file: %v", err)
		}
		defer func() { _ = f.Close() }()
	}

	if *debug {
		utils.SetDebug(true)
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

	demo := rs.resolveDemoMode(*demoMode)

	if !demo && !rs.runCameraLoop() {
		demo = true // the camera would not start
	}

	if demo {
		rs.enterDemoMode()
		rs.runDemoLoop()
	}
}
