//go:build gocv

package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/config"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
	"github.com/chrisl8/robot_tracker_go/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
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
// Two fields cross these boundaries and are read without the "owning"
// cluster's lock, exactly as before this split (not introduced by it):
// calibration.cameraName is also read by the obstacle store's
// GetObstaclesPath, and calibration.positionEstimator is also read by the
// hub's BroadcastPaths. Fixing that pre-existing lack of synchronization is
// a correctness change, out of scope here — see
// docs/archived/code-review-2026-09-27.md.
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
	isRunning    bool
	httpServer   *http.Server
	httpServerMu sync.Mutex
	wsPongWait   time.Duration
	wsPingPeriod time.Duration
	wsWriteWait  time.Duration
}

// broadcastHub is the registry of connected WebSocket clients that
// BroadcastOverlay fans messages out to.
type broadcastHub struct {
	clients     map[*websocket.Conn]*sync.Mutex
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
	OnDestinationSet      func(int, [2]float64)
	OnDestinationClear    func(int)
	OnCalibrationComplete func(string)
	OnCommand             func(string) error
	OnModeChange          func(string) error
	OnEmergencyStop       func()
	OnClearEmergencyStop  func() error
	OnGetControlState     func() (string, bool)

	OnPathsChanged func() map[int][][2]float64
}

type OverlayMessage struct {
	Type          string                    `json:"type"`
	BBox          *BBoxMessage              `json:"bbox,omitempty"`
	Track         *TrackMessage             `json:"track,omitempty"`
	Tracks        *TracksMessage            `json:"tracks,omitempty"`
	Path          *PathMessage              `json:"path,omitempty"`
	Paths         *PathsMessage             `json:"paths,omitempty"`
	Status        *StatusMessage            `json:"status,omitempty"`
	Command       *CommandMessage           `json:"command,omitempty"`
	Calibration   *CalibrationStatusMessage `json:"calibration,omitempty"`
	Obstacles     *ObstaclesMessage         `json:"obstacles,omitempty"`
	Destination   *DestinationMessage       `json:"destination,omitempty"`
	TempObstacles *TempObstaclesMessage     `json:"temp_obstacles,omitempty"`
}

// TempObstacleResponse is one temporary (detected, not user-marked) obstacle.
type TempObstacleResponse struct {
	ID               string     `json:"id"`
	PixelTopLeft     [2]int     `json:"pixel_top_left"`
	PixelBottomRight [2]int     `json:"pixel_bottom_right"`
	WorldTopLeft     [2]float64 `json:"world_top_left"`
	WorldBottomRight [2]float64 `json:"world_bottom_right"`
	// PixelQuad/WorldQuad are the obstacle's exact (possibly rotated)
	// footprint, four corners in order, omitted when unavailable (in which
	// case the UI should fall back to the rectangle above). world_top_left/
	// world_bottom_right/pixel_* stay populated as that shape's bounding box.
	PixelQuad [][2]int     `json:"pixel_quad,omitempty"`
	WorldQuad [][2]float64 `json:"world_quad,omitempty"`
}

// TempObstaclesMessage carries the current temporary obstacles and the
// detector's state to the UI.
type TempObstaclesMessage struct {
	Obstacles []TempObstacleResponse `json:"obstacles"`
	Applied   bool                   `json:"applied"`
	Warming   bool                   `json:"warming"`
	Guarded   bool                   `json:"guarded"`
	Enabled   bool                   `json:"enabled"`
}

// ForegroundState is the detector's state as returned by GET /api/foreground/state.
type ForegroundState struct {
	Enabled bool `json:"enabled"`
	Applied bool `json:"applied"`
	Warming bool `json:"warming"`
	Guarded bool `json:"guarded"`
	Count   int  `json:"count"`
	// ShadowSuppressed is how many pixels the last frame were reclassified
	// from would-be foreground to background as a cast shadow — diagnostic,
	// for correlating a flagged obstacle with heavy shadow activity nearby.
	ShadowSuppressed int `json:"shadow_suppressed"`
}

type TracksMessage struct {
	Tracks []TrackMessage `json:"tracks"`
}

type ObstaclesMessage struct {
	Obstacles []ObstacleResponse `json:"obstacles"`
	Count     int                `json:"count"`
}

type BBoxMessage struct {
	X1         int     `json:"x1"`
	Y1         int     `json:"y1"`
	X2         int     `json:"x2"`
	Y2         int     `json:"y2"`
	Label      string  `json:"label"`
	Color      string  `json:"color"`
	Confidence float64 `json:"confidence"`
}

type TrackMessage struct {
	ID            int            `json:"id"`
	TagID         *int           `json:"tag_id,omitempty"`
	BBox          []int          `json:"bbox"`
	History       [][2]int       `json:"history"`
	Color         string         `json:"color"`
	Confidence    float64        `json:"confidence"`
	State         string         `json:"state"`
	Configured    bool           `json:"configured"`
	Name          string         `json:"name,omitempty"`
	PixelRadius   *float64       `json:"pixel_radius,omitempty"`
	Heading       *float64       `json:"heading,omitempty"`
	Corners       *[4][2]float64 `json:"corners,omitempty"`
	HeadingOffset *float64       `json:"heading_offset,omitempty"`
	MotionState   *string        `json:"motion_state,omitempty"`
}

type PathMessage struct {
	RobotID int      `json:"robot_id"`
	Points  [][2]int `json:"points"`
	Color   string   `json:"color"`
}

type PathsMessage struct {
	Paths []PathMessage `json:"paths"`
}

type StatusMessage struct {
	Connected    bool    `json:"connected"`
	FPS          float64 `json:"fps"`
	RobotCount   int     `json:"robotCount"`
	ArduinoState string  `json:"arduinoState"`
	HostMemoryMB float64 `json:"hostMemoryMB,omitempty"`
	UptimeSec    float64 `json:"uptimeSec,omitempty"`
	// CameraStalled is true when no video frame has been processed for a couple
	// of seconds; FPS is then 0 and FrameAgeSec says for how long.
	CameraStalled bool    `json:"cameraStalled,omitempty"`
	FrameAgeSec   float64 `json:"frameAgeSec,omitempty"`
}

type CommandMessage struct {
	Action string `json:"action"`
}

type DestinationRequest struct {
	RobotID int `json:"robot_id"`
	X       int `json:"x"`
	Y       int `json:"y"`
}

type DestinationMessage struct {
	RobotID int  `json:"robot_id"`
	X       int  `json:"x"`
	Y       int  `json:"y"`
	Valid   bool `json:"valid"`
}

type CommandRequest struct {
	Command string `json:"command"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     originAllowed,
}

const wsMaxMessageSize = 4096 // the only thing a client sends is a small {"command": "..."} JSON object

// Default WebSocket liveness timings. These live as fields on WebServer
// (defaulted below) rather than package-level vars so a test can shrink one
// server's timers without racing another, concurrently-running server's
// goroutines that read the same values.
const (
	// defaultWSPongWait is how long we tolerate silence from a client (no
	// data frame, no pong) before treating the connection as dead.
	defaultWSPongWait = 60 * time.Second
	// defaultWSPingPeriod must be shorter than defaultWSPongWait so a ping
	// always has time to be answered before the read deadline expires.
	defaultWSPingPeriod = (defaultWSPongWait * 9) / 10
	// defaultWSWriteWait bounds how long a single write (ping or broadcast
	// message) may block on a client that has stopped reading.
	defaultWSWriteWait = 10 * time.Second
)

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
			isRunning:    false,
			wsPongWait:   defaultWSPongWait,
			wsPingPeriod: defaultWSPingPeriod,
			wsWriteWait:  defaultWSWriteWait,
		},
		hub: broadcastHub{
			clients: make(map[*websocket.Conn]*sync.Mutex),
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

func (s *WebServer) handleIndex(c *gin.Context) {
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	data, err := StaticFiles.ReadFile("static/index.html")
	if err != nil {
		c.String(500, "Failed to load index.html: %v", err)
		return
	}
	c.Data(200, "text/html; charset=utf-8", data)
}

func (s *WebServer) handleMJPEG(c *gin.Context) {
	s.router.stream.serveHTTP(c.Writer, c.Request)
}

func (s *WebServer) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	// A client that vanishes without a clean TCP close (sleep, NAT timeout,
	// pulled network cable) never makes ReadMessage return an error on its
	// own — without a deadline it just blocks forever, leaking this
	// connection's goroutine and its s.hub.clients entry, while BroadcastOverlay
	// keeps trying to write to it every frame. SetReadDeadline plus a pong
	// handler that renews it turns silence into a timeout error, and
	// wsPinger below is what solicits those pongs.
	conn.SetReadLimit(wsMaxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(s.router.wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(s.router.wsPongWait))
	})

	mu := &sync.Mutex{}
	s.hub.clientMutex.Lock()
	s.hub.clients[conn] = mu
	s.hub.clientMutex.Unlock()

	go s.wsReader(conn)
	go s.wsPinger(conn, mu)
}

func (s *WebServer) wsReader(conn *websocket.Conn) {
	defer func() {
		_ = conn.Close()
		s.hub.clientMutex.Lock()
		delete(s.hub.clients, conn)
		s.hub.clientMutex.Unlock()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var cmd CommandRequest
		if err := json.Unmarshal(message, &cmd); err == nil && cmd.Command != "" {
			s.broadcastCommand(cmd.Command)
		}
	}
}

// wsPinger periodically pings conn to detect a silently-dead connection (see
// the comment in handleWebSocket) and to keep the read deadline from
// expiring on an otherwise-idle-but-alive client. It shares mu with
// BroadcastOverlay so writes to this connection are never interleaved
// (gorilla/websocket panics if two goroutines write to the same connection
// concurrently), and it exits once wsReader has removed conn from s.hub.clients.
func (s *WebServer) wsPinger(conn *websocket.Conn, mu *sync.Mutex) {
	ticker := time.NewTicker(s.router.wsPingPeriod)
	defer ticker.Stop()

	for range ticker.C {
		s.hub.clientMutex.RLock()
		_, stillConnected := s.hub.clients[conn]
		s.hub.clientMutex.RUnlock()
		if !stillConnected {
			return
		}

		mu.Lock()
		_ = conn.SetWriteDeadline(time.Now().Add(s.router.wsWriteWait))
		err := conn.WriteMessage(websocket.PingMessage, nil)
		mu.Unlock()
		if err != nil {
			// The write failed (or blocked past its deadline); close so
			// wsReader's blocked ReadMessage unblocks with an error and
			// cleans up s.hub.clients.
			_ = conn.Close()
			return
		}
	}
}

func (s *WebServer) broadcastCommand(cmd string) {
	msg := OverlayMessage{
		Type:    "command",
		Command: &CommandMessage{Action: cmd},
	}
	s.BroadcastOverlay(msg)
}

// BroadcastOverlay sends msg to every connected client. gorilla/websocket
// panics if two goroutines write to the same connection at once (this project
// broadcasts from the frame loop, HTTP handlers, and a watchdog timer, all
// concurrently), so each connection's writes are serialized with its own
// mutex while the client list itself is only read-locked.
func (s *WebServer) BroadcastOverlay(msg OverlayMessage) {
	s.hub.clientMutex.RLock()
	clients := make(map[*websocket.Conn]*sync.Mutex, len(s.hub.clients))
	for conn, mu := range s.hub.clients {
		clients[conn] = mu
	}
	s.hub.clientMutex.RUnlock()

	for conn, mu := range clients {
		mu.Lock()
		_ = conn.SetWriteDeadline(time.Now().Add(s.router.wsWriteWait))
		err := conn.WriteJSON(msg)
		mu.Unlock()
		if err != nil {
			// Don't leave a connection that just failed to write sitting
			// around until the next ping cycle notices it's dead — close it
			// now so wsReader unblocks and cleans up s.hub.clients.
			_ = conn.Close()
		}
	}
}

// BroadcastTempObstacles sends the temporary obstacles and detector state to
// every connected UI.
func (s *WebServer) BroadcastTempObstacles(msg TempObstaclesMessage) {
	if msg.Obstacles == nil {
		msg.Obstacles = []TempObstacleResponse{}
	}
	s.BroadcastOverlay(OverlayMessage{Type: "temp_obstacles", TempObstacles: &msg})
}

func (s *WebServer) handleForegroundState(c *gin.Context) {
	state := ForegroundState{}
	if s.Callbacks.ForegroundState != nil {
		state = s.Callbacks.ForegroundState()
	}
	c.JSON(http.StatusOK, state)
}

func (s *WebServer) handleForegroundEnabled(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be {\"enabled\": true|false}"})
		return
	}
	if s.Callbacks.OnForegroundEnabled != nil {
		s.Callbacks.OnForegroundEnabled(*req.Enabled)
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "enabled": *req.Enabled})
}

func (s *WebServer) handleForegroundApply(c *gin.Context) {
	var req struct {
		Apply *bool `json:"apply"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Apply == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be {\"apply\": true|false}"})
		return
	}
	if s.Callbacks.OnForegroundApply != nil {
		s.Callbacks.OnForegroundApply(*req.Apply)
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "apply": *req.Apply})
}

func (s *WebServer) handleForegroundReset(c *gin.Context) {
	if s.Callbacks.OnForegroundReset != nil {
		s.Callbacks.OnForegroundReset()
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *WebServer) handleForegroundAbsorb(c *gin.Context) {
	var req struct {
		X *float64 `json:"x"`
		Y *float64 `json:"y"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.X == nil || req.Y == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be {\"x\": number, \"y\": number}"})
		return
	}
	if s.Callbacks.OnForegroundAbsorb != nil {
		s.Callbacks.OnForegroundAbsorb(int(*req.X), int(*req.Y))
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleForegroundDebug serves the detector's debug view. Asking for it makes
// the detector render it for the next couple of seconds.
func (s *WebServer) handleForegroundDebug(c *gin.Context) {
	var jpeg []byte
	if s.Callbacks.ForegroundDebugJPEG != nil {
		jpeg = s.Callbacks.ForegroundDebugJPEG()
	}
	if len(jpeg) == 0 {
		c.Status(http.StatusNoContent)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/jpeg", jpeg)
}

func (s *WebServer) BroadcastObstacles() {
	s.obstacles.mutex.RLock()
	defer s.obstacles.mutex.RUnlock()

	obstacles := make([]ObstacleResponse, 0, len(s.obstacles.list))
	for _, obs := range s.obstacles.list {
		obstacles = append(obstacles, ObstacleResponse{
			ID:               obs.Name,
			Name:             obs.Name,
			PixelTopLeft:     obs.PixelsTopLeft,
			PixelBottomRight: obs.PixelsBottomRight,
			WorldTopLeft:     obs.WorldTopLeft,
			WorldBottomRight: obs.WorldBottomRight,
			Clearance:        0.02,
		})
	}

	s.BroadcastOverlay(OverlayMessage{
		Type: "obstacles",
		Obstacles: &ObstaclesMessage{
			Obstacles: obstacles,
			Count:     len(obstacles),
		},
	})
}

// notifyObstaclesChanged broadcasts the current obstacle list to WebSocket
// clients and, if OnObstaclesChanged is set, invokes it with a locked
// snapshot of s.obstacles.list. Callers must call this only after releasing
// obstaclesMutex: BroadcastObstacles takes its own RLock (so calling this
// while still holding the write lock would deadlock), and OnObstaclesChanged
// is arbitrary application code that must not run while any lock is held.
func (s *WebServer) notifyObstaclesChanged() {
	s.BroadcastObstacles()

	if s.Callbacks.OnObstaclesChanged != nil {
		s.obstacles.mutex.RLock()
		obstacles := s.obstacles.list
		s.obstacles.mutex.RUnlock()
		s.Callbacks.OnObstaclesChanged(obstacles)
	}
}

func (s *WebServer) BroadcastTracks(tracks []tracking.Track, robots []config.RobotConfig, robotCommands map[int]string) {
	trackMessages := make([]TrackMessage, 0, len(tracks))
	for _, track := range tracks {
		bbox := []int{track.Bbox[0], track.Bbox[1], track.Bbox[2], track.Bbox[3]}
		history := make([][2]int, len(track.History))
		for i, hp := range track.History {
			history[i] = [2]int{hp.Bbox[0], hp.Bbox[1]}
		}
		msg := TrackMessage{
			ID:          track.TrackID,
			TagID:       track.TagID,
			BBox:        bbox,
			History:     history,
			Confidence:  track.Confidence,
			State:       track.StateString(),
			PixelRadius: &track.PixelRadius,
		}
		if track.TagID != nil {
			for _, rc := range robots {
				if rc.TagID == *track.TagID {
					msg.Configured = true
					msg.Name = rc.Name
					break
				}
			}
		}
		if track.PixelRadius <= 0 {
			msg.PixelRadius = nil
		}
		if track.TagID != nil && track.Corners != [4][2]float64{} {
			heading := track.Heading
			corners := track.Corners
			headingOffset := track.HeadingOffset
			msg.Heading = &heading
			msg.Corners = &corners
			msg.HeadingOffset = &headingOffset
		}
		if track.TagID != nil {
			if ms, ok := robotCommands[*track.TagID]; ok && ms != "" {
				msg.MotionState = &ms
			}
		}
		trackMessages = append(trackMessages, msg)
	}
	s.BroadcastOverlay(OverlayMessage{
		Type:   "tracks",
		Tracks: &TracksMessage{Tracks: trackMessages},
	})
}

func (s *WebServer) handleCommand(c *gin.Context) {
	var req CommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if s.Callbacks.OnCommand != nil {
		if err := s.Callbacks.OnCommand(req.Command); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "command": req.Command})
}

func (s *WebServer) handleDestination(c *gin.Context) {
	var req DestinationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.obstacles.mutex.RLock()
	obstacles := s.obstacles.list
	s.obstacles.mutex.RUnlock()

	if len(obstacles) > 0 {
		destX := float64(req.X)
		destY := float64(req.Y)

		for _, obs := range obstacles {
			if destX >= float64(obs.PixelsTopLeft[0]) && destX <= float64(obs.PixelsBottomRight[0]) &&
				destY >= float64(obs.PixelsTopLeft[1]) && destY <= float64(obs.PixelsBottomRight[1]) {
				utils.Logf("DESTINATION_REJECTED: Destination (%.0f, %.0f) overlaps with obstacle '%s'",
					destX, destY, obs.Name)
				c.JSON(http.StatusBadRequest, gin.H{
					"error":    "Destination overlaps with obstacle",
					"obstacle": obs.Name,
				})
				return
			}
		}
	}

	s.destination.mutex.Lock()
	s.destination.current = DestinationMessage{
		RobotID: req.RobotID,
		X:       req.X,
		Y:       req.Y,
		Valid:   true,
	}
	s.destination.mutex.Unlock()

	if s.Callbacks.OnDestinationSet != nil {
		s.Callbacks.OnDestinationSet(req.RobotID, [2]float64{float64(req.X), float64(req.Y)})
	}

	s.BroadcastOverlay(OverlayMessage{
		Type:        "destination",
		Destination: &s.destination.current,
	})

	c.JSON(http.StatusOK, gin.H{"status": "ok", "destination": req})
}

func (s *WebServer) ClearDestination(robotID int) {
	s.destination.mutex.Lock()
	s.destination.current = DestinationMessage{Valid: false}
	s.destination.mutex.Unlock()

	cleared := DestinationMessage{RobotID: robotID, Valid: false}
	s.BroadcastOverlay(OverlayMessage{
		Type:        "destination_clear",
		Destination: &cleared,
	})
}

func (s *WebServer) handleDestinationClear(c *gin.Context) {
	var req struct {
		RobotID int `json:"robot_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.ClearDestination(req.RobotID)

	if s.Callbacks.OnDestinationClear != nil {
		s.Callbacks.OnDestinationClear(req.RobotID)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "robot_id": req.RobotID})
}

func (s *WebServer) handleStatus(c *gin.Context) {
	s.stats.mutex.RLock()
	tagCount := s.stats.lastTagCount
	fps := s.stats.lastFPS
	uptimeSec := s.stats.lastUptimeSec
	hostMemMB := s.stats.lastHostMemMB
	s.stats.mutex.RUnlock()

	s.stats.arduinoMutex.RLock()
	connected := s.stats.arduinoConnected
	s.stats.arduinoMutex.RUnlock()

	state := "Disconnected"
	if connected {
		state = "Connected"
	}

	c.JSON(http.StatusOK, gin.H{
		"connected":    s.router.isRunning,
		"fps":          fps,
		"robotCount":   tagCount,
		"tagCount":     tagCount,
		"arduinoState": state,
		"hostMemoryMB": hostMemMB,
		"uptimeSec":    uptimeSec,
	})
}

func (s *WebServer) SetCameraName(name string) {
	s.calibration.cameraName = name
}

func (s *WebServer) SetCalibrationState(state, message, filename string, tagSize float64) {
	s.calibration.mutex.Lock()
	s.calibration.state = state
	s.calibration.message = message
	s.calibration.filename = filename
	s.calibration.tagSize = tagSize
	s.calibration.mutex.Unlock()

	s.BroadcastOverlay(OverlayMessage{
		Type: "calibration",
		Calibration: &CalibrationStatusMessage{
			State:    state,
			Message:  message,
			Filename: filename,
			TagSize:  tagSize,
		},
	})
}

func (s *WebServer) SetPositionEstimator(pe *position.PositionEstimator) {
	s.calibration.positionEstimator = pe
}

func (s *WebServer) pixelCornersToWorld(pixelTL, pixelBR [2]int) ([2]float64, [2]float64) {
	if s.calibration.positionEstimator != nil && s.calibration.positionEstimator.IsCalibrated() {
		wTL := s.calibration.positionEstimator.PixelToWorld(pixelTL[0], pixelTL[1])
		wBR := s.calibration.positionEstimator.PixelToWorld(pixelBR[0], pixelBR[1])
		worldTL := [2]float64{wTL.X, wTL.Y}
		worldBR := [2]float64{wBR.X, wBR.Y}
		// Normalize so TopLeft has min coords and BottomRight has max coords
		if worldTL[0] > worldBR[0] {
			worldTL[0], worldBR[0] = worldBR[0], worldTL[0]
		}
		if worldTL[1] > worldBR[1] {
			worldTL[1], worldBR[1] = worldBR[1], worldTL[1]
		}
		return worldTL, worldBR
	}
	// Fallback when not calibrated: use pixel coords directly
	return [2]float64{float64(pixelTL[0]), float64(pixelTL[1])},
		[2]float64{float64(pixelBR[0]), float64(pixelBR[1])}
}

func (s *WebServer) BroadcastPaths() {
	if s.Callbacks.OnPathsChanged == nil {
		utils.Debugf("BroadcastPaths: OnPathsChanged is nil, skipping")
		return
	}
	if s.calibration.positionEstimator == nil {
		utils.Debugf("BroadcastPaths: positionEstimator is nil, skipping")
		return
	}

	paths := s.Callbacks.OnPathsChanged()
	if len(paths) == 0 {
		s.BroadcastOverlay(OverlayMessage{
			Type:  "paths",
			Paths: &PathsMessage{Paths: []PathMessage{}},
		})
		return
	}

	utils.Debugf("BroadcastPaths: got %d paths, converting to pixels", len(paths))
	pathMessages := make([]PathMessage, 0, len(paths))
	for robotID, path := range paths {
		if len(path) == 0 {
			continue
		}

		pixels := make([][2]int, len(path))
		for i, wp := range path {
			px, py := s.calibration.positionEstimator.WorldToPixel(position.Point2D{X: wp[0], Y: wp[1]})
			pixels[i] = [2]int{px, py}
		}

		brightColors := []string{"#FFFF00", "#00FF00", "#FF00FF", "#00FFFF", "#FF6600", "#FF0066", "#66FF00", "#0099FF"}
		color := brightColors[robotID%len(brightColors)]
		pathMessages = append(pathMessages, PathMessage{
			RobotID: robotID,
			Points:  pixels,
			Color:   color,
		})
		utils.Debugf("BroadcastPaths: robot %d has %d waypoints, first pixel=(%d,%d)", robotID, len(pixels), pixels[0][0], pixels[0][1])
	}

	utils.Debugf("BroadcastPaths: broadcasting %d path messages via WebSocket", len(pathMessages))
	s.BroadcastOverlay(OverlayMessage{
		Type:  "paths",
		Paths: &PathsMessage{Paths: pathMessages},
	})
}

type CalibrationStatusMessage struct {
	State    string  `json:"state"`
	Message  string  `json:"message"`
	Filename string  `json:"filename"`
	TagSize  float64 `json:"tagSize"`
}

func (s *WebServer) handleCalibrationStatus(c *gin.Context) {
	s.calibration.mutex.RLock()
	state := s.calibration.state
	message := s.calibration.message
	filename := s.calibration.filename
	tagSize := s.calibration.tagSize
	s.calibration.mutex.RUnlock()

	resp := gin.H{
		"state":              state,
		"message":            message,
		"filename":           filename,
		"tagSize":            tagSize,
		"resolutionMismatch": false,
	}
	if pe := s.calibration.positionEstimator; pe != nil {
		resp["resolutionMismatch"] = pe.ResolutionMismatch()
		if w, h, ok := pe.CalibratedResolution(); ok {
			resp["calibratedResolution"] = [2]int{w, h}
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (s *WebServer) handleCalibrationStart(c *gin.Context) {
	s.SetCalibrationState("detecting", "Looking for the calibration tags...", "", position.TargetTagSize)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "state": "detecting"})
}

type DetectedTagInfo struct {
	ID      int           `json:"id"`
	Center  [2]float64    `json:"center"`
	Corners [4][2]float64 `json:"corners"`
}

// CalibrationTargetSpec describes the printable calibration target so the UI
// does not have to hardcode tag IDs, labels or sizes.
type CalibrationTargetSpec struct {
	TagSize float64              `json:"tagSize"`
	Tags    []position.TargetTag `json:"tags"`
}

func calibrationTargetSpec() CalibrationTargetSpec {
	return CalibrationTargetSpec{
		TagSize: position.TargetTagSize,
		Tags:    position.TargetTags(),
	}
}

type CalibrationDetectedTagsResponse struct {
	Tags        []DetectedTagInfo     `json:"tags"`
	Count       int                   `json:"count"`
	FrameWidth  int                   `json:"frameWidth"`
	FrameHeight int                   `json:"frameHeight"`
	Target      CalibrationTargetSpec `json:"target"`
}

func (s *WebServer) UpdateDetectedTags(tags []DetectedTagInfo, frameWidth, frameHeight int) {
	s.calibration.detectedTagsMut.Lock()
	s.calibration.detectedTags = tags
	s.calibration.detectedFrameW = frameWidth
	s.calibration.detectedFrameH = frameHeight
	s.calibration.lastTagUpdate = time.Now()
	s.calibration.detectedTagsMut.Unlock()
}

// calibrationPollWindow is how recently the calibration wizard must have
// polled for tags for the stream to count as "in calibration view".
const calibrationPollWindow = 3 * time.Second

// CalibrationViewActive reports whether the calibration wizard is open and
// polling for tags. While it is, the video stream is sent without the
// detection overlay so the user sees only the clean camera view. Deriving
// this from polling means it clears itself if the browser goes away.
func (s *WebServer) CalibrationViewActive() bool {
	s.calibration.detectedTagsMut.RLock()
	defer s.calibration.detectedTagsMut.RUnlock()
	return !s.calibration.lastTagPoll.IsZero() && time.Since(s.calibration.lastTagPoll) < calibrationPollWindow
}

func (s *WebServer) handleCalibrationDetectedTags(c *gin.Context) {
	s.calibration.detectedTagsMut.Lock()
	s.calibration.lastTagPoll = time.Now()
	s.calibration.detectedTagsMut.Unlock()

	s.calibration.detectedTagsMut.RLock()
	tags := s.calibration.detectedTags
	if time.Since(s.calibration.lastTagUpdate) > 2*time.Second {
		tags = nil
	}
	width, height := s.calibration.detectedFrameW, s.calibration.detectedFrameH
	s.calibration.detectedTagsMut.RUnlock()

	if tags == nil {
		tags = []DetectedTagInfo{}
	}
	c.JSON(http.StatusOK, CalibrationDetectedTagsResponse{
		Tags:        tags,
		Count:       len(tags),
		FrameWidth:  width,
		FrameHeight: height,
		Target:      calibrationTargetSpec(),
	})
}

type CalibrationTagCapture struct {
	ID      int           `json:"id"`
	Corners [4][2]float64 `json:"corners"`
}

type CalibrationComputeRequest struct {
	Tags []CalibrationTagCapture `json:"tags"`
}

type CalibrationComputeResponse struct {
	State      string                   `json:"state"`
	RMSCm      float64                  `json:"rmsCm"`
	MaxCm      float64                  `json:"maxCm"`
	QualityCm  float64                  `json:"qualityCm"`
	Rating     string                   `json:"rating,omitempty"`
	WorstTagID int                      `json:"worstTagId,omitempty"`
	PerTag     []position.TagFit        `json:"perTag,omitempty"`
	Checks     []position.DistanceCheck `json:"checks,omitempty"`
	Message    string                   `json:"message,omitempty"`
	Error      string                   `json:"error,omitempty"`
	Filename   string                   `json:"filename,omitempty"`
}

func worstTagLabel(fit *position.FitResult) string {
	for _, t := range fit.PerTag {
		if t.ID == fit.WorstTagID {
			return t.Label
		}
	}
	return ""
}

func (s *WebServer) handleCalibrationCompute(c *gin.Context) {
	var req CalibrationComputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CalibrationComputeResponse{State: "error", Error: err.Error()})
		return
	}

	captures := make([]position.TargetCapture, 0, len(req.Tags))
	for _, t := range req.Tags {
		capture := position.TargetCapture{ID: t.ID}
		for i, corner := range t.Corners {
			capture.Corners[i] = position.Point2D{X: corner[0], Y: corner[1]}
		}
		captures = append(captures, capture)
	}
	utils.Logf("Calibration compute: %d tags", len(captures))

	fit, err := position.FitTarget(captures)
	if err != nil {
		resp := CalibrationComputeResponse{State: "error", Error: err.Error()}
		if fit != nil {
			resp.RMSCm, resp.MaxCm, resp.QualityCm = fit.RMSCm, fit.MaxCm, fit.QualityCm
			resp.Rating, resp.WorstTagID, resp.PerTag, resp.Checks = fit.Rating, fit.WorstTagID, fit.PerTag, fit.Checks
			if errors.Is(err, position.ErrPoorFit) {
				resp.Error = fmt.Sprintf("The tags do not fit together as flat 15 cm squares (%.1f cm off). %s fits worst. Make sure every tag lies flat, is not curled, and was printed at exactly 15 cm, then try again. Nothing was saved.",
					fit.QualityCm, worstTagLabel(fit))
			}
		}
		utils.Logf("Calibration rejected: %v", err)
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	s.calibration.detectedTagsMut.RLock()
	frameW, frameH := s.calibration.detectedFrameW, s.calibration.detectedFrameH
	s.calibration.detectedTagsMut.RUnlock()
	if frameW <= 0 || frameH <= 0 {
		c.JSON(http.StatusBadRequest, CalibrationComputeResponse{State: "error", Error: "no camera frame has been seen yet"})
		return
	}

	utils.Logf("Homography fit: rms=%.2fcm max=%.2fcm quality=%.2fcm rating=%s H=%v",
		fit.RMSCm, fit.MaxCm, fit.QualityCm, fit.Rating, fit.Homography.H)

	cameraFile := GetCalibrationFilename(s.calibration.cameraName)
	cfg := position.NewCalibrationConfig(s.calibration.cameraName, frameW, frameH, fit, time.Now())
	if err := position.SaveCalibration(cameraFile, cfg); err != nil {
		utils.Logf("Failed to save calibration: %v", err)
		c.JSON(http.StatusInternalServerError, CalibrationComputeResponse{State: "error", Error: err.Error()})
		return
	}

	if s.Callbacks.OnCalibrationComplete != nil {
		s.Callbacks.OnCalibrationComplete(cameraFile)
	}

	message := fmt.Sprintf("Calibration saved: %.1f cm average error (%s).", fit.RMSCm, fit.Rating)
	if fit.Rating != position.RatingGood {
		message += fmt.Sprintf(" %s fits worst; make sure it lies flat and printed at 15 cm if accuracy matters.", worstTagLabel(fit))
	}
	s.SetCalibrationState("calibrated", message, cameraFile, position.TargetTagSize)

	c.JSON(http.StatusOK, CalibrationComputeResponse{
		State:      "calibrated",
		RMSCm:      fit.RMSCm,
		MaxCm:      fit.MaxCm,
		QualityCm:  fit.QualityCm,
		Rating:     fit.Rating,
		WorstTagID: fit.WorstTagID,
		PerTag:     fit.PerTag,
		Checks:     fit.Checks,
		Message:    message,
		Filename:   cameraFile,
	})
}

func GetCalibrationFilename(cameraName string) string {
	sanitized := sanitizeCameraName(cameraName)
	return fmt.Sprintf("config/calibration_%s.yaml", sanitized)
}

// GetForegroundStateFilename returns where the foreground detector's learned
// background is saved for cameraName, so a restart can restore it instead of
// re-learning.
func GetForegroundStateFilename(cameraName string) string {
	sanitized := sanitizeCameraName(cameraName)
	return fmt.Sprintf("config/foreground_bg_%s.bin", sanitized)
}

// GetObstaclesFilename returns the default per-camera obstacles file path
// (config/obstacles_<sanitized-camera-name>.yaml), or the camera-agnostic
// config/obstacles.yaml when cameraName is empty. Mirrors GetCalibrationFilename.
func GetObstaclesFilename(cameraName string) string {
	if cameraName == "" {
		return "config/obstacles.yaml"
	}
	return fmt.Sprintf("config/obstacles_%s.yaml", sanitizeCameraName(cameraName))
}

// ResolveObstaclesPath is the single source of truth for where the obstacles
// file lives, used identically by startup loading (RobotSystem.
// loadStaticObstacles), the position estimator's own obstacle copy
// (RobotSystem.initPositionEstimator), and the web UI's save/clear handlers
// (via GetObstaclesPath), so they can no longer drift out of sync (see
// docs/archived/code-review-2026-09-27.md).
//
// An explicitly configured path (ObstaclesConfig.File/Path, via GetPath())
// is honored only if it points to a file that actually exists -- this lets
// the checked-in tracking_config.yaml default (config/obstacles.yaml, which
// nothing has ever saved to) fall through to the real per-camera file
// instead of silently never loading/saving anything. Otherwise, the
// per-camera default from GetObstaclesFilename is used, so a fresh setup's
// first save lands exactly where later loads will look for it.
func ResolveObstaclesPath(configuredPath, cameraName string) string {
	if configuredPath != "" && utils.FileExists(configuredPath) {
		return configuredPath
	}
	return GetObstaclesFilename(cameraName)
}

func sanitizeCameraName(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := reg.ReplaceAllString(name, "_")
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		sanitized = "unknown"
	}
	return sanitized
}

func (s *WebServer) Start() {
	s.router.isRunning = true
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
	s.router.isRunning = false
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

func (s *WebServer) PushFrame(img image.Image) {
	if img == nil {
		return
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return
	}
	s.router.stream.updateJPEG(buf.Bytes())
}

func (s *WebServer) PushRawJPEG(jpegData []byte) {
	if len(jpegData) == 0 {
		return
	}
	s.router.stream.updateJPEG(jpegData)
}

func (s *WebServer) UpdateStats(tagCount int) {
	s.stats.mutex.Lock()
	s.stats.lastTagCount = tagCount
	s.stats.mutex.Unlock()
}

func (s *WebServer) SetArduinoConnected(connected bool) {
	s.stats.arduinoMutex.Lock()
	s.stats.arduinoConnected = connected
	s.stats.arduinoMutex.Unlock()
}

func (s *WebServer) BroadcastStatus(trackCount int, fps float64, uptimeSec float64) {
	s.broadcastStatus(trackCount, fps, uptimeSec, 0, false)
}

// BroadcastCameraStalled tells the UI that no frames are arriving. Status is
// normally sent from the frame loop, which is silent exactly when the camera
// has stalled, so a separate watchdog calls this instead.
func (s *WebServer) BroadcastCameraStalled(uptimeSec, frameAgeSec float64) {
	s.broadcastStatus(0, 0, uptimeSec, frameAgeSec, true)
}

func (s *WebServer) broadcastStatus(trackCount int, fps, uptimeSec, frameAgeSec float64, stalled bool) {
	s.stats.arduinoMutex.RLock()
	connected := s.stats.arduinoConnected
	s.stats.arduinoMutex.RUnlock()

	state := "Disconnected"
	if connected {
		state = "Connected"
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	hostMemMB := float64(memStats.Alloc) / 1024 / 1024

	s.stats.mutex.Lock()
	s.stats.lastFPS = fps
	s.stats.lastUptimeSec = uptimeSec
	s.stats.lastHostMemMB = hostMemMB
	s.stats.mutex.Unlock()

	s.BroadcastOverlay(OverlayMessage{
		Type: "status",
		Status: &StatusMessage{
			Connected:    s.router.isRunning,
			FPS:          fps,
			RobotCount:   trackCount,
			ArduinoState: state,
			HostMemoryMB: hostMemMB,
			UptimeSec:    uptimeSec,

			CameraStalled: stalled,
			FrameAgeSec:   frameAgeSec,
		},
	})
}

func (s *WebServer) handleCalibrationCancel(c *gin.Context) {
	s.SetCalibrationState("not_calibrated", "Click Settings to calibrate", "", 0)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "state": "cancelled"})
}

func (s *WebServer) GetObstaclesPath() string {
	return ResolveObstaclesPath(s.obstacles.path, s.calibration.cameraName)
}

// SetObstaclesPath sets the configured obstacles-file override (from
// ObstaclesConfig.GetPath()) that GetObstaclesPath/ResolveObstaclesPath
// consult before falling back to the per-camera default.
func (s *WebServer) SetObstaclesPath(path string) {
	s.obstacles.path = path
}

func (s *WebServer) handleObstaclesList(c *gin.Context) {
	s.obstacles.mutex.RLock()
	defer s.obstacles.mutex.RUnlock()

	obstacles := make([]ObstacleResponse, 0, len(s.obstacles.list))
	for _, obs := range s.obstacles.list {
		obstacles = append(obstacles, ObstacleResponse{
			ID:               obs.Name,
			Name:             obs.Name,
			PixelTopLeft:     obs.PixelsTopLeft,
			PixelBottomRight: obs.PixelsBottomRight,
			WorldTopLeft:     obs.WorldTopLeft,
			WorldBottomRight: obs.WorldBottomRight,
			Clearance:        0.02,
		})
	}

	c.JSON(http.StatusOK, ObstaclesListResponse{
		Obstacles: obstacles,
		Count:     len(obstacles),
		Saved:     s.obstacles.saved,
	})
}

func (s *WebServer) handleObstacleAdd(c *gin.Context) {
	var req AddObstacleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.obstacles.mutex.Lock()

	worldTL, worldBR := s.pixelCornersToWorld(req.PixelTopLeft, req.PixelBottomRight)

	newObs := planning.NewRectObstacle(uniqueObstacleName(s.obstacles.list, req.Name), worldTL, worldBR)
	newObs.PixelsTopLeft = req.PixelTopLeft
	newObs.PixelsBottomRight = req.PixelBottomRight

	s.obstacles.list = append(s.obstacles.list, newObs)
	s.obstacles.saved = false
	count := len(s.obstacles.list)

	s.obstacles.mutex.Unlock()
	s.notifyObstaclesChanged()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": newObs.Name, "count": count})
}

func (s *WebServer) handleObstacleDelete(c *gin.Context) {
	id := c.Param("id")

	s.obstacles.mutex.Lock()

	newObs := make([]planning.Obstacle, 0, len(s.obstacles.list))
	for _, obs := range s.obstacles.list {
		if obs.Name != id {
			newObs = append(newObs, obs)
		}
	}
	s.obstacles.list = newObs
	s.obstacles.saved = false
	count := len(newObs)

	s.obstacles.mutex.Unlock()
	s.notifyObstaclesChanged()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": id, "count": count})
}

func (s *WebServer) handleObstacleUpdate(c *gin.Context) {
	id := c.Param("id")
	var req UpdateObstacleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.obstacles.mutex.Lock()

	worldTL, worldBR := s.pixelCornersToWorld(req.PixelTopLeft, req.PixelBottomRight)

	for i, obs := range s.obstacles.list {
		if obs.Name == id {
			updated := planning.NewRectObstacle(id, worldTL, worldBR)
			updated.PixelsTopLeft = req.PixelTopLeft
			updated.PixelsBottomRight = req.PixelBottomRight
			s.obstacles.list[i] = updated
			break
		}
	}
	s.obstacles.saved = false

	s.obstacles.mutex.Unlock()
	s.notifyObstaclesChanged()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": id})
}

func (s *WebServer) handleObstaclesClear(c *gin.Context) {
	s.obstacles.mutex.Lock()

	s.obstacles.list = make([]planning.Obstacle, 0)
	s.obstacles.saved = false

	s.obstacles.mutex.Unlock()
	s.notifyObstaclesChanged()

	// Persist cleared state to disk so obstacles don't return on restart
	path := s.GetObstaclesPath()
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0750); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if err := s.saveObstaclesToFile(path, []planning.Obstacle{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.obstacles.mutex.Lock()
	s.obstacles.saved = true
	s.obstacles.mutex.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "count": 0})
}

func (s *WebServer) handleObstaclesSave(c *gin.Context) {
	s.obstacles.mutex.RLock()
	obstacles := s.obstacles.list
	s.obstacles.mutex.RUnlock()

	path := s.GetObstaclesPath()
	dir := filepath.Dir(path)
	// #nosec G304
	if err := os.MkdirAll(dir, 0750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := s.saveObstaclesToFile(path, obstacles); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.obstacles.mutex.Lock()
	s.obstacles.saved = true
	s.obstacles.mutex.Unlock()

	s.BroadcastObstacles()

	c.JSON(http.StatusOK, SaveObstaclesResponse{
		Success: true,
		Message: fmt.Sprintf("Saved %d obstacles to %s", len(obstacles), path),
	})
}

// obstacleFileEntry and obstacleFile mirror the on-disk obstacle YAML shape
// that PositionEstimator.LoadObstacles parses generically (keys "version",
// "obstacles[].name", ".pixels.top_left"/".bottom_right",
// ".world.top_left"/".bottom_right"). Marshaling through these tagged
// structs, rather than building the YAML text by hand, keeps escaping and
// nesting correct by construction.
type obstacleFileEntry struct {
	Name   string `yaml:"name"`
	Pixels struct {
		TopLeft     [2]int `yaml:"top_left"`
		BottomRight [2]int `yaml:"bottom_right"`
	} `yaml:"pixels"`
	World struct {
		TopLeft     [2]float64 `yaml:"top_left"`
		BottomRight [2]float64 `yaml:"bottom_right"`
	} `yaml:"world"`
}

type obstacleFile struct {
	Version   int                 `yaml:"version"`
	Obstacles []obstacleFileEntry `yaml:"obstacles"`
}

func (s *WebServer) saveObstaclesToFile(path string, obstacles []planning.Obstacle) error {
	file := obstacleFile{Version: 1, Obstacles: make([]obstacleFileEntry, len(obstacles))}
	for i, obs := range obstacles {
		entry := obstacleFileEntry{Name: obs.Name}
		entry.Pixels.TopLeft = obs.PixelsTopLeft
		entry.Pixels.BottomRight = obs.PixelsBottomRight
		entry.World.TopLeft = obs.WorldTopLeft
		entry.World.BottomRight = obs.WorldBottomRight
		file.Obstacles[i] = entry
	}

	data, err := yaml.Marshal(file)
	if err != nil {
		return fmt.Errorf("failed to marshal obstacles: %w", err)
	}

	// #nosec G304
	// #nosec G306
	return os.WriteFile(path, data, 0600)
}

func (s *WebServer) GetObstacles() []planning.Obstacle {
	s.obstacles.mutex.RLock()
	defer s.obstacles.mutex.RUnlock()
	return s.obstacles.list
}

func (s *WebServer) SetObstacles(obstacles []planning.Obstacle) {
	s.obstacles.mutex.Lock()
	s.obstacles.list = obstacles
	s.obstacles.mutex.Unlock()

	if s.Callbacks.OnObstaclesChanged != nil {
		s.Callbacks.OnObstaclesChanged(obstacles)
	}
}

func (s *WebServer) GetAllObstacles() []planning.Obstacle {
	s.obstacles.mutex.RLock()
	defer s.obstacles.mutex.RUnlock()
	return s.obstacles.list
}

type ModeRequest struct {
	Mode string `json:"mode"`
}

type ControlStateResponse struct {
	Mode             string `json:"mode"`
	EmergencyStopped bool   `json:"emergency_stopped"`
}

func (s *WebServer) handleSetMode(c *gin.Context) {
	var req ModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Mode != "hold" && req.Mode != "manual" && req.Mode != "autonomous" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mode, must be hold/manual/autonomous"})
		return
	}
	if s.Callbacks.OnModeChange != nil {
		if err := s.Callbacks.OnModeChange(req.Mode); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "mode": req.Mode})
}

func (s *WebServer) handleEmergencyStop(c *gin.Context) {
	if s.Callbacks.OnEmergencyStop != nil {
		s.Callbacks.OnEmergencyStop()
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "emergency_stopped": true})
}

func (s *WebServer) handleClearEmergencyStop(c *gin.Context) {
	if s.Callbacks.OnClearEmergencyStop != nil {
		if err := s.Callbacks.OnClearEmergencyStop(); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "emergency_stopped": false})
}

func (s *WebServer) handleControlState(c *gin.Context) {
	mode := "hold"
	eStopped := false
	if s.Callbacks.OnGetControlState != nil {
		mode, eStopped = s.Callbacks.OnGetControlState()
	}
	c.JSON(http.StatusOK, ControlStateResponse{
		Mode:             mode,
		EmergencyStopped: eStopped,
	})
}
