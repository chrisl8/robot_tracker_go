//go:build gocv

package ui

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestWebServer_SilentlyDeadClientIsCleanedUp is the regression test for
// finding #11 in docs/archived/code-review-2026-09-27.md: a client that vanishes
// without a clean TCP close (sleep, NAT timeout, cable pull) never makes
// ReadMessage return on its own, so without a read deadline the server used
// to leak that connection's goroutine and its s.clients entry forever. Here
// the client connects but never calls ReadMessage again (gorilla/websocket
// only answers ping control frames from inside a read loop), so it can never
// produce a pong — the read-deadline/ping mechanism is the only thing that
// can ever notice it's gone.
func TestWebServer_SilentlyDeadClientIsCleanedUp(t *testing.T) {
	server := NewWebServer(":0")
	// Shrink this server's liveness timers so the test doesn't wait out real
	// minute-scale timeouts. These are per-server fields (not shared package
	// vars) specifically so this doesn't race another test's WebServer.
	server.router.wsPongWait = 150 * time.Millisecond
	server.router.wsPingPeriod = (server.router.wsPongWait * 9) / 10
	server.router.wsWriteWait = 50 * time.Millisecond
	httpServer := httptest.NewServer(server.router.engine)
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("client failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()
	// Deliberately never call conn.ReadMessage(): a real dead client (killed
	// process, vanished network) never reads or answers pings either.

	// Wait for the server to register the connection.
	deadline := time.Now().Add(time.Second)
	for {
		server.hub.clientMutex.RLock()
		n := len(server.hub.clients)
		server.hub.clientMutex.RUnlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server never registered the client connection")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// The client answers nothing, so once wsPongWait elapses the server's
	// read deadline should expire and clean up the connection on its own.
	deadline = time.Now().Add(2 * time.Second)
	for {
		server.hub.clientMutex.RLock()
		n := len(server.hub.clients)
		server.hub.clientMutex.RUnlock()
		if n == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never cleaned up a silently-dead client (still has %d entries)", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
