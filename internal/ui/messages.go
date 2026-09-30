//go:build gocv

package ui

type OverlayMessage struct {
	Type          string                    `json:"type"`
	Tracks        *TracksMessage            `json:"tracks,omitempty"`
	Paths         *PathsMessage             `json:"paths,omitempty"`
	Status        *StatusMessage            `json:"status,omitempty"`
	Calibration   *CalibrationStatusMessage `json:"calibration,omitempty"`
	Obstacles     *ObstaclesMessage         `json:"obstacles,omitempty"`
	Destination   *DestinationMessage       `json:"destination,omitempty"`
	TempObstacles *TempObstaclesMessage     `json:"temp_obstacles,omitempty"`
	Control       *ControlStateResponse     `json:"control,omitempty"`
}

// TempObstacleResponse is one temporary (detected, not user-marked) obstacle.
type TempObstacleResponse struct {
	ID               string     `json:"id"`
	PixelTopLeft     [2]int     `json:"pixel_top_left"`
	PixelBottomRight [2]int     `json:"pixel_bottom_right"`
	WorldTopLeft     [2]float64 `json:"world_top_left"`
	WorldBottomRight [2]float64 `json:"world_bottom_right"`
	// PixelQuad/WorldQuad are the obstacle's exact (possibly rotated)
	// footprint, four corners in order, omitted when unavailable (in which
	// case the UI should fall back to the rectangle above). world_top_left/
	// world_bottom_right/pixel_* stay populated as that shape's bounding box.
	PixelQuad [][2]int     `json:"pixel_quad,omitempty"`
	WorldQuad [][2]float64 `json:"world_quad,omitempty"`
}

// TempObstaclesMessage carries the current temporary obstacles and the
// detector's state to the UI.
type TempObstaclesMessage struct {
	Obstacles []TempObstacleResponse `json:"obstacles"`
	Applied   bool                   `json:"applied"`
	Warming   bool                   `json:"warming"`
	Guarded   bool                   `json:"guarded"`
	Enabled   bool                   `json:"enabled"`
}

// ForegroundState is the detector's state as returned by GET /api/foreground/state.
type ForegroundState struct {
	Enabled bool `json:"enabled"`
	Applied bool `json:"applied"`
	Warming bool `json:"warming"`
	Guarded bool `json:"guarded"`
	Count   int  `json:"count"`
	// ShadowSuppressed is how many pixels the last frame were reclassified
	// from would-be foreground to background as a cast shadow — diagnostic,
	// for correlating a flagged obstacle with heavy shadow activity nearby.
	ShadowSuppressed int `json:"shadow_suppressed"`
}

type TracksMessage struct {
	Tracks []TrackMessage `json:"tracks"`
}

type ObstaclesMessage struct {
	Obstacles []ObstacleResponse `json:"obstacles"`
	Count     int                `json:"count"`
}

type TrackMessage struct {
	ID            int            `json:"id"`
	TagID         *int           `json:"tag_id,omitempty"`
	BBox          []int          `json:"bbox"`
	History       [][2]int       `json:"history"`
	Color         string         `json:"color"`
	Confidence    float64        `json:"confidence"`
	State         string         `json:"state"`
	Configured    bool           `json:"configured"`
	Name          string         `json:"name,omitempty"`
	PixelRadius   *float64       `json:"pixel_radius,omitempty"`
	Heading       *float64       `json:"heading,omitempty"`
	Corners       *[4][2]float64 `json:"corners,omitempty"`
	HeadingOffset *float64       `json:"heading_offset,omitempty"`
	MotionState   *string        `json:"motion_state,omitempty"`
}

type PathMessage struct {
	RobotID int      `json:"robot_id"`
	Points  [][2]int `json:"points"`
	Color   string   `json:"color"`
}

type PathsMessage struct {
	Paths []PathMessage `json:"paths"`
}

type StatusMessage struct {
	Connected    bool    `json:"connected"`
	FPS          float64 `json:"fps"`
	RobotCount   int     `json:"robotCount"`
	ArduinoState string  `json:"arduinoState"`
	// RobotLink is "alive", "silent" or "unknown". "silent" while ArduinoState
	// is Connected means the gamepad is fine but the robot is off or out of range.
	RobotLink    string  `json:"robotLink"`
	HostMemoryMB float64 `json:"hostMemoryMB,omitempty"`
	UptimeSec    float64 `json:"uptimeSec,omitempty"`
	// CameraStalled is true when no video frame has been processed for a couple
	// of seconds; FPS is then 0 and FrameAgeSec says for how long.
	CameraStalled bool    `json:"cameraStalled,omitempty"`
	FrameAgeSec   float64 `json:"frameAgeSec,omitempty"`
}

type DestinationMessage struct {
	RobotID int  `json:"robot_id"`
	X       int  `json:"x"`
	Y       int  `json:"y"`
	Valid   bool `json:"valid"`
}
