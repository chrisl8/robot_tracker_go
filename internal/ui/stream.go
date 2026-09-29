//go:build gocv

package ui

import (
	"bytes"
	"image"
	"image/jpeg"

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
