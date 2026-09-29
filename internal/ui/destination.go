//go:build gocv

package ui

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// DestinationRequest's fields are pointers so a missing field is a 400 rather
// than silently becoming 0 (robot 0, or pixel (0,0)).
type DestinationRequest struct {
	RobotID *int `json:"robot_id"`
	X       *int `json:"x"`
	Y       *int `json:"y"`
}

// frameSize is the current video frame size, or 0, 0 if not yet known.
func (s *WebServer) frameSize() (int, int) {
	if pe := s.estimator(); pe != nil {
		return pe.FrameSize()
	}
	return 0, 0
}

func (s *WebServer) handleDestination(c *gin.Context) {
	var req DestinationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.RobotID == nil || req.X == nil || req.Y == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "robot_id, x and y are all required"})
		return
	}
	robotID, x, y := *req.RobotID, *req.X, *req.Y

	frameW, frameH := s.frameSize()
	if problem := destinationProblem(robotID, x, y, frameW, frameH); problem != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": problem})
		return
	}

	s.obstacles.mutex.RLock()
	obstacles := s.obstacles.list
	s.obstacles.mutex.RUnlock()

	if len(obstacles) > 0 {
		destX := float64(x)
		destY := float64(y)

		for _, obs := range obstacles {
			if destX >= float64(obs.PixelsTopLeft[0]) && destX <= float64(obs.PixelsBottomRight[0]) &&
				destY >= float64(obs.PixelsTopLeft[1]) && destY <= float64(obs.PixelsBottomRight[1]) {
				utils.Logf("DESTINATION_REJECTED: Destination (%.0f, %.0f) overlaps with obstacle '%s'",
					destX, destY, obs.Name)
				c.JSON(http.StatusBadRequest, gin.H{
					"error":    "Destination overlaps with obstacle",
					"obstacle": obs.Name,
				})
				return
			}
		}
	}

	// Ask the app first: if it refuses (e.g. not calibrated) the destination
	// must not be stored or shown, or the UI would display a goal the planner
	// never received.
	if s.Callbacks.OnDestinationSet != nil {
		if err := s.Callbacks.OnDestinationSet(robotID, [2]float64{float64(x), float64(y)}); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	}

	dest := DestinationMessage{RobotID: robotID, X: x, Y: y, Valid: true}
	s.destination.mutex.Lock()
	s.destination.current = dest
	s.destination.mutex.Unlock()

	// Broadcast a copy: the shared struct is rewritten by ClearDestination.
	s.BroadcastOverlay(OverlayMessage{
		Type:        "destination",
		Destination: &dest,
	})

	c.JSON(http.StatusOK, gin.H{"status": "ok", "destination": gin.H{"robot_id": robotID, "x": x, "y": y}})
}

func (s *WebServer) ClearDestination(robotID int) {
	s.destination.mutex.Lock()
	s.destination.current = DestinationMessage{Valid: false}
	s.destination.mutex.Unlock()

	cleared := DestinationMessage{RobotID: robotID, Valid: false}
	s.BroadcastOverlay(OverlayMessage{
		Type:        "destination_clear",
		Destination: &cleared,
	})
}

func (s *WebServer) handleDestinationClear(c *gin.Context) {
	var req struct {
		RobotID *int `json:"robot_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.RobotID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "robot_id is required"})
		return
	}
	robotID := *req.RobotID

	s.ClearDestination(robotID)

	if s.Callbacks.OnDestinationClear != nil {
		s.Callbacks.OnDestinationClear(robotID)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "robot_id": robotID})
}
