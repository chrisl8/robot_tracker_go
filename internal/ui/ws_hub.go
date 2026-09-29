//go:build gocv

package ui

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     originAllowed,
}

const wsMaxMessageSize = 4096

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
