//go:build gocv

package ui

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestBroadcastOverlay_ConcurrentCallersDoNotPanic is the regression test for a
// crash observed live: the frame loop, HTTP obstacle handlers, and the FPS
// watchdog timer all call BroadcastOverlay from different goroutines, and
// gorilla/websocket panics (crashing the whole process, since this happens
// outside any HTTP handler's gin.Recovery) if two goroutines write to the same
// connection at once. This drives many concurrent broadcasters against several
// real client connections and fails if any of them panics.
func TestBroadcastOverlay_ConcurrentCallersDoNotPanic(t *testing.T) {
	server := NewWebServer(":0")
	httpServer := httptest.NewServer(server.engine)
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	const numClients = 3
	for i := 0; i < numClients; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("client %d failed to connect: %v", i, err)
		}
		defer func() { _ = conn.Close() }()

		// Drain messages so the server never blocks on a full write buffer.
		go func(c *websocket.Conn) {
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}(conn)
	}

	// Let the server register all the clients before hammering it.
	deadline := time.Now().Add(time.Second)
	for {
		server.clientMutex.RLock()
		n := len(server.clients)
		server.clientMutex.RUnlock()
		if n == numClients || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	const (
		broadcasters      = 8
		messagesPerCaller = 200
	)
	panics := make(chan any, broadcasters)
	var wg sync.WaitGroup
	wg.Add(broadcasters)
	for i := 0; i < broadcasters; i++ {
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics <- r
				}
			}()
			for j := 0; j < messagesPerCaller; j++ {
				// Mirrors the real broadcasters: frame loop status updates, HTTP
				// obstacle handlers, and the watchdog's stalled-camera message.
				switch (id + j) % 3 {
				case 0:
					server.BroadcastOverlay(OverlayMessage{Type: "status", Status: &StatusMessage{FPS: float64(j)}})
				case 1:
					server.BroadcastObstacles()
				default:
					server.BroadcastCameraStalled(1.0, 2.0)
				}
			}
		}(i)
	}
	wg.Wait()
	close(panics)

	for p := range panics {
		t.Errorf("concurrent BroadcastOverlay callers panicked: %v", p)
	}
}
