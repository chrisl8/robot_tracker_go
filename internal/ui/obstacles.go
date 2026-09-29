//go:build gocv

package ui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/chrisl8/robot_tracker_go/internal/planning"
)

func (s *WebServer) BroadcastObstacles() {
	s.obstacles.mutex.RLock()
	defer s.obstacles.mutex.RUnlock()

	obstacles := make([]ObstacleResponse, 0, len(s.obstacles.list))
	for _, obs := range s.obstacles.list {
		obstacles = append(obstacles, ObstacleResponse{
			ID:               obs.Name,
			Name:             obs.Name,
			PixelTopLeft:     obs.PixelsTopLeft,
			PixelBottomRight: obs.PixelsBottomRight,
			WorldTopLeft:     obs.WorldTopLeft,
			WorldBottomRight: obs.WorldBottomRight,
			Clearance:        0.02,
		})
	}

	s.BroadcastOverlay(OverlayMessage{
		Type: "obstacles",
		Obstacles: &ObstaclesMessage{
			Obstacles: obstacles,
			Count:     len(obstacles),
		},
	})
}

// notifyObstaclesChanged broadcasts the current obstacle list to WebSocket
// clients and, if OnObstaclesChanged is set, invokes it with a locked
// snapshot of s.obstacles.list. Callers must call this only after releasing
// obstaclesMutex: BroadcastObstacles takes its own RLock (so calling this
// while still holding the write lock would deadlock), and OnObstaclesChanged
// is arbitrary application code that must not run while any lock is held.
func (s *WebServer) notifyObstaclesChanged() {
	s.BroadcastObstacles()

	if s.Callbacks.OnObstaclesChanged != nil {
		s.obstacles.mutex.RLock()
		obstacles := s.obstacles.list
		s.obstacles.mutex.RUnlock()
		s.Callbacks.OnObstaclesChanged(obstacles)
	}
}

func (s *WebServer) handleObstaclesList(c *gin.Context) {
	s.obstacles.mutex.RLock()
	defer s.obstacles.mutex.RUnlock()

	obstacles := make([]ObstacleResponse, 0, len(s.obstacles.list))
	for _, obs := range s.obstacles.list {
		obstacles = append(obstacles, ObstacleResponse{
			ID:               obs.Name,
			Name:             obs.Name,
			PixelTopLeft:     obs.PixelsTopLeft,
			PixelBottomRight: obs.PixelsBottomRight,
			WorldTopLeft:     obs.WorldTopLeft,
			WorldBottomRight: obs.WorldBottomRight,
			Clearance:        0.02,
		})
	}

	c.JSON(http.StatusOK, ObstaclesListResponse{
		Obstacles: obstacles,
		Count:     len(obstacles),
		Saved:     s.obstacles.saved,
	})
}

func (s *WebServer) handleObstacleAdd(c *gin.Context) {
	var req AddObstacleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	frameW, frameH := s.frameSize()
	if problem := pixelBoxProblem(req.PixelTopLeft, req.PixelBottomRight, frameW, frameH); problem != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": problem})
		return
	}

	s.obstacles.mutex.Lock()

	worldTL, worldBR := s.pixelCornersToWorld(req.PixelTopLeft, req.PixelBottomRight)

	newObs := planning.NewRectObstacle(uniqueObstacleName(s.obstacles.list, req.Name), worldTL, worldBR)
	newObs.PixelsTopLeft = req.PixelTopLeft
	newObs.PixelsBottomRight = req.PixelBottomRight

	s.obstacles.list = append(s.obstacles.list, newObs)
	s.obstacles.saved = false
	count := len(s.obstacles.list)

	s.obstacles.mutex.Unlock()
	s.notifyObstaclesChanged()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": newObs.Name, "count": count})
}

func (s *WebServer) handleObstacleDelete(c *gin.Context) {
	id := c.Param("id")

	s.obstacles.mutex.Lock()

	newObs := make([]planning.Obstacle, 0, len(s.obstacles.list))
	for _, obs := range s.obstacles.list {
		if obs.Name != id {
			newObs = append(newObs, obs)
		}
	}
	if len(newObs) == len(s.obstacles.list) {
		s.obstacles.mutex.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "no obstacle with id " + id})
		return
	}
	s.obstacles.list = newObs
	s.obstacles.saved = false
	count := len(newObs)

	s.obstacles.mutex.Unlock()
	s.notifyObstaclesChanged()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": id, "count": count})
}

func (s *WebServer) handleObstacleUpdate(c *gin.Context) {
	id := c.Param("id")
	var req UpdateObstacleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	frameW, frameH := s.frameSize()
	if problem := pixelBoxProblem(req.PixelTopLeft, req.PixelBottomRight, frameW, frameH); problem != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": problem})
		return
	}

	s.obstacles.mutex.Lock()

	worldTL, worldBR := s.pixelCornersToWorld(req.PixelTopLeft, req.PixelBottomRight)

	// Copy-on-write: snapshots of the list were handed to the planner callback
	// and the file saver, so never mutate the shared backing array in place.
	newList := append([]planning.Obstacle(nil), s.obstacles.list...)
	found := false
	for i, obs := range newList {
		if obs.Name == id {
			updated := planning.NewRectObstacle(id, worldTL, worldBR)
			updated.PixelsTopLeft = req.PixelTopLeft
			updated.PixelsBottomRight = req.PixelBottomRight
			newList[i] = updated
			found = true
			break
		}
	}
	if !found {
		s.obstacles.mutex.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "no obstacle with id " + id})
		return
	}
	s.obstacles.list = newList
	s.obstacles.saved = false

	s.obstacles.mutex.Unlock()
	s.notifyObstaclesChanged()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": id})
}

func (s *WebServer) handleObstaclesClear(c *gin.Context) {
	s.obstacles.mutex.Lock()

	s.obstacles.list = make([]planning.Obstacle, 0)
	s.obstacles.saved = false

	s.obstacles.mutex.Unlock()
	s.notifyObstaclesChanged()

	// Persist cleared state to disk so obstacles don't return on restart
	path := s.GetObstaclesPath()
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0750); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if err := s.saveObstaclesToFile(path, []planning.Obstacle{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.obstacles.mutex.Lock()
	s.obstacles.saved = true
	s.obstacles.mutex.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "ok", "count": 0})
}

func (s *WebServer) handleObstaclesSave(c *gin.Context) {
	s.obstacles.mutex.RLock()
	obstacles := s.obstacles.list
	s.obstacles.mutex.RUnlock()

	path := s.GetObstaclesPath()
	dir := filepath.Dir(path)
	// #nosec G304
	if err := os.MkdirAll(dir, 0750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := s.saveObstaclesToFile(path, obstacles); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.obstacles.mutex.Lock()
	s.obstacles.saved = true
	s.obstacles.mutex.Unlock()

	s.BroadcastObstacles()

	c.JSON(http.StatusOK, SaveObstaclesResponse{
		Success: true,
		Message: fmt.Sprintf("Saved %d obstacles to %s", len(obstacles), path),
	})
}

// obstacleFileEntry and obstacleFile mirror the on-disk obstacle YAML shape
// that PositionEstimator.LoadObstacles parses generically (keys "version",
// "obstacles[].name", ".pixels.top_left"/".bottom_right",
// ".world.top_left"/".bottom_right"). Marshaling through these tagged
// structs, rather than building the YAML text by hand, keeps escaping and
// nesting correct by construction.
type obstacleFileEntry struct {
	Name   string `yaml:"name"`
	Pixels struct {
		TopLeft     [2]int `yaml:"top_left"`
		BottomRight [2]int `yaml:"bottom_right"`
	} `yaml:"pixels"`
	World struct {
		TopLeft     [2]float64 `yaml:"top_left"`
		BottomRight [2]float64 `yaml:"bottom_right"`
	} `yaml:"world"`
}

type obstacleFile struct {
	Version   int                 `yaml:"version"`
	Obstacles []obstacleFileEntry `yaml:"obstacles"`
}

func (s *WebServer) saveObstaclesToFile(path string, obstacles []planning.Obstacle) error {
	file := obstacleFile{Version: 1, Obstacles: make([]obstacleFileEntry, len(obstacles))}
	for i, obs := range obstacles {
		entry := obstacleFileEntry{Name: obs.Name}
		entry.Pixels.TopLeft = obs.PixelsTopLeft
		entry.Pixels.BottomRight = obs.PixelsBottomRight
		entry.World.TopLeft = obs.WorldTopLeft
		entry.World.BottomRight = obs.WorldBottomRight
		file.Obstacles[i] = entry
	}

	data, err := yaml.Marshal(file)
	if err != nil {
		return fmt.Errorf("failed to marshal obstacles: %w", err)
	}

	// #nosec G304
	// #nosec G306
	return os.WriteFile(path, data, 0600)
}

// RecomputeObstacleWorld re-derives every obstacle's floor coordinates from its
// pixel box using the current calibration. Obstacles are drawn in pixels, but
// the planner steers by the world coordinates, which are only correct for the
// calibration they were computed under; after a recalibration (or loading a
// file saved under an older one) they must be redone or the planner avoids a
// different patch of floor than the UI shows. Does nothing until the camera is
// calibrated.
func (s *WebServer) RecomputeObstacleWorld() {
	if pe := s.estimator(); pe == nil || !pe.IsCalibrated() {
		return
	}

	s.obstacles.mutex.Lock()
	if len(s.obstacles.list) == 0 {
		s.obstacles.mutex.Unlock()
		return
	}
	// Copy-on-write: the previous list was handed to the planner callback and
	// the file saver.
	updated := make([]planning.Obstacle, len(s.obstacles.list))
	for i, obs := range s.obstacles.list {
		worldTL, worldBR := s.pixelCornersToWorld(obs.PixelsTopLeft, obs.PixelsBottomRight)
		rebuilt := planning.NewRectObstacle(obs.Name, worldTL, worldBR)
		rebuilt.PixelsTopLeft = obs.PixelsTopLeft
		rebuilt.PixelsBottomRight = obs.PixelsBottomRight
		updated[i] = rebuilt
	}
	s.obstacles.list = updated
	s.obstacles.mutex.Unlock()

	s.notifyObstaclesChanged()
}

func (s *WebServer) GetObstacles() []planning.Obstacle {
	s.obstacles.mutex.RLock()
	defer s.obstacles.mutex.RUnlock()
	return s.obstacles.list
}

func (s *WebServer) SetObstacles(obstacles []planning.Obstacle) {
	s.obstacles.mutex.Lock()
	s.obstacles.list = obstacles
	s.obstacles.mutex.Unlock()

	if s.Callbacks.OnObstaclesChanged != nil {
		s.Callbacks.OnObstaclesChanged(obstacles)
	}
}

func (s *WebServer) GetObstaclesPath() string {
	s.obstacles.mutex.RLock()
	configured := s.obstacles.path
	s.obstacles.mutex.RUnlock()
	return ResolveObstaclesPath(configured, s.cameraNameValue())
}

// SetObstaclesPath sets the configured obstacles-file override (from
// ObstaclesConfig.GetPath()) that GetObstaclesPath/ResolveObstaclesPath
// consult before falling back to the per-camera default.
func (s *WebServer) SetObstaclesPath(path string) {
	s.obstacles.mutex.Lock()
	s.obstacles.path = path
	s.obstacles.mutex.Unlock()
}
