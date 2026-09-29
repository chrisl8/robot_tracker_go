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
	httpServer := httptest.NewServer(server.router.engine)
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
		server.hub.clientMutex.RLock()
		n := len(server.hub.clients)
		server.hub.clientMutex.RUnlock()
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

// BroadcastOverlay runs on the frame loop (which also drives the robot), so a
// browser that has stopped reading must not hold it up: the write used to be
// synchronous, blocking the broadcaster for up to the write deadline per dead
// client.
func TestBroadcastOverlay_StalledClientDoesNotBlockBroadcaster(t *testing.T) {
	server := NewWebServer(":0")
	server.router.wsWriteWait = 30 * time.Second // the old code would sit in a write this long
	httpServer := httptest.NewServer(server.router.engine)
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	// Never read from conn: like a phone that went to sleep with the socket open.

	deadline := time.Now().Add(time.Second)
	for {
		server.hub.clientMutex.RLock()
		n := len(server.hub.clients)
		server.hub.clientMutex.RUnlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server never registered the client")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Big enough that the kernel socket buffers fill and the writer blocks.
	big := OverlayMessage{Type: "obstacles", Obstacles: &ObstaclesMessage{
		Obstacles: make([]ObstacleResponse, 400),
	}}
	start := time.Now()
	for i := 0; i < 300; i++ {
		server.BroadcastOverlay(big)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("300 broadcasts took %v with one stalled client; the broadcaster is waiting on the network", elapsed)
	}

	// The stalled client falls further behind than its queue allows and is dropped.
	deadline = time.Now().Add(2 * time.Second)
	for {
		server.hub.clientMutex.RLock()
		n := len(server.hub.clients)
		server.hub.clientMutex.RUnlock()
		if n == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the stalled client was never dropped")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
