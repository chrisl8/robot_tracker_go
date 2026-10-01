//go:build gocv

package ui

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *WebServer) handleIndex(c *gin.Context) {
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	data, err := StaticFiles.ReadFile("static/index.html")
	if err != nil {
		c.String(500, "Failed to load index.html: %v", err)
		return
	}
	c.Data(200, "text/html; charset=utf-8", data)
}

// handleVersion reports the id of the UI build embedded in this binary, so an
// open page can tell it was loaded from an older build. The id is empty when
// there is no UI build embedded (a Go-only build).
func (s *WebServer) handleVersion(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var v struct {
		ID string `json:"id"`
	}
	if data, err := StaticFiles.ReadFile("static/version.json"); err == nil {
		_ = json.Unmarshal(data, &v)
	}
	c.JSON(http.StatusOK, v)
}

func (s *WebServer) handleMJPEG(c *gin.Context) {
	s.router.stream.serveHTTP(c.Writer, c.Request)
}

func (s *WebServer) PushFrame(img image.Image) {
	if img == nil {
		return
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return
	}
	s.router.stream.updateJPEG(buf.Bytes())
}

func (s *WebServer) PushRawJPEG(jpegData []byte) {
	if len(jpegData) == 0 {
		return
	}
	s.router.stream.updateJPEG(jpegData)
}
