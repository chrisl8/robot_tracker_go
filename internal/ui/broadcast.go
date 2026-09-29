//go:build gocv

package ui

import (
	"github.com/chrisl8/robot_tracker_go/internal/config"
	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// BroadcastTempObstacles sends the temporary obstacles and detector state to
// every connected UI.
func (s *WebServer) BroadcastTempObstacles(msg TempObstaclesMessage) {
	if msg.Obstacles == nil {
		msg.Obstacles = []TempObstacleResponse{}
	}
	s.BroadcastOverlay(OverlayMessage{Type: "temp_obstacles", TempObstacles: &msg})
}

func (s *WebServer) BroadcastTracks(tracks []tracking.Track, robots []config.RobotConfig, robotCommands map[int]string) {
	trackMessages := make([]TrackMessage, 0, len(tracks))
	for _, track := range tracks {
		bbox := []int{track.Bbox[0], track.Bbox[1], track.Bbox[2], track.Bbox[3]}
		history := make([][2]int, len(track.History))
		for i, hp := range track.History {
			history[i] = [2]int{hp.Bbox[0], hp.Bbox[1]}
		}
		msg := TrackMessage{
			ID:          track.TrackID,
			TagID:       track.TagID,
			BBox:        bbox,
			History:     history,
			Confidence:  track.Confidence,
			State:       track.StateString(),
			PixelRadius: &track.PixelRadius,
		}
		if track.TagID != nil {
			for _, rc := range robots {
				if rc.TagID == *track.TagID {
					msg.Configured = true
					msg.Name = rc.Name
					break
				}
			}
		}
		if track.PixelRadius <= 0 {
			msg.PixelRadius = nil
		}
		if track.TagID != nil && track.Corners != [4][2]float64{} {
			heading := track.Heading
			corners := track.Corners
			headingOffset := track.HeadingOffset
			msg.Heading = &heading
			msg.Corners = &corners
			msg.HeadingOffset = &headingOffset
		}
		if track.TagID != nil {
			if ms, ok := robotCommands[*track.TagID]; ok && ms != "" {
				msg.MotionState = &ms
			}
		}
		trackMessages = append(trackMessages, msg)
	}
	s.BroadcastOverlay(OverlayMessage{
		Type:   "tracks",
		Tracks: &TracksMessage{Tracks: trackMessages},
	})
}

func (s *WebServer) BroadcastPaths() {
	if s.Callbacks.OnPathsChanged == nil {
		utils.Debugf("BroadcastPaths: OnPathsChanged is nil, skipping")
		return
	}
	if s.calibration.positionEstimator == nil {
		utils.Debugf("BroadcastPaths: positionEstimator is nil, skipping")
		return
	}

	paths := s.Callbacks.OnPathsChanged()
	if len(paths) == 0 {
		s.BroadcastOverlay(OverlayMessage{
			Type:  "paths",
			Paths: &PathsMessage{Paths: []PathMessage{}},
		})
		return
	}

	utils.Debugf("BroadcastPaths: got %d paths, converting to pixels", len(paths))
	pathMessages := make([]PathMessage, 0, len(paths))
	for robotID, path := range paths {
		if len(path) == 0 {
			continue
		}

		pixels := make([][2]int, len(path))
		for i, wp := range path {
			px, py := s.calibration.positionEstimator.WorldToPixel(position.Point2D{X: wp[0], Y: wp[1]})
			pixels[i] = [2]int{px, py}
		}

		brightColors := []string{"#FFFF00", "#00FF00", "#FF00FF", "#00FFFF", "#FF6600", "#FF0066", "#66FF00", "#0099FF"}
		color := brightColors[robotID%len(brightColors)]
		pathMessages = append(pathMessages, PathMessage{
			RobotID: robotID,
			Points:  pixels,
			Color:   color,
		})
		utils.Debugf("BroadcastPaths: robot %d has %d waypoints, first pixel=(%d,%d)", robotID, len(pixels), pixels[0][0], pixels[0][1])
	}

	utils.Debugf("BroadcastPaths: broadcasting %d path messages via WebSocket", len(pathMessages))
	s.BroadcastOverlay(OverlayMessage{
		Type:  "paths",
		Paths: &PathsMessage{Paths: pathMessages},
	})
}
