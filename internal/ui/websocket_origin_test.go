//go:build gocv

package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWebSocket_RejectsCrossOriginHandshake(t *testing.T) {
	server := NewWebServer(":0")
	httpServer := httptest.NewServer(server.router.engine)
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	tests := []struct {
		name     string
		origin   string
		wantConn bool
	}{
		{"no origin", "", true},
		{"same origin", httpServer.URL, true},
		{"foreign site", "https://evil.example", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hdr := http.Header{}
			if tt.origin != "" {
				hdr.Set("Origin", tt.origin)
			}
			conn, resp, err := websocket.DefaultDialer.Dial(wsURL, hdr)
			if conn != nil {
				_ = conn.Close()
			}
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			if tt.wantConn && err != nil {
				t.Errorf("handshake failed: %v", err)
			}
			if !tt.wantConn {
				if err == nil {
					t.Fatal("cross-origin handshake succeeded, want rejection")
				}
				if resp == nil || resp.StatusCode != http.StatusForbidden {
					t.Errorf("response = %v, want 403", resp)
				}
			}
		})
	}
}
