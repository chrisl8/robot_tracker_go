//go:build gocv

package main

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/camera"
	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
	"github.com/chrisl8/robot_tracker_go/internal/ui"
)

// The sub-structs below group RobotSystem's fields by the subsystem that
// owns them, so a field access like rs.heading.lastHeading names both the
// concern and the concrete field, instead of an unqualified rs.lastHeading
// that gives no hint of which of RobotSystem's many collaborators it belongs
// to. See the doc comment on RobotSystem for how these compose and the order
// Initialize() sets them up in.
//
// These are named (not embedded) fields deliberately: embedding would let
// every existing call site keep compiling unchanged, but it would also
// silently promote every field back onto RobotSystem itself, throwing away
// the exact traceability this split exists to add, and it creates a latent
// ambiguous-selector trap the day two of these types ever grow same-named
// methods. Every field below keeps its original name from the old flat
// RobotSystem struct — only a group qualifier was added.

// cameraSubsystem owns the physical/virtual camera handle used for capture.
type cameraSubsystem struct {
	cam           camera.Camera
	cameraRunning atomic.Bool
	cameraConfig  *camera.CameraConfig // stored for retry if initial open fails

	// frameMu is held for the whole of ProcessFrame/ProcessDemoFrame. Stop()
	// takes it (and sets stopped) so it never saves or closes the detector's
	// OpenCV resources while a frame is still being processed, and so any
	// frame arriving after shutdown began is dropped instead.
	frameMu sync.Mutex
	stopped bool // guarded by frameMu
}

// detectionSubsystem owns AprilTag detection and the background-subtraction
// foreground/obstacle detector.
type detectionSubsystem struct {
	detectionPipe *detection.DetectionPipeline
	fg            *foregroundGlue
}

// trackingSubsystem owns the multi-object tracker.
type trackingSubsystem struct {
	tracker tracking.Tracker
}

// planningSubsystem owns the path planner.
type planningSubsystem struct {
	planner        *planning.Planner
	lastReplanTime map[int]time.Time // robotID -> last proximity replan time
}

// obstaclesSubsystem owns the static (config-file) obstacle list.
type obstaclesSubsystem struct {
	StaticObstacles []planning.Obstacle
}

// positionSubsystem owns pixel<->world coordinate calibration.
type positionSubsystem struct {
	positionEst *position.PositionEstimator
}

// controlIOSubsystem owns the Arduino serial connection, its command queue,
// and the path-following state derived from planned waypoints.
type controlIOSubsystem struct {
	arduino             *controller.ArduinoController
	commandQueue        *controller.CommandQueue
	pathExecutor        *controller.PathExecutor
	waypointThreshold   float64
	trackingLostTimeout time.Duration
	lastCommandTime     time.Time
	robotCommands       map[int]string // tag_id -> current motion state
}

// webSubsystem owns the HTTP/WebSocket/MJPEG server.
type webSubsystem struct {
	webServer *ui.WebServer
}

// controlStateSubsystem owns the operator-facing control mode and e-stop
// state, guarded by its own mutex since it's read/written from both the
// frame-processing goroutine and HTTP handler goroutines.
type controlStateSubsystem struct {
	controlMode      ControlMode
	emergencyStopped bool
	controlMu        sync.RWMutex
}

// headingSubsystem owns per-robot heading smoothing/rejection state.
type headingSubsystem struct {
	lastHeading        map[int]float64
	headingDelta       map[int]float64
	smoothedHeading    map[int]float64
	headingRejectCount map[int]int
	headingLostCount   map[int]int
}

// frameTimingSubsystem owns per-frame bookkeeping: FPS smoothing, watchdog
// state, shutdown idempotency, and CPU/perf-window tracking.
type frameTimingSubsystem struct {
	frameNum            int
	lastFrameTime       time.Time
	lastStatusBroadcast time.Time
	lastFrameNanos      atomic.Int64
	watchdogOnce        sync.Once
	watchdogStopOnce    sync.Once
	watchdogStop        chan struct{}
	stopOnce            sync.Once
	perf                perfWindow
	lastCPUSeconds      float64
	lastCPUAt           time.Time
	lastOffenderLog     time.Time
	smoothedFPS         float64
	startTime           time.Time
}
