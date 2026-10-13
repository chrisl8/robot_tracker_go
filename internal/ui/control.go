//go:build gocv

package ui

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CommandRequest is the body of a manual drive command: F, B, L, R or S.
type CommandRequest struct {
	Command string `json:"command"`
}

func (s *WebServer) handleCommand(c *gin.Context) {
	var req CommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !isValidCommand(req.Command) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown command, must be one of F/B/L/R/S"})
		return
	}
	if s.Callbacks.OnCommand != nil {
		if err := s.Callbacks.OnCommand(req.Command); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "command": req.Command})
}

// ModeRequest is the body of a control-mode change: "hold", "manual" or "autonomous".
type ModeRequest struct {
	Mode string `json:"mode"`
}

// ControlStateResponse is the control mode and emergency-stop state, as sent to the UI.
type ControlStateResponse struct {
	Mode             string `json:"mode"`
	EmergencyStopped bool   `json:"emergency_stopped"`
}

func (s *WebServer) handleSetMode(c *gin.Context) {
	var req ModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Mode != "hold" && req.Mode != "manual" && req.Mode != "autonomous" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mode, must be hold/manual/autonomous"})
		return
	}
	if s.Callbacks.OnModeChange != nil {
		if err := s.Callbacks.OnModeChange(req.Mode); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	}
	s.broadcastControlState()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "mode": req.Mode})
}

func (s *WebServer) handleEmergencyStop(c *gin.Context) {
	if s.Callbacks.OnEmergencyStop != nil {
		s.Callbacks.OnEmergencyStop()
	}
	s.broadcastControlState()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "emergency_stopped": true})
}

func (s *WebServer) handleClearEmergencyStop(c *gin.Context) {
	if s.Callbacks.OnClearEmergencyStop != nil {
		if err := s.Callbacks.OnClearEmergencyStop(); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	}
	s.broadcastControlState()
	c.JSON(http.StatusOK, gin.H{"status": "ok", "emergency_stopped": false})
}

// broadcastControlState tells every connected UI the current control mode and
// e-stop state, so a second tab (or a reconnected one) doesn't keep showing
// stale controls after another operator changed them.
func (s *WebServer) broadcastControlState() {
	mode, eStopped := "hold", false
	if s.Callbacks.OnGetControlState != nil {
		mode, eStopped = s.Callbacks.OnGetControlState()
	}
	s.BroadcastOverlay(OverlayMessage{
		Type:    "control_state",
		Control: &ControlStateResponse{Mode: mode, EmergencyStopped: eStopped},
	})
}

func (s *WebServer) handleControlState(c *gin.Context) {
	mode := "hold"
	eStopped := false
	if s.Callbacks.OnGetControlState != nil {
		mode, eStopped = s.Callbacks.OnGetControlState()
	}
	c.JSON(http.StatusOK, ControlStateResponse{
		Mode:             mode,
		EmergencyStopped: eStopped,
	})
}
