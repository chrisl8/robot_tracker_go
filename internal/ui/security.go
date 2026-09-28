package ui

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// maxRequestBodyBytes caps every request body. The largest legitimate payload
// (a calibration compute request) is a few KB.
const maxRequestBodyBytes = 1 << 20

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func hostnameOnly(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return strings.Trim(hostport, "[]")
}

// originAllowed reports whether a browser-originated request may drive this
// server. The server has no authentication and controls a physical robot, so
// without this any web page open in a browser on the LAN could POST commands
// to it (cross-origin fetches are sent even when the response is unreadable).
//
// A request with no Origin header is not from a cross-site browser context and
// is allowed. Otherwise the origin must be the server's own host, or a
// loopback origin talking to a loopback server (the Vite dev proxy keeps the
// dev server's Origin while rewriting Host).
func originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false // includes the opaque "null" origin
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	return isLoopbackHost(u.Hostname()) && isLoopbackHost(hostnameOnly(r.Host))
}

// requestGuardMiddleware protects state-changing requests: it rejects
// cross-origin callers, requires a JSON Content-Type on any request with a body
// (a non-JSON type is a CORS "simple request" that needs no preflight), and
// bounds the body size. It deliberately adds no CORS headers: the UI is served
// by this server, so no cross-origin access is needed.
func requestGuardMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if !originAllowed(c.Request) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross-origin request rejected"})
			return
		}
		if c.Request.ContentLength != 0 && c.ContentType() != "application/json" {
			c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{"error": "Content-Type must be application/json"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
		c.Next()
	}
}
