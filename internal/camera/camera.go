package camera

import (
	"time"
)

type Frame struct {
	Data     []byte
	Width    int
	Height   int
	Channels int
}

type Camera interface {
	Start() error
	Stop()
	GetFrame() (*Frame, error)
	GetName() string
	IsConnected() bool
	GetWidth() int
	GetHeight() int
	GetFPS() int
}

type CameraConfig struct {
	Type     string  `yaml:"type"`
	Name     string  `yaml:"name"`
	CameraID int     `yaml:"camera_id"`
	URL      string  `yaml:"url"`
	Width    int     `yaml:"width"`
	Height   int     `yaml:"height"`
	FPS      int     `yaml:"fps"`
	Backend  string  `yaml:"backend"`
	Timeout  float64 `yaml:"timeout"`
}

type CameraError struct {
	Message string
}

func (e *CameraError) Error() string {
	return e.Message
}

const DefaultWidth = 640
const DefaultHeight = 480
const DefaultFPS = 30
const DefaultTimeout = 10.0 * time.Second
