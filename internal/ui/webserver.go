//go:build gocv

package ui

import (
	"bytes"
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
)

type WebServer struct {
	addr          string
	engine        *gin.Engine
	stream        *mjpegStream
	clients       map[*websocket.Conn]*sync.Mutex
	clientMutex   sync.RWMutex
	isRunning     bool
	stopChan      chan struct{}
	lastTagCount  int
	lastYoloCount int
	lastFPS       float64
	lastUptimeSec float64
	lastHostMemMB float64
	statsMutex    sync.RWMutex

	calibrationMutex    sync.RWMutex
	calibrationState    string
	calibrationMessage  string
	calibrationFilename string
	calibrationTagSize  float64
	cameraName          string

	arduinoConnected bool
	arduinoMutex     sync.RWMutex

	detectedTags    []DetectedTagInfo
	detectedFrameW  int
	detectedFrameH  int
	lastTagPoll     time.Time
	detectedTagsMut sync.RWMutex
	lastTagUpdate   time.Time

	obstaclesMutex sync.RWMutex
	obstacles      []planning.Obstacle
	obstaclesSaved bool
	obstaclesPath  string

	destinationMutex sync.RWMutex
	destination      DestinationMessage

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

	OnPathsChanged    func() map[int][][2]float64
	positionEstimator *position.PositionEstimator
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
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewWebServer(addr string) *WebServer {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(corsMiddleware())

	server := &WebServer{
		addr:      addr,
		engine:    engine,
		stream:    newMJPEGStream(),
		clients:   make(map[*websocket.Conn]*sync.Mutex),
		stopChan:  make(chan struct{}),
		isRunning: false,
	}

	server.setupRoutes()
	return server
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	}
}

func (s *WebServer) setupRoutes() {
	staticFS, err := fs.Sub(StaticFiles, "static")
	if err != nil {
		utils.Logf("Warning: Failed to create static FS sub-directory: %v", err)
	} else {
		s.engine.GET("/assets/*path", gin.WrapH(http.FileServer(http.FS(staticFS))))
		s.engine.GET("/calibration-tags/*path", gin.WrapH(http.FileServer(http.FS(staticFS))))
	}
	s.engine.GET("/", s.handleIndex)
	s.engine.GET("/stream", s.handleMJPEG)
	s.engine.GET("/ws", s.handleWebSocket)
	s.engine.POST("/api/command", s.handleCommand)
	s.engine.POST("/api/destination", s.handleDestination)
	s.engine.DELETE("/api/destination", s.handleDestinationClear)
	s.engine.GET("/api/status", s.handleStatus)
	s.engine.GET("/api/obstacles", s.handleObstaclesList)
	s.engine.POST("/api/obstacles", s.handleObstacleAdd)
	s.engine.DELETE("/api/obstacles/:id", s.handleObstacleDelete)
	s.engine.PUT("/api/obstacles/:id", s.handleObstacleUpdate)
	s.engine.POST("/api/obstacles/clear", s.handleObstaclesClear)
	s.engine.POST("/api/obstacles/save", s.handleObstaclesSave)
	s.engine.GET("/api/calibration/status", s.handleCalibrationStatus)
	s.engine.POST("/api/calibration/start", s.handleCalibrationStart)
	s.engine.GET("/api/calibration/detected-tags", s.handleCalibrationDetectedTags)
	s.engine.POST("/api/calibration/compute", s.handleCalibrationCompute)
	s.engine.POST("/api/calibration/cancel", s.handleCalibrationCancel)
	s.engine.GET("/api/foreground/state", s.handleForegroundState)
	s.engine.POST("/api/foreground/enabled", s.handleForegroundEnabled)
	s.engine.POST("/api/foreground/apply", s.handleForegroundApply)
	s.engine.POST("/api/foreground/reset", s.handleForegroundReset)
	s.engine.POST("/api/foreground/absorb", s.handleForegroundAbsorb)
	s.engine.GET("/api/foreground/debug.jpg", s.handleForegroundDebug)
	s.engine.POST("/api/mode", s.handleSetMode)
	s.engine.POST("/api/emergency-stop", s.handleEmergencyStop)
	s.engine.POST("/api/clear-emergency-stop", s.handleClearEmergencyStop)
	s.engine.GET("/api/control-state", s.handleControlState)
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
	s.stream.serveHTTP(c.Writer, c.Request)
}

func (s *WebServer) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	s.clientMutex.Lock()
	s.clients[conn] = &sync.Mutex{}
	s.clientMutex.Unlock()

	go s.wsReader(conn)
}

func (s *WebServer) wsReader(conn *websocket.Conn) {
	defer func() {
		_ = conn.Close()
		s.clientMutex.Lock()
		delete(s.clients, conn)
		s.clientMutex.Unlock()
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
	s.clientMutex.RLock()
	clients := make(map[*websocket.Conn]*sync.Mutex, len(s.clients))
	for conn, mu := range s.clients {
		clients[conn] = mu
	}
	s.clientMutex.RUnlock()

	for conn, mu := range clients {
		mu.Lock()
		_ = conn.WriteJSON(msg)
		mu.Unlock()
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
	if s.ForegroundState != nil {
		state = s.ForegroundState()
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
	if s.OnForegroundEnabled != nil {
		s.OnForegroundEnabled(*req.Enabled)
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
	if s.OnForegroundApply != nil {
		s.OnForegroundApply(*req.Apply)
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "apply": *req.Apply})
}

func (s *WebServer) handleForegroundReset(c *gin.Context) {
	if s.OnForegroundReset != nil {
		s.OnForegroundReset()
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
	if s.OnForegroundAbsorb != nil {
		s.OnForegroundAbsorb(int(*req.X), int(*req.Y))
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleForegroundDebug serves the detector's debug view. Asking for it makes
// the detector render it for the next couple of seconds.
func (s *WebServer) handleForegroundDebug(c *gin.Context) {
	var jpeg []byte
	if s.ForegroundDebugJPEG != nil {
		jpeg = s.ForegroundDebugJPEG()
	}
	if len(jpeg) == 0 {
		c.Status(http.StatusNoContent)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/jpeg", jpeg)
}

func (s *WebServer) BroadcastObstacles() {
	s.obstaclesMutex.RLock()
	defer s.obstaclesMutex.RUnlock()

	obstacles := make([]ObstacleResponse, 0, len(s.obstacles))
	for _, obs := range s.obstacles {
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
	if s.OnCommand != nil {
		if err := s.OnCommand(req.Command); err != nil {
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

	s.obstaclesMutex.RLock()
	obstacles := s.obstacles
	s.obstaclesMutex.RUnlock()

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

	s.destinationMutex.Lock()
	s.destination = DestinationMessage{
		RobotID: req.RobotID,
		X:       req.X,
		Y:       req.Y,
		Valid:   true,
	}
	s.destinationMutex.Unlock()

	if s.OnDestinationSet != nil {
		s.OnDestinationSet(req.RobotID, [2]float64{float64(req.X), float64(req.Y)})
	}

	s.BroadcastOverlay(OverlayMessage{
		Type:        "destination",
		Destination: &s.destination,
	})

	c.JSON(http.StatusOK, gin.H{"status": "ok", "destination": req})
}

func (s *WebServer) ClearDestination(robotID int) {
	s.destinationMutex.Lock()
	s.destination = DestinationMessage{Valid: false}
	s.destinationMutex.Unlock()

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

	if s.OnDestinationClear != nil {
		s.OnDestinationClear(req.RobotID)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "robot_id": req.RobotID})
}

func (s *WebServer) handleStatus(c *gin.Context) {
	s.statsMutex.RLock()
	tagCount := s.lastTagCount
	yoloCount := s.lastYoloCount
	fps := s.lastFPS
	uptimeSec := s.lastUptimeSec
	hostMemMB := s.lastHostMemMB
	s.statsMutex.RUnlock()

	s.arduinoMutex.RLock()
	connected := s.arduinoConnected
	s.arduinoMutex.RUnlock()

	state := "Disconnected"
	if connected {
		state = "Connected"
	}

	c.JSON(http.StatusOK, gin.H{
		"connected":    s.isRunning,
		"fps":          fps,
		"robotCount":   tagCount,
		"tagCount":     tagCount,
		"yoloCount":    yoloCount,
		"arduinoState": state,
		"hostMemoryMB": hostMemMB,
		"uptimeSec":    uptimeSec,
	})
}

func (s *WebServer) SetCameraName(name string) {
	s.cameraName = name
}

func (s *WebServer) SetCalibrationState(state, message, filename string, tagSize float64) {
	s.calibrationMutex.Lock()
	s.calibrationState = state
	s.calibrationMessage = message
	s.calibrationFilename = filename
	s.calibrationTagSize = tagSize
	s.calibrationMutex.Unlock()

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
	s.positionEstimator = pe
}

func (s *WebServer) pixelCornersToWorld(pixelTL, pixelBR [2]int) ([2]float64, [2]float64) {
	if s.positionEstimator != nil && s.positionEstimator.IsCalibrated() {
		wTL := s.positionEstimator.PixelToWorld(pixelTL[0], pixelTL[1])
		wBR := s.positionEstimator.PixelToWorld(pixelBR[0], pixelBR[1])
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
	if s.OnPathsChanged == nil {
		utils.Debugf("BroadcastPaths: OnPathsChanged is nil, skipping")
		return
	}
	if s.positionEstimator == nil {
		utils.Debugf("BroadcastPaths: positionEstimator is nil, skipping")
		return
	}

	paths := s.OnPathsChanged()
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
			px, py := s.positionEstimator.WorldToPixel(position.Point2D{X: wp[0], Y: wp[1]})
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
	s.calibrationMutex.RLock()
	state := s.calibrationState
	message := s.calibrationMessage
	filename := s.calibrationFilename
	tagSize := s.calibrationTagSize
	s.calibrationMutex.RUnlock()

	resp := gin.H{
		"state":              state,
		"message":            message,
		"filename":           filename,
		"tagSize":            tagSize,
		"resolutionMismatch": false,
	}
	if pe := s.positionEstimator; pe != nil {
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
	s.detectedTagsMut.Lock()
	s.detectedTags = tags
	s.detectedFrameW = frameWidth
	s.detectedFrameH = frameHeight
	s.lastTagUpdate = time.Now()
	s.detectedTagsMut.Unlock()
}

// calibrationPollWindow is how recently the calibration wizard must have
// polled for tags for the stream to count as "in calibration view".
const calibrationPollWindow = 3 * time.Second

// CalibrationViewActive reports whether the calibration wizard is open and
// polling for tags. While it is, the video stream is sent without the
// detection overlay so the user sees only the clean camera view. Deriving
// this from polling means it clears itself if the browser goes away.
func (s *WebServer) CalibrationViewActive() bool {
	s.detectedTagsMut.RLock()
	defer s.detectedTagsMut.RUnlock()
	return !s.lastTagPoll.IsZero() && time.Since(s.lastTagPoll) < calibrationPollWindow
}

func (s *WebServer) handleCalibrationDetectedTags(c *gin.Context) {
	s.detectedTagsMut.Lock()
	s.lastTagPoll = time.Now()
	s.detectedTagsMut.Unlock()

	s.detectedTagsMut.RLock()
	tags := s.detectedTags
	if time.Since(s.lastTagUpdate) > 2*time.Second {
		tags = nil
	}
	width, height := s.detectedFrameW, s.detectedFrameH
	s.detectedTagsMut.RUnlock()

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

	s.detectedTagsMut.RLock()
	frameW, frameH := s.detectedFrameW, s.detectedFrameH
	s.detectedTagsMut.RUnlock()
	if frameW <= 0 || frameH <= 0 {
		c.JSON(http.StatusBadRequest, CalibrationComputeResponse{State: "error", Error: "no camera frame has been seen yet"})
		return
	}

	utils.Logf("Homography fit: rms=%.2fcm max=%.2fcm quality=%.2fcm rating=%s H=%v",
		fit.RMSCm, fit.MaxCm, fit.QualityCm, fit.Rating, fit.Homography.H)

	cameraFile := GetCalibrationFilename(s.cameraName)
	cfg := position.NewCalibrationConfig(s.cameraName, frameW, frameH, fit, time.Now())
	if err := position.SaveCalibration(cameraFile, cfg); err != nil {
		utils.Logf("Failed to save calibration: %v", err)
		c.JSON(http.StatusInternalServerError, CalibrationComputeResponse{State: "error", Error: err.Error()})
		return
	}

	if s.OnCalibrationComplete != nil {
		s.OnCalibrationComplete(cameraFile)
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
	s.isRunning = true
	go func() {
		srv := &http.Server{
			Addr:              s.addr,
			Handler:           s.engine,
			ReadHeaderTimeout: 5 * time.Second,
		}
		if err := srv.ListenAndServe(); err != nil && !strings.Contains(err.Error(), "Server closed") {
			utils.Logf("HTTP server error: %v", err)
		}
	}()
}

func (s *WebServer) Stop() {
	s.isRunning = false
	close(s.stopChan)
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
	s.stream.updateJPEG(buf.Bytes())
}

func (s *WebServer) PushRawJPEG(jpegData []byte) {
	if len(jpegData) == 0 {
		return
	}
	s.stream.updateJPEG(jpegData)
}

func (s *WebServer) UpdateStats(tagCount, yoloCount int) {
	s.statsMutex.Lock()
	s.lastTagCount = tagCount
	s.lastYoloCount = yoloCount
	s.statsMutex.Unlock()
}

func (s *WebServer) SetArduinoConnected(connected bool) {
	s.arduinoMutex.Lock()
	s.arduinoConnected = connected
	s.arduinoMutex.Unlock()
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
	s.arduinoMutex.RLock()
	connected := s.arduinoConnected
	s.arduinoMutex.RUnlock()

	state := "Disconnected"
	if connected {
		state = "Connected"
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	hostMemMB := float64(memStats.Alloc) / 1024 / 1024

	s.statsMutex.Lock()
	s.lastFPS = fps
	s.lastUptimeSec = uptimeSec
	s.lastHostMemMB = hostMemMB
	s.statsMutex.Unlock()

	s.BroadcastOverlay(OverlayMessage{
		Type: "status",
		Status: &StatusMessage{
			Connected:    s.isRunning,
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
	if s.obstaclesPath != "" {
		return s.obstaclesPath
	}
	if s.cameraName != "" {
		sanitized := sanitizeCameraName(s.cameraName)
		return fmt.Sprintf("config/obstacles_%s.yaml", sanitized)
	}
	return "config/obstacles.yaml"
}

func (s *WebServer) SetObstaclesPath(path string) {
	s.obstaclesPath = path
}

func (s *WebServer) handleObstaclesList(c *gin.Context) {
	s.obstaclesMutex.RLock()
	defer s.obstaclesMutex.RUnlock()

	obstacles := make([]ObstacleResponse, 0, len(s.obstacles))
	for _, obs := range s.obstacles {
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
		Saved:     s.obstaclesSaved,
	})
}

func (s *WebServer) handleObstacleAdd(c *gin.Context) {
	var req AddObstacleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.obstaclesMutex.Lock()

	worldTL, worldBR := s.pixelCornersToWorld(req.PixelTopLeft, req.PixelBottomRight)

	newObs := planning.Obstacle{
		Name:              fmt.Sprintf("obstacle_%d", len(s.obstacles)+1),
		PixelsTopLeft:     req.PixelTopLeft,
		PixelsBottomRight: req.PixelBottomRight,
		WorldTopLeft:      worldTL,
		WorldBottomRight:  worldBR,
	}

	if req.Name != "" {
		newObs.Name = req.Name
	}

	s.obstacles = append(s.obstacles, newObs)
	s.obstaclesSaved = false

	s.obstaclesMutex.Unlock()
	s.BroadcastObstacles()

	if s.OnObstaclesChanged != nil {
		s.obstaclesMutex.RLock()
		obstacles := s.obstacles
		s.obstaclesMutex.RUnlock()
		s.OnObstaclesChanged(obstacles)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": newObs.Name, "count": len(s.obstacles)})
}

func (s *WebServer) handleObstacleDelete(c *gin.Context) {
	id := c.Param("id")

	s.obstaclesMutex.Lock()

	newObs := make([]planning.Obstacle, 0, len(s.obstacles))
	for _, obs := range s.obstacles {
		if obs.Name != id {
			newObs = append(newObs, obs)
		}
	}
	s.obstacles = newObs
	s.obstaclesSaved = false

	s.obstaclesMutex.Unlock()
	s.BroadcastObstacles()

	if s.OnObstaclesChanged != nil {
		s.obstaclesMutex.RLock()
		obstacles := s.obstacles
		s.obstaclesMutex.RUnlock()
		s.OnObstaclesChanged(obstacles)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": id, "count": len(s.obstacles)})
}

func (s *WebServer) handleObstacleUpdate(c *gin.Context) {
	id := c.Param("id")
	var req UpdateObstacleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.obstaclesMutex.Lock()

	worldTL, worldBR := s.pixelCornersToWorld(req.PixelTopLeft, req.PixelBottomRight)

	for i, obs := range s.obstacles {
		if obs.Name == id {
			s.obstacles[i] = planning.Obstacle{
				Name:              id,
				PixelsTopLeft:     req.PixelTopLeft,
				PixelsBottomRight: req.PixelBottomRight,
				WorldTopLeft:      worldTL,
				WorldBottomRight:  worldBR,
			}
			break
		}
	}
	s.obstaclesSaved = false

	s.obstaclesMutex.Unlock()
	s.BroadcastObstacles()

	if s.OnObstaclesChanged != nil {
		s.obstaclesMutex.RLock()
		obstacles := s.obstacles
		s.obstaclesMutex.RUnlock()
		s.OnObstaclesChanged(obstacles)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": id})
}

func (s *WebServer) handleObstaclesClear(c *gin.Context) {
	s.obstaclesMutex.Lock()

	s.obstacles = make([]planning.Obstacle, 0)
	s.obstaclesSaved = false

	s.obstaclesMutex.Unlock()
	s.BroadcastObstacles()

	if s.OnObstaclesChanged != nil {
		s.OnObstaclesChanged([]planning.Obstacle{})
	}

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

	s.obstaclesMutex.Lock()
	s.obstaclesSaved = true
	s.obstaclesMutex.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "count": 0})
}

func (s *WebServer) handleObstaclesSave(c *gin.Context) {
	s.obstaclesMutex.RLock()
	obstacles := s.obstacles
	s.obstaclesMutex.RUnlock()

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

	s.obstaclesMutex.Lock()
	s.obstaclesSaved = true
	s.obstaclesMutex.Unlock()

	s.BroadcastObstacles()

	c.JSON(http.StatusOK, SaveObstaclesResponse{
		Success: true,
		Message: fmt.Sprintf("Saved %d obstacles to %s", len(obstacles), path),
	})
}

func (s *WebServer) saveObstaclesToFile(path string, obstacles []planning.Obstacle) error {
	yamlContent := "version: 1\nobstacles:\n"

	for i, obs := range obstacles {
		yamlContent += fmt.Sprintf("  - name: %q\n", obs.Name)
		yamlContent += "    pixels:\n"
		yamlContent += fmt.Sprintf("      top_left: [%d, %d]\n", obs.PixelsTopLeft[0], obs.PixelsTopLeft[1])
		yamlContent += fmt.Sprintf("      bottom_right: [%d, %d]\n", obs.PixelsBottomRight[0], obs.PixelsBottomRight[1])
		yamlContent += "    world:\n"
		yamlContent += fmt.Sprintf("      top_left: [%.4f, %.4f]\n", obs.WorldTopLeft[0], obs.WorldTopLeft[1])
		yamlContent += fmt.Sprintf("      bottom_right: [%.4f, %.4f]\n", obs.WorldBottomRight[0], obs.WorldBottomRight[1])
		if i < len(obstacles)-1 {
			yamlContent += "\n"
		}
	}

	// #nosec G304
	// #nosec G306
	return os.WriteFile(path, []byte(yamlContent), 0600)
}

func (s *WebServer) GetObstacles() []planning.Obstacle {
	s.obstaclesMutex.RLock()
	defer s.obstaclesMutex.RUnlock()
	return s.obstacles
}

func (s *WebServer) SetObstacles(obstacles []planning.Obstacle) {
	s.obstaclesMutex.Lock()
	s.obstacles = obstacles
	s.obstaclesMutex.Unlock()

	if s.OnObstaclesChanged != nil {
		s.OnObstaclesChanged(obstacles)
	}
}

func (s *WebServer) GetAllObstacles() []planning.Obstacle {
	s.obstaclesMutex.RLock()
	defer s.obstaclesMutex.RUnlock()
	return s.obstacles
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
	if s.OnModeChange != nil {
		if err := s.OnModeChange(req.Mode); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "mode": req.Mode})
}

func (s *WebServer) handleEmergencyStop(c *gin.Context) {
	if s.OnEmergencyStop != nil {
		s.OnEmergencyStop()
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "emergency_stopped": true})
}

func (s *WebServer) handleClearEmergencyStop(c *gin.Context) {
	if s.OnClearEmergencyStop != nil {
		if err := s.OnClearEmergencyStop(); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "emergency_stopped": false})
}

func (s *WebServer) handleControlState(c *gin.Context) {
	mode := "hold"
	eStopped := false
	if s.OnGetControlState != nil {
		mode, eStopped = s.OnGetControlState()
	}
	c.JSON(http.StatusOK, ControlStateResponse{
		Mode:             mode,
		EmergencyStopped: eStopped,
	})
}
