package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOriginAllowed(t *testing.T) {
	tests := []struct {
		name, host, origin string
		want               bool
	}{
		{"no origin (curl, native client)", "192.168.1.5:9086", "", true},
		{"same origin", "192.168.1.5:9086", "http://192.168.1.5:9086", true},
		{"same origin hostname", "robot.local:9086", "http://robot.local:9086", true},
		{"other site", "192.168.1.5:9086", "https://evil.example", false},
		{"other port same host", "192.168.1.5:9086", "http://192.168.1.5:8000", false},
		{"vite dev proxy", "localhost:9086", "http://localhost:5173", true},
		{"loopback ip dev", "127.0.0.1:9086", "http://localhost:5173", true},
		{"loopback origin against LAN server", "192.168.1.5:9086", "http://localhost:5173", false},
		{"opaque null origin", "192.168.1.5:9086", "null", false},
		{"garbage origin", "192.168.1.5:9086", "::not a url", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "http://"+tt.host+"/api/command", nil)
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			if got := originAllowed(r); got != tt.want {
				t.Errorf("originAllowed(host=%q, origin=%q) = %v, want %v", tt.host, tt.origin, got, tt.want)
			}
		})
	}
}

func guardedEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(requestGuardMiddleware())
	h := func(c *gin.Context) {
		if _, err := io.Copy(io.Discard, c.Request.Body); err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusOK)
	}
	e.GET("/x", h)
	e.POST("/x", h)
	e.DELETE("/x", h)
	return e
}

func do(e *gin.Engine, method, body string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://robot:9086/x", strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

func TestRequestGuard(t *testing.T) {
	e := guardedEngine()
	js := map[string]string{"Content-Type": "application/json"}

	tests := []struct {
		name   string
		method string
		body   string
		hdr    map[string]string
		want   int
	}{
		{"GET is never blocked, even cross-origin", "GET", "", map[string]string{"Origin": "https://evil.example"}, 200},
		{"JSON POST", "POST", `{"a":1}`, js, 200},
		{"JSON with charset", "POST", `{}`, map[string]string{"Content-Type": "application/json; charset=utf-8"}, 200},
		{"bodyless POST (e-stop style)", "POST", "", nil, 200},
		{"text/plain body is a CORS simple request", "POST", `{"a":1}`, map[string]string{"Content-Type": "text/plain"}, 415},
		{"body with no content type", "POST", `{"a":1}`, nil, 415},
		{"cross-origin JSON POST", "POST", `{}`, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, 403},
		{"cross-origin bodyless POST", "POST", "", map[string]string{"Origin": "https://evil.example"}, 403},
		{"cross-origin DELETE", "DELETE", "", map[string]string{"Origin": "https://evil.example"}, 403},
		{"same-origin POST", "POST", `{}`, map[string]string{"Content-Type": "application/json", "Origin": "http://robot:9086"}, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := do(e, tt.method, tt.body, tt.hdr).Code; got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRequestGuard_RejectsOversizedBody(t *testing.T) {
	big := strings.Repeat("a", maxRequestBodyBytes+1)
	w := do(guardedEngine(), "POST", big, map[string]string{"Content-Type": "application/json"})
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", w.Code)
	}
}

func TestRequestGuard_NoCORSHeaders(t *testing.T) {
	w := do(guardedEngine(), "GET", "", nil)
	if v := w.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want none", v)
	}
}
