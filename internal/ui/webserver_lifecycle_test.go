//go:build gocv

package ui

import (
	"net"
	"net/http"
	"testing"
	"time"
)

// TestWebServer_StopShutsDownListener is the regression test for finding #12
// in docs/code-review-2026-09-27.md: Stop() used to close an unused
// `stopChan` and flip a flag, but never stored or shut down the *http.Server
// created inside Start()'s goroutine, so the HTTP/WS/MJPEG listener kept
// accepting connections forever. Here we start a real listener, confirm it
// answers, call Stop(), and confirm the port stops accepting connections.
func TestWebServer_StopShutsDownListener(t *testing.T) {
	addr := freeAddr(t)
	server := NewWebServer(addr)
	server.Start()

	waitForServerUp(t, addr)

	server.Stop()

	waitForServerDown(t, addr)
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find a free port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func waitForServerUp(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/api/status")
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s never came up", addr)
}

func waitForServerDown(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err != nil {
			return
		}
		_ = conn.Close()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s is still accepting connections after Stop()", addr)
}
