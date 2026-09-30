//go:build gocv

package ui

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
)

func (s *WebServer) handleStatus(c *gin.Context) {
	s.stats.mutex.RLock()
	tagCount := s.stats.lastTagCount
	fps := s.stats.lastFPS
	uptimeSec := s.stats.lastUptimeSec
	hostMemMB := s.stats.lastHostMemMB
	s.stats.mutex.RUnlock()

	s.stats.arduinoMutex.RLock()
	connected := s.stats.arduinoConnected
	robot := s.robotSnapshotLocked()
	s.stats.arduinoMutex.RUnlock()

	state := "Disconnected"
	if connected {
		state = "Connected"
	}

	c.JSON(http.StatusOK, gin.H{
		"connected":    s.router.isRunning.Load(),
		"fps":          fps,
		"robotCount":   tagCount,
		"tagCount":     tagCount,
		"arduinoState": state,
		"robotLink":    robot.link,
		"robotServos":  robot.servos,
		"robotMode":    robot.mode,
		"robotReboots": robot.reboots,
		"hostMemoryMB": hostMemMB,
		"uptimeSec":    uptimeSec,
	})
}

func (s *WebServer) UpdateStats(tagCount int) {
	s.stats.mutex.Lock()
	s.stats.lastTagCount = tagCount
	s.stats.mutex.Unlock()
}

func (s *WebServer) SetArduinoConnected(connected bool) {
	s.stats.arduinoMutex.Lock()
	s.stats.arduinoConnected = connected
	s.stats.arduinoMutex.Unlock()
}

// robotState is the robot-side half of the status: whether it answers, and what
// its heartbeat said.
type robotState struct {
	link, servos, mode string
	reboots            int
}

// robotSnapshotLocked copies the robot fields; the caller holds arduinoMutex.
func (s *WebServer) robotSnapshotLocked() robotState {
	return robotState{
		link:    s.stats.robotLink,
		servos:  s.stats.robotServos,
		mode:    s.stats.robotMode,
		reboots: s.stats.robotReboots,
	}
}

// SetRobotInfo records whether the robot is answering (link: "alive", "silent"
// or "unknown") and what its last heartbeat said; servos is "asleep", "awake"
// or empty. It is sent with the next status broadcast.
func (s *WebServer) SetRobotInfo(link, servos, mode string, reboots int) {
	s.stats.arduinoMutex.Lock()
	s.stats.robotLink = link
	s.stats.robotServos = servos
	s.stats.robotMode = mode
	s.stats.robotReboots = reboots
	s.stats.arduinoMutex.Unlock()
}

func (s *WebServer) BroadcastStatus(trackCount int, fps float64, uptimeSec float64) {
	s.broadcastStatus(trackCount, fps, uptimeSec, 0, false)
}

// BroadcastCameraStalled tells the UI that no frames are arriving. Status is
// normally sent from the frame loop, which is silent exactly when the camera
// has stalled, so a separate watchdog calls this instead.
func (s *WebServer) BroadcastCameraStalled(uptimeSec, frameAgeSec float64) {
	s.broadcastStatus(0, 0, uptimeSec, frameAgeSec, true)
}

func (s *WebServer) broadcastStatus(trackCount int, fps, uptimeSec, frameAgeSec float64, stalled bool) {
	s.stats.arduinoMutex.RLock()
	connected := s.stats.arduinoConnected
	robot := s.robotSnapshotLocked()
	s.stats.arduinoMutex.RUnlock()

	state := "Disconnected"
	if connected {
		state = "Connected"
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	hostMemMB := float64(memStats.Alloc) / 1024 / 1024

	s.stats.mutex.Lock()
	s.stats.lastFPS = fps
	s.stats.lastUptimeSec = uptimeSec
	s.stats.lastHostMemMB = hostMemMB
	s.stats.mutex.Unlock()

	s.BroadcastOverlay(OverlayMessage{
		Type: "status",
		Status: &StatusMessage{
			Connected:    s.router.isRunning.Load(),
			FPS:          fps,
			RobotCount:   trackCount,
			ArduinoState: state,
			RobotLink:    robot.link,
			RobotServos:  robot.servos,
			RobotMode:    robot.mode,
			RobotReboots: robot.reboots,
			HostMemoryMB: hostMemMB,
			UptimeSec:    uptimeSec,

			CameraStalled: stalled,
			FrameAgeSec:   frameAgeSec,
		},
	})
}
