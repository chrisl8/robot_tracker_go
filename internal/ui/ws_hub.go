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
	// wsSendBuffer is how many broadcast messages may queue for one client. A
	// client that falls this far behind (a dead phone whose TCP connection is
	// still open) is dropped instead of being waited for: broadcasts come from
	// the frame loop, which must never block on a slow browser.
	wsSendBuffer = 128
)

// wsClient is one connected browser. Every write to its connection goes
// through writePump, so writes are never interleaved and BroadcastOverlay never
// blocks on the network.
type wsClient struct {
	conn      *websocket.Conn
	send      chan []byte // marshaled JSON messages
	done      chan struct{}
	closeOnce sync.Once
}

func newWSClient(conn *websocket.Conn) *wsClient {
	return &wsClient{conn: conn, send: make(chan []byte, wsSendBuffer), done: make(chan struct{})}
}

// close shuts the connection down (unblocking the reader) and stops the
// writer. Safe to call from any goroutine, any number of times.
func (c *wsClient) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

func (s *WebServer) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	// A client that vanishes without a clean TCP close (sleep, NAT timeout,
	// pulled network cable) never makes ReadMessage return an error on its
	// own — without a deadline it just blocks forever, leaking this
	// connection's goroutines and its s.hub.clients entry. SetReadDeadline plus
	// a pong handler that renews it turns silence into a timeout error, and
	// writePump's pings are what solicit those pongs.
	conn.SetReadLimit(wsMaxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(s.router.wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(s.router.wsPongWait))
	})

	client := newWSClient(conn)
	s.hub.clientMutex.Lock()
	s.hub.clients[conn] = client
	s.hub.clientMutex.Unlock()

	go s.wsReader(client)
	go s.writePump(client)
}

// wsReader drains the connection so pongs and close frames are processed (the
// server takes no commands over the socket; drive commands are POSTed and
// validated), and unregisters the client when the connection ends.
func (s *WebServer) wsReader(client *wsClient) {
	defer func() {
		client.close()
		s.hub.clientMutex.Lock()
		delete(s.hub.clients, client.conn)
		s.hub.clientMutex.Unlock()
	}()

	for {
		if _, _, err := client.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump is the only writer for a client: it sends queued broadcasts and
// periodic pings (which detect a silently-dead connection, see
// handleWebSocket), each bounded by wsWriteWait. On any failure it closes the
// client, which unblocks wsReader so the registry entry is cleaned up.
func (s *WebServer) writePump(client *wsClient) {
	ticker := time.NewTicker(s.router.wsPingPeriod)
	defer ticker.Stop()
	defer client.close()

	for {
		var err error
		select {
		case <-client.done:
			return
		case msg := <-client.send:
			_ = client.conn.SetWriteDeadline(time.Now().Add(s.router.wsWriteWait))
			err = client.conn.WriteMessage(websocket.TextMessage, msg)
		case <-ticker.C:
			_ = client.conn.SetWriteDeadline(time.Now().Add(s.router.wsWriteWait))
			err = client.conn.WriteMessage(websocket.PingMessage, nil)
		}
		if err != nil {
			return
		}
	}
}

// BroadcastOverlay queues msg for every connected client and returns without
// waiting for the network: it is called from the frame loop, the HTTP
// handlers and the watchdog, and a stalled browser must not stall any of them.
// A client whose queue is full is dropped; the UI reconnects and re-reads
// state.
func (s *WebServer) BroadcastOverlay(msg OverlayMessage) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}

	s.hub.clientMutex.RLock()
	clients := make([]*wsClient, 0, len(s.hub.clients))
	for _, client := range s.hub.clients {
		clients = append(clients, client)
	}
	s.hub.clientMutex.RUnlock()

	for _, client := range clients {
		select {
		case client.send <- payload:
		default:
			client.close()
		}
	}
}
