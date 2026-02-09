//go:build gocv

package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hybridgroup/mjpeg"
	"gopkg.in/yaml.v3"
	"robot_tracker_go/internal/planning"
	"robot_tracker_go/internal/position"
	"robot_tracker_go/internal/tracking"
)

type WebServer struct {
	addr          string
	engine        *gin.Engine
	stream        *mjpeg.Stream
	wsUpgrader    websocket.Upgrader
	clients       map[*websocket.Conn]bool
	clientMutex   sync.RWMutex
	isRunning     bool
	stopChan      chan struct{}
	lastTagCount  int
	lastYoloCount int
	statsMutex    sync.RWMutex

	calibrationMutex    sync.RWMutex
	calibrationState    string
	calibrationMessage  string
	calibrationFilename string
	calibrationTagSize  float64
	calibrationData     *CalibrationSaveRequest
	cameraName          string

	detectedTags    []DetectedTagInfo
	detectedTagsMut sync.RWMutex
	lastTagUpdate   time.Time

	obstaclesMutex sync.RWMutex
	obstacles      []planning.Obstacle
	obstaclesSaved bool
	obstaclesPath  string

	OnObstaclesChanged func([]planning.Obstacle)
}

type OverlayMessage struct {
	Type        string                    `json:"type"`
	BBox        *BBoxMessage              `json:"bbox,omitempty"`
	Track       *TrackMessage             `json:"track,omitempty"`
	Tracks      *TracksMessage            `json:"tracks,omitempty"`
	Path        *PathMessage              `json:"path,omitempty"`
	Status      *StatusMessage            `json:"status,omitempty"`
	Command     *CommandMessage           `json:"command,omitempty"`
	Calibration *CalibrationStatusMessage `json:"calibration,omitempty"`
	Obstacles   *ObstaclesMessage         `json:"obstacles,omitempty"`
}

type TracksMessage struct {
	Tracks []TrackMessage `json:"tracks"`
}

type ObstaclesMessage struct {
	Obstacles []ObstacleResponse `json:"obstacles"`
	Count     int                `json:"count"`
}

type BBoxMessage struct {
	X1, Y1, X2, Y2 int     `json:"x1,y1,x2,y2"`
	Label          string  `json:"label"`
	Color          string  `json:"color"`
	Confidence     float64 `json:"confidence"`
}

type TrackMessage struct {
	ID          int      `json:"id"`
	BBox        []int    `json:"bbox"`
	History     [][2]int `json:"history"`
	Color       string   `json:"color"`
	Confidence  float64  `json:"confidence"`
	State       string   `json:"state"`
	PixelRadius *float64 `json:"pixel_radius,omitempty"`
}

type PathMessage struct {
	Points [][2]int `json:"points"`
	Color  string   `json:"color"`
}

type StatusMessage struct {
	Connected    bool    `json:"connected"`
	FPS          float64 `json:"fps"`
	RobotCount   int     `json:"robotCount"`
	ArduinoState string  `json:"arduinoState"`
}

type CommandMessage struct {
	Action string `json:"action"`
}

type DestinationRequest struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type CommandRequest struct {
	Command string `json:"command"`
}

type CalibrationStartRequest struct {
	TagSize float64 `json:"tagSize"`
}

type CalibrationDetectRequest struct {
	TagID   int           `json:"tagId"`
	Corners [4][2]float64 `json:"corners"`
}

type CalibrationSaveRequest struct {
	ComputedWidth  float64     `json:"computedWidth"`
	ComputedHeight float64     `json:"computedHeight"`
	PixelsPerMeter float64     `json:"pixelsPerMeter"`
	Homography     [][]float64 `json:"homography"`
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
		stream:    mjpeg.NewStream(),
		clients:   make(map[*websocket.Conn]bool),
		stopChan:  make(chan struct{}),
		isRunning: false,
	}

	server.setupRoutes()
	return server
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
		log.Printf("Warning: Failed to create static FS sub-directory: %v", err)
	} else {
		s.engine.GET("/assets/*path", gin.WrapH(http.FileServer(http.FS(staticFS))))
	}
	s.engine.GET("/", s.handleIndex)
	s.engine.GET("/stream", s.handleMJPEG)
	s.engine.GET("/ws", s.handleWebSocket)
	s.engine.POST("/api/command", s.handleCommand)
	s.engine.POST("/api/destination", s.handleDestination)
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
	s.stream.ServeHTTP(c.Writer, c.Request)
}

func (s *WebServer) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	s.clientMutex.Lock()
	s.clients[conn] = true
	s.clientMutex.Unlock()

	go s.wsReader(conn)
}

func (s *WebServer) wsReader(conn *websocket.Conn) {
	defer func() {
		conn.Close()
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
	s.clientMutex.RLock()
	for client := range s.clients {
		client.WriteJSON(msg)
	}
	s.clientMutex.RUnlock()
}

func (s *WebServer) BroadcastOverlay(msg OverlayMessage) {
	s.clientMutex.RLock()
	for client := range s.clients {
		client.WriteJSON(msg)
	}
	s.clientMutex.RUnlock()
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

func (s *WebServer) BroadcastTracks(tracks []tracking.Track) {
	trackMessages := make([]TrackMessage, 0, len(tracks))
	for _, track := range tracks {
		bbox := []int{track.Bbox[0], track.Bbox[1], track.Bbox[2], track.Bbox[3]}
		history := make([][2]int, len(track.History))
		for i, hp := range track.History {
			history[i] = [2]int{hp.Bbox[0], hp.Bbox[1]}
		}
		msg := TrackMessage{
			ID:          track.TrackID,
			BBox:        bbox,
			History:     history,
			Confidence:  track.Confidence,
			State:       track.StateString(),
			PixelRadius: &track.PixelRadius,
		}
		if track.PixelRadius <= 0 {
			msg.PixelRadius = nil
		}
		trackMessages = append(trackMessages, msg)
	}
	log.Printf("BroadcastTracks: %d tracks, %d with pixel_radius", len(trackMessages), len(trackMessages))
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
	c.JSON(http.StatusOK, gin.H{"status": "ok", "command": req.Command})
}

func (s *WebServer) handleDestination(c *gin.Context) {
	var req DestinationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "destination": req})
}

func (s *WebServer) handleStatus(c *gin.Context) {
	s.statsMutex.RLock()
	tagCount := s.lastTagCount
	yoloCount := s.lastYoloCount
	s.statsMutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"connected":    s.isRunning,
		"fps":          30.0,
		"robotCount":   tagCount,
		"tagCount":     tagCount,
		"yoloCount":    yoloCount,
		"arduinoState": "disconnected",
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

	c.JSON(http.StatusOK, gin.H{
		"state":    state,
		"message":  message,
		"filename": filename,
		"tagSize":  tagSize,
	})
}

func (s *WebServer) handleCalibrationStart(c *gin.Context) {
	var req CalibrationStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tagSize := req.TagSize
	if tagSize <= 0 {
		tagSize = 0.15
	}

	s.SetCalibrationState("detecting", "Looking for AprilTags...", "", tagSize)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "state": "detecting"})
}

type DetectedTagInfo struct {
	ID      int           `json:"id"`
	Center  [2]float64    `json:"center"`
	Corners [4][2]float64 `json:"corners"`
}

type CalibrationDetectedTagsResponse struct {
	Tags  []DetectedTagInfo `json:"tags"`
	Count int               `json:"count"`
}

func (s *WebServer) UpdateDetectedTags(tags []DetectedTagInfo) {
	s.detectedTagsMut.Lock()
	s.detectedTags = tags
	s.lastTagUpdate = time.Now()
	s.detectedTagsMut.Unlock()
}

func (s *WebServer) handleCalibrationDetectedTags(c *gin.Context) {
	s.detectedTagsMut.RLock()
	if time.Since(s.lastTagUpdate) > 2*time.Second {
		s.detectedTags = nil
	}
	tags := s.detectedTags
	s.detectedTagsMut.RUnlock()
	c.JSON(http.StatusOK, gin.H{"tags": tags, "count": len(tags)})
}

type CalibrationComputeRequest struct {
	TagID   int           `json:"tagId"`
	TagSize float64       `json:"tagSize"`
	Corners [4][2]float64 `json:"corners"`
}

type CalibrationComputeResponse struct {
	State          string      `json:"state"`
	TagID          int         `json:"tagId,omitempty"`
	ComputedWidth  float64     `json:"computedWidth,omitempty"`
	ComputedHeight float64     `json:"computedHeight,omitempty"`
	PixelsPerMeter float64     `json:"pixelsPerMeter,omitempty"`
	Message        string      `json:"message,omitempty"`
	Error          string      `json:"error,omitempty"`
	Filename       string      `json:"filename,omitempty"`
	Homography     [][]float64 `json:"homography,omitempty"`
}

func (s *WebServer) handleCalibrationCompute(c *gin.Context) {
	var req CalibrationComputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tagSize := req.TagSize
	if tagSize <= 0 {
		tagSize = 0.15
	}

	halfSize := tagSize / 2.0

	srcPoints := []position.Point2D{
		{X: req.Corners[0][0], Y: req.Corners[0][1]},
		{X: req.Corners[1][0], Y: req.Corners[1][1]},
		{X: req.Corners[2][0], Y: req.Corners[2][1]},
		{X: req.Corners[3][0], Y: req.Corners[3][1]},
	}

	dstPoints := []position.Point2D{
		{X: -halfSize, Y: -halfSize},
		{X: halfSize, Y: -halfSize},
		{X: halfSize, Y: halfSize},
		{X: -halfSize, Y: halfSize},
	}

	h := position.NewHomography()
	err := h.ComputeFromPoints(srcPoints, dstPoints)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to compute homography: %v", err)})
		return
	}

	computedWidth := tagSize
	computedHeight := tagSize

	pixelsPerMeter := h.GetPixelsPerMeter()

	hMatrix := [][]float64{
		{h.H[0][0], h.H[0][1], h.H[0][2]},
		{h.H[1][0], h.H[1][1], h.H[1][2]},
		{h.H[2][0], h.H[2][1], h.H[2][2]},
	}

	s.calibrationMutex.Lock()
	s.calibrationData = &CalibrationSaveRequest{
		ComputedWidth:  computedWidth,
		ComputedHeight: computedHeight,
		PixelsPerMeter: pixelsPerMeter,
		Homography:     hMatrix,
	}
	s.calibrationTagSize = tagSize
	s.calibrationMutex.Unlock()

	cameraFile := GetCalibrationFilename(s.cameraName)

	calib := &position.CalibrationConfig{
		Version: 1,
		Camera: position.CameraInfo{
			Name:       s.cameraName,
			Resolution: [2]int{1280, 720},
		},
		Intrinsics: position.CameraIntrinsics{
			CameraMatrix: [3][3]float64{
				{pixelsPerMeter * 800, 0, 640},
				{0, pixelsPerMeter * 800, 360},
				{0, 0, 1},
			},
			DistortionCoeffs: [5]float64{0, 0, 0, 0, 0},
			Width:            1280,
			Height:           720,
		},
		Homography:   hMatrix,
		WorldScale:   pixelsPerMeter,
		TagSize:      tagSize,
		CalibratedAt: "2026-02-06",
	}

	saveToFile := func(filename string) error {
		data, err := yaml.Marshal(calib)
		if err != nil {
			return fmt.Errorf("failed to marshal: %w", err)
		}
		dir := filepath.Dir(filename)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		return nil
	}

	if err := saveToFile(cameraFile); err != nil {
		log.Printf("Warning: failed to save calibration: %v", err)
	}

	s.SetCalibrationState("complete",
		fmt.Sprintf("Calibration complete! Area: %.2fm x %.2fm", computedWidth, computedHeight),
		cameraFile, tagSize)

	c.JSON(http.StatusOK, CalibrationComputeResponse{
		State:          "complete",
		TagID:          req.TagID,
		ComputedWidth:  computedWidth,
		ComputedHeight: computedHeight,
		PixelsPerMeter: pixelsPerMeter,
		Message:        "Calibration computed successfully",
		Filename:       cameraFile,
		Homography:     hMatrix,
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
			Addr:    s.addr,
			Handler: s.engine,
		}
		if err := srv.ListenAndServe(); err != nil && !strings.Contains(err.Error(), "Server closed") {
			log.Printf("HTTP server error: %v", err)
		}
	}()
}

func (s *WebServer) Stop() {
	s.isRunning = false
	close(s.stopChan)
	log.Printf("Web server stopped")
}

func (s *WebServer) PushFrame(img image.Image) {
	if img == nil {
		return
	}
	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	buf := make([]byte, width*height*3)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			idx := ((y-bounds.Min.Y)*width + (x - bounds.Min.X)) * 3
			buf[idx+0] = byte(r >> 8)
			buf[idx+1] = byte(g >> 8)
			buf[idx+2] = byte(b >> 8)
		}
	}
	s.stream.UpdateJPEG(buf)
}

func (s *WebServer) PushRawJPEG(jpegData []byte) {
	if len(jpegData) == 0 {
		return
	}
	s.stream.UpdateJPEG(jpegData)
}

func (s *WebServer) UpdateStats(tagCount, yoloCount int) {
	s.statsMutex.Lock()
	s.lastTagCount = tagCount
	s.lastYoloCount = yoloCount
	s.statsMutex.Unlock()
}

func (s *WebServer) handleCalibrationCancel(c *gin.Context) {
	s.SetCalibrationState("not_calibrated", "Click Settings to calibrate", "", 0)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "state": "cancelled"})
}

func distPoints(p1, p2 [2]float64) float64 {
	dx := p2[0] - p1[0]
	dy := p2[1] - p1[1]
	return math.Sqrt(dx*dx + dy*dy)
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

	newObs := planning.Obstacle{
		Name:              fmt.Sprintf("obstacle_%d", len(s.obstacles)+1),
		PixelsTopLeft:     req.PixelTopLeft,
		PixelsBottomRight: req.PixelBottomRight,
		WorldTopLeft:      [2]float64{float64(req.PixelTopLeft[0]) / 100, float64(req.PixelTopLeft[1]) / 100},
		WorldBottomRight:  [2]float64{float64(req.PixelBottomRight[0]) / 100, float64(req.PixelBottomRight[1]) / 100},
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

	for i, obs := range s.obstacles {
		if obs.Name == id {
			s.obstacles[i] = planning.Obstacle{
				Name:              id,
				PixelsTopLeft:     req.PixelTopLeft,
				PixelsBottomRight: req.PixelBottomRight,
				WorldTopLeft:      [2]float64{float64(req.PixelTopLeft[0]) / 100, float64(req.PixelTopLeft[1]) / 100},
				WorldBottomRight:  [2]float64{float64(req.PixelBottomRight[0]) / 100, float64(req.PixelBottomRight[1]) / 100},
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

	c.JSON(http.StatusOK, gin.H{"status": "ok", "count": 0})
}

func (s *WebServer) handleObstaclesSave(c *gin.Context) {
	s.obstaclesMutex.RLock()
	obstacles := s.obstacles
	s.obstaclesMutex.RUnlock()

	path := s.GetObstaclesPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
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
		yamlContent += fmt.Sprintf("    pixels:\n")
		yamlContent += fmt.Sprintf("      top_left: [%d, %d]\n", obs.PixelsTopLeft[0], obs.PixelsTopLeft[1])
		yamlContent += fmt.Sprintf("      bottom_right: [%d, %d]\n", obs.PixelsBottomRight[0], obs.PixelsBottomRight[1])
		yamlContent += fmt.Sprintf("    world:\n")
		yamlContent += fmt.Sprintf("      top_left: [%.4f, %.4f]\n", obs.WorldTopLeft[0], obs.WorldTopLeft[1])
		yamlContent += fmt.Sprintf("      bottom_right: [%.4f, %.4f]\n", obs.WorldBottomRight[0], obs.WorldBottomRight[1])
		if i < len(obstacles)-1 {
			yamlContent += "\n"
		}
	}

	return os.WriteFile(path, []byte(yamlContent), 0644)
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
