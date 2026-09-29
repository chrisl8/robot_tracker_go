//go:build gocv

package ui

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// WebServer serves the HTTP API, MJPEG stream, and WebSocket overlay feed,
// and holds several independently-locked pieces of application state that
// the HTTP handlers read and write. Each state cluster gets its own
// sub-struct (and its own mutex, unchanged from before this split) rather
// than living as a flat list of fields, since a newcomer reading "does the
// obstacle store share a lock with calibration?" should be able to answer
// that from the struct definition alone:
//
//   - router: HTTP/WS/MJPEG server lifecycle (addr, engine, listener).
//   - hub: the broadcast/WebSocket client registry (see BroadcastOverlay).
//   - obstacles: the user-placed obstacle list and its file persistence.
//   - calibration: calibration status, detected AprilTags during
//     calibration, the camera name, and the position estimator used to
//     convert pixel obstacle corners to world coordinates.
//   - stats: FPS/track-count/memory telemetry and the Arduino-connected flag.
//   - destination: the single pending click-to-drive destination.
//   - Callbacks: every app-supplied hook (see WebServerCallbacks) — the
//     seam between this package's HTTP layer and cmd's application logic.
//
// calibration.cameraName and calibration.positionEstimator are also used by
// the obstacle store and the hub (GetObstaclesPath, BroadcastPaths); they are
// guarded by calibration.mutex and read through cameraNameValue/estimator.
type WebServer struct {
	router      webRouter
	hub         broadcastHub
	obstacles   obstacleStore
	calibration calibrationStore
	stats       statsStore
	destination destinationStore
	Callbacks   WebServerCallbacks
}

// webRouter owns the HTTP/WS/MJPEG server's lifecycle: the gin engine, the
// MJPEG snapshot stream, and the *http.Server started by Start/stopped by Stop.
type webRouter struct {
	addr         string
	engine       *gin.Engine
	stream       *mjpegStream
	isRunning    atomic.Bool
	httpServer   *http.Server
	httpServerMu sync.Mutex
	wsPongWait   time.Duration
	wsPingPeriod time.Duration
	wsWriteWait  time.Duration
}

// broadcastHub is the registry of connected WebSocket clients that
// BroadcastOverlay fans messages out to.
type broadcastHub struct {
	clients     map[*websocket.Conn]*wsClient
	clientMutex sync.RWMutex
}

// obstacleStore holds the user-placed obstacle list and its on-disk path,
// independent of everything else WebServer tracks.
type obstacleStore struct {
	mutex sync.RWMutex
	list  []planning.Obstacle
	saved bool
	path  string
}

// calibrationStore holds calibration status/progress, the AprilTags
// detected while the calibration wizard is open, the camera's name, and the
// position estimator calibration produces — grouped together since
// calibration is what populates and consumes all of them.
type calibrationStore struct {
	mutex      sync.RWMutex
	state      string
	message    string
	filename   string
	tagSize    float64
	cameraName string

	detectedTagsMut sync.RWMutex
	detectedTags    []DetectedTagInfo
	detectedFrameW  int
	detectedFrameH  int
	lastTagPoll     time.Time
	lastTagUpdate   time.Time

	positionEstimator *position.PositionEstimator
}

// statsStore holds telemetry pushed by the frame loop (FPS, track count,
// memory) and the Arduino-connected flag shown alongside it.
type statsStore struct {
	mutex         sync.RWMutex
	lastTagCount  int
	lastFPS       float64
	lastUptimeSec float64
	lastHostMemMB float64

	arduinoMutex     sync.RWMutex
	arduinoConnected bool
}

// destinationStore holds the single pending click-to-drive destination.
type destinationStore struct {
	mutex   sync.RWMutex
	current DestinationMessage
}

// WebServerCallbacks are the app-supplied hooks WebServer calls into for
// side effects it doesn't own (persistence, hardware control, planning).
// Every field is optional: WebServer nil-checks each one before calling it,
// so a hook the app never wires (e.g. every foreground/control hook in demo
// mode — see cmd/main.go's initDemoMode) is a documented no-op, not a bug.
type WebServerCallbacks struct {
	OnObstaclesChanged func([]planning.Obstacle)

	// Foreground (temporary obstacle) detector controls, set by the app.
	OnForegroundEnabled   func(bool)
	OnForegroundApply     func(bool)
	OnForegroundReset     func()
	OnForegroundAbsorb    func(x, y int)
	ForegroundDebugJPEG   func() []byte
	ForegroundState       func() ForegroundState
	OnDestinationSet      func(int, [2]float64) error
	OnDestinationClear    func(int)
	OnCalibrationComplete func(string)
	OnCommand             func(string) error
	OnModeChange          func(string) error
	OnEmergencyStop       func()
	OnClearEmergencyStop  func() error
	OnGetControlState     func() (string, bool)

	OnPathsChanged func() map[int][][2]float64
}

func NewWebServer(addr string) *WebServer {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(requestGuardMiddleware())

	server := &WebServer{
		router: webRouter{
			addr:         addr,
			engine:       engine,
			stream:       newMJPEGStream(),
			wsPongWait:   defaultWSPongWait,
			wsPingPeriod: defaultWSPingPeriod,
			wsWriteWait:  defaultWSWriteWait,
		},
		hub: broadcastHub{
			clients: make(map[*websocket.Conn]*wsClient),
		},
	}

	server.setupRoutes()
	return server
}

func (s *WebServer) setupRoutes() {
	staticFS, err := fs.Sub(StaticFiles, "static")
	if err != nil {
		utils.Logf("Warning: Failed to create static FS sub-directory: %v", err)
	} else {
		s.router.engine.GET("/assets/*path", gin.WrapH(http.FileServer(http.FS(staticFS))))
		s.router.engine.GET("/calibration-tags/*path", gin.WrapH(http.FileServer(http.FS(staticFS))))
	}
	s.router.engine.GET("/", s.handleIndex)
	s.router.engine.GET("/stream", s.handleMJPEG)
	s.router.engine.GET("/ws", s.handleWebSocket)
	s.router.engine.POST("/api/command", s.handleCommand)
	s.router.engine.POST("/api/destination", s.handleDestination)
	s.router.engine.DELETE("/api/destination", s.handleDestinationClear)
	s.router.engine.GET("/api/status", s.handleStatus)
	s.router.engine.GET("/api/obstacles", s.handleObstaclesList)
	s.router.engine.POST("/api/obstacles", s.handleObstacleAdd)
	s.router.engine.DELETE("/api/obstacles/:id", s.handleObstacleDelete)
	s.router.engine.PUT("/api/obstacles/:id", s.handleObstacleUpdate)
	s.router.engine.POST("/api/obstacles/clear", s.handleObstaclesClear)
	s.router.engine.POST("/api/obstacles/save", s.handleObstaclesSave)
	s.router.engine.GET("/api/calibration/status", s.handleCalibrationStatus)
	s.router.engine.POST("/api/calibration/start", s.handleCalibrationStart)
	s.router.engine.GET("/api/calibration/detected-tags", s.handleCalibrationDetectedTags)
	s.router.engine.POST("/api/calibration/compute", s.handleCalibrationCompute)
	s.router.engine.POST("/api/calibration/cancel", s.handleCalibrationCancel)
	s.router.engine.GET("/api/foreground/state", s.handleForegroundState)
	s.router.engine.POST("/api/foreground/enabled", s.handleForegroundEnabled)
	s.router.engine.POST("/api/foreground/apply", s.handleForegroundApply)
	s.router.engine.POST("/api/foreground/reset", s.handleForegroundReset)
	s.router.engine.POST("/api/foreground/absorb", s.handleForegroundAbsorb)
	s.router.engine.GET("/api/foreground/debug.jpg", s.handleForegroundDebug)
	s.router.engine.POST("/api/mode", s.handleSetMode)
	s.router.engine.POST("/api/emergency-stop", s.handleEmergencyStop)
	s.router.engine.POST("/api/clear-emergency-stop", s.handleClearEmergencyStop)
	s.router.engine.GET("/api/control-state", s.handleControlState)
}

func (s *WebServer) Start() {
	s.router.isRunning.Store(true)
	srv := &http.Server{
		Addr:              s.router.addr,
		Handler:           s.router.engine,
		ReadHeaderTimeout: 5 * time.Second,
		// No WriteTimeout: the MJPEG stream and WebSocket write for as long as
		// the client stays connected.
		ReadTimeout: 30 * time.Second,
		IdleTimeout: 2 * time.Minute,
	}
	s.router.httpServerMu.Lock()
	s.router.httpServer = srv
	s.router.httpServerMu.Unlock()
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			utils.Logf("HTTP server error: %v", err)
		}
	}()
}

// Stop gracefully shuts down the HTTP/WS/MJPEG listener started by Start,
// waiting up to 5s for in-flight requests (including open WebSocket/MJPEG
// streams) to finish before forcing the listener closed.
func (s *WebServer) Stop() {
	s.router.isRunning.Store(false)
	s.router.httpServerMu.Lock()
	srv := s.router.httpServer
	s.router.httpServerMu.Unlock()
	if srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		utils.Logf("HTTP server shutdown error: %v", err)
	}
	utils.Log("Web server stopped")
}
