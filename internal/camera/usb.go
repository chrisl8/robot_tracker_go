package camera

import (
	"fmt"
)

type USBCamera struct {
	config    CameraConfig
	name      string
	width     int
	height    int
	fps       int
	connected bool
	running   bool
}

func NewUSBCamera(config CameraConfig) *USBCamera {
	if config.Width == 0 {
		config.Width = DefaultWidth
	}
	if config.Height == 0 {
		config.Height = DefaultHeight
	}
	if config.FPS == 0 {
		config.FPS = DefaultFPS
	}
	if config.Backend == "" {
		config.Backend = "dshow"
	}

	name := fmt.Sprintf("USB Camera %d", config.CameraID)
	if config.Name != "" {
		name = config.Name
	}

	return &USBCamera{
		config:    config,
		name:      name,
		width:     config.Width,
		height:    config.Height,
		fps:       config.FPS,
		connected: false,
		running:   false,
	}
}

func (c *USBCamera) Start() error {
	c.connected = true
	c.running = true
	return nil
}

func (c *USBCamera) Stop() {
	c.running = false
	c.connected = false
}

func (c *USBCamera) GetFrame() (*Frame, error) {
	if !c.connected || !c.running {
		return nil, &CameraError{Message: "camera not connected"}
	}

	return &Frame{
		Data:     make([]byte, c.width*c.height*3),
		Width:    c.width,
		Height:   c.height,
		Channels: 3,
	}, nil
}

func (c *USBCamera) GetName() string {
	return c.name
}

func (c *USBCamera) IsConnected() bool {
	return c.connected && c.running
}

func (c *USBCamera) GetWidth() int {
	return c.width
}

func (c *USBCamera) GetHeight() int {
	return c.height
}

func (c *USBCamera) GetFPS() int {
	return c.fps
}

func (c *USBCamera) SetWidth(width int) {
	c.width = width
}

func (c *USBCamera) SetHeight(height int) {
	c.height = height
}

func (c *USBCamera) SetFPS(fps int) {
	c.fps = fps
}

func (c *USBCamera) GetBackendName() string {
	return c.config.Backend
}

func (c *USBCamera) GetCameraID() int {
	return c.config.CameraID
}
