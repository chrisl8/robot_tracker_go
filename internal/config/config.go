package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/chrisl8/robot_tracker_go/internal/position"
)

type Config struct {
	Robots        []RobotConfig       `yaml:"robots"`
	Planning      PlanningConfig      `yaml:"planning"`
	LocalPlanning LocalPlanningConfig `yaml:"local_planning"`
	PathExecution PathExecutionConfig `yaml:"path_execution"`
	AprilTags     AprilTagConfig      `yaml:"april_tags"`
	YOLO          YOLOConfig          `yaml:"yolo"`
	Obstacles     ObstaclesConfig     `yaml:"obstacles"`
	Position      PositionConfig      `yaml:"position"`
	Output        OutputConfig        `yaml:"output"`
	Tracking      TrackingConfig      `yaml:"tracking"`
	Cameras       []CameraConfig      `yaml:"cameras"`
	Controller    ControllerConfig    `yaml:"controller"`
	Processing    ProcessingConfig    `yaml:"processing"`
	Foreground    ForegroundConfig    `yaml:"foreground"`
}

// ForegroundConfig configures the background-subtraction detector that finds
// temporary obstacles (anything in the arena that is not the empty floor, a
// robot, or a marked static obstacle). Zero values fall back to the defaults in
// EffectiveForeground.
type ForegroundConfig struct {
	Enabled        *bool   `yaml:"enabled"` // nil = on
	ApplyToPlanner *bool   `yaml:"apply_to_planner"`
	Scale          float64 `yaml:"scale"`
	Threshold      float64 `yaml:"threshold"`
	DarkFactor     float64 `yaml:"dark_factor"`
	MinSizeM       float64 `yaml:"min_size_m"`
	AppearMs       int     `yaml:"appear_ms"`
	VanishMs       int     `yaml:"vanish_ms"`
	TauSec         float64 `yaml:"tau_sec"`
	WarmupSec      float64 `yaml:"warmup_sec"`
	RobotMarginM   float64 `yaml:"robot_margin_m"`
	StaticMarginPx int     `yaml:"static_margin_px"`
	MaxBlobs       int     `yaml:"max_blobs"`
	AbsorbAfterSec float64 `yaml:"absorb_after_sec"`
	GuardFraction  float64 `yaml:"guard_fraction"`
	PadM           float64 `yaml:"pad_m"`
	// PersistBackground saves the learned background to disk and restores it
	// on the next start (when the camera and resolution still match), so a
	// restart resumes instantly instead of re-learning. nil = on.
	PersistBackground  *bool   `yaml:"persist_background"`
	PersistIntervalSec float64 `yaml:"persist_interval_sec"`
	// Shadow suppression: a darkened pixel whose colour is still just the
	// background colour scaled down (within these bounds) is a cast shadow,
	// not an object. See detection.isShadowColor.
	ShadowAlphaMin  float64 `yaml:"shadow_alpha_min"`
	ShadowAlphaMax  float64 `yaml:"shadow_alpha_max"`
	ShadowChromaMax float64 `yaml:"shadow_chroma_max"`
}

// ForegroundSettings is ForegroundConfig with every default applied.
type ForegroundSettings struct {
	Enabled            bool
	ApplyToPlanner     bool
	Scale              float64
	Threshold          float64
	DarkFactor         float64
	MinSizeM           float64
	AppearMs           int
	VanishMs           int
	TauSec             float64
	WarmupSec          float64
	RobotMarginM       float64
	StaticMarginPx     int
	MaxBlobs           int
	AbsorbAfterSec     float64
	GuardFraction      float64
	PadM               float64
	PersistBackground  bool
	PersistIntervalSec float64
	ShadowAlphaMin     float64
	ShadowAlphaMax     float64
	ShadowChromaMax    float64
}

// EffectiveForeground returns the foreground settings with defaults filled in.
// Detection is on by default; steering the planner around detected obstacles is
// off by default (shadow mode) until it is turned on in config or the UI.
func (c *Config) EffectiveForeground() ForegroundSettings {
	var f ForegroundConfig
	if c != nil {
		f = c.Foreground
	}
	orF := func(v, def float64) float64 {
		if v > 0 {
			return v
		}
		return def
	}
	orI := func(v, def int) int {
		if v > 0 {
			return v
		}
		return def
	}
	s := ForegroundSettings{
		Enabled:            true,
		Scale:              orF(f.Scale, 0.5),
		Threshold:          orF(f.Threshold, 22),
		DarkFactor:         orF(f.DarkFactor, 1.4),
		MinSizeM:           orF(f.MinSizeM, 0.05),
		AppearMs:           orI(f.AppearMs, 400),
		VanishMs:           orI(f.VanishMs, 1500),
		TauSec:             orF(f.TauSec, 60),
		WarmupSec:          orF(f.WarmupSec, 3),
		RobotMarginM:       orF(f.RobotMarginM, 0.06),
		StaticMarginPx:     orI(f.StaticMarginPx, 6),
		MaxBlobs:           orI(f.MaxBlobs, 8),
		AbsorbAfterSec:     f.AbsorbAfterSec, // 0 = never (objects stay obstacles)
		GuardFraction:      orF(f.GuardFraction, 0.25),
		PadM:               orF(f.PadM, 0.02),
		PersistBackground:  true,
		PersistIntervalSec: orF(f.PersistIntervalSec, 30),
		// Defaults must match detection.DefaultForegroundParams.
		ShadowAlphaMin:  orF(f.ShadowAlphaMin, 0.15),
		ShadowAlphaMax:  orF(f.ShadowAlphaMax, 0.98),
		ShadowChromaMax: orF(f.ShadowChromaMax, 0.20),
	}
	if f.Enabled != nil {
		s.Enabled = *f.Enabled
	}
	if f.ApplyToPlanner != nil {
		s.ApplyToPlanner = *f.ApplyToPlanner
	}
	if f.PersistBackground != nil {
		s.PersistBackground = *f.PersistBackground
	}
	return s
}

// DefaultMaxFPS is the frame-processing cap used when processing.max_fps is
// unset. The path-following controller is tuned in frames around 12 fps; an
// uncapped loop would run about twice as fast and burn far more CPU.
const DefaultMaxFPS = 15

// ProcessingConfig controls how hard the tracker works.
type ProcessingConfig struct {
	MaxFPS int `yaml:"max_fps"`
}

// EffectiveMaxFPS returns the configured cap, or DefaultMaxFPS when unset or
// not positive.
func (c *Config) EffectiveMaxFPS() int {
	if c == nil || c.Processing.MaxFPS <= 0 {
		return DefaultMaxFPS
	}
	return c.Processing.MaxFPS
}

type ControllerConfig struct {
	Enabled          bool         `yaml:"enabled"`
	Serial           SerialConfig `yaml:"serial"`
	CommandInterval  float64      `yaml:"command_interval"`
	HeartbeatTimeout float64      `yaml:"heartbeat_timeout"`
}

type SerialConfig struct {
	Port     string  `yaml:"port"`
	BaudRate int     `yaml:"baudrate"`
	Timeout  float64 `yaml:"timeout"`
}

type CameraConfig struct {
	Type     string `yaml:"type"`
	Name     string `yaml:"name"`
	CameraID int    `yaml:"camera_id"`
	URL      string `yaml:"url"`
	Width    int    `yaml:"width"`
	Height   int    `yaml:"height"`
	FPS      int    `yaml:"fps"`
}

type RobotConfig struct {
	TagID                int     `yaml:"tag_id"`
	Name                 string  `yaml:"name"`
	Diameter             float64 `yaml:"diameter"`
	Speed                float64 `yaml:"speed"`
	AvoidanceStrength    float64 `yaml:"avoidance_strength"`
	PauseThreshold       float64 `yaml:"pause_threshold"`
	HeadingOffsetDegrees float64 `yaml:"heading_offset_degrees"`
	CenterOffsetX        float64 `yaml:"center_offset_x"`
	CenterOffsetY        float64 `yaml:"center_offset_y"`
}

type PlanningConfig struct {
	StepSize       float64         `yaml:"step_size"`
	ReplanInterval float64         `yaml:"replan_interval"`
	Avoidance      AvoidanceConfig `yaml:"avoidance"`
}

type AvoidanceConfig struct {
	SafetyMargin float64 `yaml:"safety_margin"`
}

type LocalPlanningConfig struct {
	Enabled         bool        `yaml:"enabled"`
	SafetyMargin    float64     `yaml:"safety_margin"`
	TimeHorizon     float64     `yaml:"time_horizon"`
	Debug           DebugConfig `yaml:"debug"`
	ObstacleClasses []string    `yaml:"obstacle_classes"`
	MinConfidence   float64     `yaml:"min_obstacle_confidence"`
}

type PathExecutionConfig struct {
	WaypointThreshold    float64 `yaml:"waypoint_threshold"`
	MaxSpeed             float64 `yaml:"max_speed"`
	TurnSpeed            float64 `yaml:"turn_speed"`
	CommandIntervalMs    int     `yaml:"command_interval_ms"`
	SpinThresholdDeg     float64 `yaml:"spin_threshold_deg"`
	BurstFrames          int     `yaml:"burst_frames"`
	MaxWaitFrames        int     `yaml:"max_wait_frames"`
	ForwardThresholdDeg  float64 `yaml:"forward_threshold_deg"`
	TrackingLostTimeoutS float64 `yaml:"tracking_lost_timeout_s"`
}

type DebugConfig struct {
	Enabled bool `yaml:"enabled"`
}

type AprilTagConfig struct {
	Family       string  `yaml:"family"`
	TagSize      float64 `yaml:"tag_size"`
	NThreads     int     `yaml:"nthreads"`
	QuadDecimate float64 `yaml:"quad_decimate"`
}

type YOLOConfig struct {
	ModelPath       string   `yaml:"model"`
	InputSize       int      `yaml:"input_size"`
	ConfThres       float64  `yaml:"conf_thres"`
	IOUThres        float64  `yaml:"iou_thres"`
	Device          string   `yaml:"device"`
	MinObstacleSize float64  `yaml:"min_obstacle_size"`
	PixelsPerMeter  float64  `yaml:"pixels_per_meter"`
	RelevantClasses []string `yaml:"classes"`
}

type ObstaclesConfig struct {
	CollisionMargin float64 `yaml:"collision_margin"`
	File            string  `yaml:"file"`
	Path            string  `yaml:"path"` // Deprecated: use File instead
	Enabled         bool    `yaml:"enabled"`
	DisplayColor    string  `yaml:"display_color"`
}

func (c *ObstaclesConfig) GetPath() string {
	if c.File != "" {
		return c.File
	}
	return c.Path
}

type StaticObstacleConfig struct {
	Enabled      bool   `yaml:"enabled"`
	DisplayColor string `yaml:"display_color"`
}

type StaticObstacle struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	PixelTopLeft     [2]int     `json:"pixel_top_left"`
	PixelBottomRight [2]int     `json:"pixel_bottom_right"`
	WorldTopLeft     [2]float64 `json:"world_top_left"`
	WorldBottomRight [2]float64 `json:"world_bottom_right"`
	Clearance        float64    `json:"clearance"`
}

type ObstaclesYAML struct {
	Version   int                `yaml:"version"`
	Obstacles []ObstacleYAMLItem `yaml:"obstacles"`
}

type ObstacleYAMLItem struct {
	Name      string       `yaml:"name"`
	Pixels    PixelBoxYAML `yaml:"pixels"`
	World     WorldBoxYAML `yaml:"world"`
	Clearance float64      `yaml:"clearance"`
}

type PixelBoxYAML struct {
	TopLeft     [2]int `yaml:"top_left"`
	BottomRight [2]int `yaml:"bottom_right"`
}

type WorldBoxYAML struct {
	TopLeft     [2]float64 `yaml:"top_left"`
	BottomRight [2]float64 `yaml:"bottom_right"`
}

type PositionConfig struct {
	GroundPlaneZ          float64 `yaml:"ground_plane_z"`
	Smoothing             bool    `yaml:"smoothing"`
	SmoothingAlpha        float64 `yaml:"smoothing_alpha"`
	OutlierThreshold      float64 `yaml:"outlier_threshold"`
	HeadingSmoothingAlpha float64 `yaml:"heading_smoothing_alpha"`
	HeadingMaxRateDeg     float64 `yaml:"heading_max_rate_deg"`
}

type TrackingConfig struct {
	TrackThresh float64 `yaml:"track_thresh"`
	TrackBuffer int     `yaml:"track_buffer"`
	MatchThresh float64 `yaml:"match_thresh"`
	FrameRate   int     `yaml:"frame_rate"`
	MOT20       bool    `yaml:"mot20"`
	MinBoxArea  int     `yaml:"min_box_area"`
	CameraFPS   int     `yaml:"camera_fps"`
}

type OutputConfig struct {
	Log LogConfig `yaml:"log"`
}

type LogConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

func Load(path string) (*Config, error) {
	// #nosec G304
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	for _, robot := range cfg.Robots {
		if position.IsTargetTagID(robot.TagID) {
			return nil, fmt.Errorf("robot %q uses tag_id %d, which is reserved for the calibration target (%d-%d)",
				robot.Name, robot.TagID, position.TargetIDMin, position.TargetIDMax)
		}
	}

	return &cfg, nil
}

func (c *Config) GetRobotByTagID(tagID int) *RobotConfig {
	for i := range c.Robots {
		if c.Robots[i].TagID == tagID {
			return &c.Robots[i]
		}
	}
	return nil
}

func (c *Config) GetRobotByName(name string) *RobotConfig {
	for i := range c.Robots {
		if c.Robots[i].Name == name {
			return &c.Robots[i]
		}
	}
	return nil
}

func (c *Config) GetPrimaryCamera() *CameraConfig {
	for i := range c.Cameras {
		if c.Cameras[i].Type == "ip" || c.Cameras[i].CameraID >= 0 {
			return &c.Cameras[i]
		}
	}
	if len(c.Cameras) > 0 {
		return &c.Cameras[0]
	}
	return nil
}
