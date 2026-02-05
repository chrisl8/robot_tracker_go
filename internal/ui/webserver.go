package ui

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hybridgroup/mjpeg"
)

type WebServer struct {
	addr        string
	engine      *gin.Engine
	stream      *mjpeg.Stream
	wsUpgrader  websocket.Upgrader
	clients     map[*websocket.Conn]bool
	clientMutex sync.RWMutex
	lastFrame   image.Image
	frameMutex  sync.RWMutex
	isRunning   bool
	stopChan    chan struct{}
}

type OverlayMessage struct {
	Type    string          `json:"type"`
	BBox    *BBoxMessage    `json:"bbox,omitempty"`
	Track   *TrackMessage   `json:"track,omitempty"`
	Path    *PathMessage    `json:"path,omitempty"`
	Status  *StatusMessage  `json:"status,omitempty"`
	Command *CommandMessage `json:"command,omitempty"`
}

type BBoxMessage struct {
	X1, Y1, X2, Y2 int     `json:"x1,y1,x2,y2"`
	Label          string  `json:"label"`
	Color          string  `json:"color"`
	Confidence     float64 `json:"confidence"`
}

type TrackMessage struct {
	ID         int      `json:"id"`
	BBox       []int    `json:"bbox"`
	History    [][2]int `json:"history"`
	Color      string   `json:"color"`
	Confidence float64  `json:"confidence"`
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
	s.engine.GET("/", s.handleIndex)
	s.engine.GET("/stream", s.handleMJPEG)
	s.engine.GET("/ws", s.handleWebSocket)
	s.engine.POST("/api/command", s.handleCommand)
	s.engine.POST("/api/destination", s.handleDestination)
	s.engine.GET("/api/status", s.handleStatus)
}

func (s *WebServer) handleIndex(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.String(http.StatusOK, indexHTML)
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
	c.JSON(http.StatusOK, gin.H{
		"connected":    s.isRunning,
		"fps":          30.0,
		"robotCount":   0,
		"arduinoState": "disconnected",
	})
}

func (s *WebServer) Start() {
	s.isRunning = true
	go s.engine.Run(s.addr)
}

func (s *WebServer) Stop() {
	s.isRunning = false
	close(s.stopChan)
}

func (s *WebServer) PushFrame(frame image.Image) {
	if frame == nil {
		return
	}

	buf := new(bytes.Buffer)
	_ = jpeg.Encode(buf, frame, &jpeg.Options{Quality: 70})
	s.stream.UpdateJPEG(buf.Bytes())
}

func (s *WebServer) BroadcastOverlay(msg OverlayMessage) {
	s.clientMutex.RLock()
	for client := range s.clients {
		client.WriteJSON(msg)
	}
	s.clientMutex.RUnlock()
}

func (s *WebServer) IsRunning() bool {
	return s.isRunning
}

func (s *WebServer) GetAddr() string {
	return s.addr
}
