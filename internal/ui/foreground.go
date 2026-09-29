//go:build gocv

package ui

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *WebServer) handleForegroundState(c *gin.Context) {
	state := ForegroundState{}
	if s.Callbacks.ForegroundState != nil {
		state = s.Callbacks.ForegroundState()
	}
	c.JSON(http.StatusOK, state)
}

func (s *WebServer) handleForegroundEnabled(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be {\"enabled\": true|false}"})
		return
	}
	if s.Callbacks.OnForegroundEnabled != nil {
		s.Callbacks.OnForegroundEnabled(*req.Enabled)
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "enabled": *req.Enabled})
}

func (s *WebServer) handleForegroundApply(c *gin.Context) {
	var req struct {
		Apply *bool `json:"apply"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Apply == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be {\"apply\": true|false}"})
		return
	}
	if s.Callbacks.OnForegroundApply != nil {
		s.Callbacks.OnForegroundApply(*req.Apply)
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "apply": *req.Apply})
}

func (s *WebServer) handleForegroundReset(c *gin.Context) {
	if s.Callbacks.OnForegroundReset != nil {
		s.Callbacks.OnForegroundReset()
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *WebServer) handleForegroundAbsorb(c *gin.Context) {
	var req struct {
		X *float64 `json:"x"`
		Y *float64 `json:"y"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.X == nil || req.Y == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be {\"x\": number, \"y\": number}"})
		return
	}
	if s.Callbacks.OnForegroundAbsorb != nil {
		s.Callbacks.OnForegroundAbsorb(int(*req.X), int(*req.Y))
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleForegroundDebug serves the detector's debug view. Asking for it makes
// the detector render it for the next couple of seconds.
func (s *WebServer) handleForegroundDebug(c *gin.Context) {
	var jpeg []byte
	if s.Callbacks.ForegroundDebugJPEG != nil {
		jpeg = s.Callbacks.ForegroundDebugJPEG()
	}
	if len(jpeg) == 0 {
		c.Status(http.StatusNoContent)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/jpeg", jpeg)
}
