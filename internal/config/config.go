package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Robots        []RobotConfig       `yaml:"robots"`
	Planning      PlanningConfig      `yaml:"planning"`
	LocalPlanning LocalPlanningConfig `yaml:"local_planning"`
	AprilTags     AprilTagConfig      `yaml:"april_tags"`
	YOLO          YOLOConfig          `yaml:"yolo"`
	Obstacles     ObstaclesConfig     `yaml:"obstacles"`
	Position      PositionConfig      `yaml:"position"`
	Output        OutputConfig        `yaml:"output"`
	Tracking      TrackingConfig      `yaml:"tracking"`
	Cameras       []CameraConfig      `yaml:"cameras"`
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
	TagID             int     `yaml:"tag_id"`
	Name              string  `yaml:"name"`
	Diameter          float64 `yaml:"diameter"`
	Speed             float64 `yaml:"speed"`
	AvoidanceStrength float64 `yaml:"avoidance_strength"`
	PauseThreshold    float64 `yaml:"pause_threshold"`
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
	ObstacleClasses []string `yaml:"obstacle_classes"`
}

type ObstaclesConfig struct {
	CollisionMargin float64 `yaml:"collision_margin"`
	Path            string  `yaml:"path"`
}

type PositionConfig struct {
	GroundPlaneZ     float64 `yaml:"ground_plane_z"`
	Smoothing        bool    `yaml:"smoothing"`
	SmoothingAlpha   float64 `yaml:"smoothing_alpha"`
	OutlierThreshold float64 `yaml:"outlier_threshold"`
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
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
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
