package camera

import (
	"fmt"
	"strings"
)

type IPCamera struct {
	config    CameraConfig
	name      string
	width     int
	height    int
	fps       int
	connected bool
	running   bool
	url       string
}

func NewIPCamera(url string, width, height, fps int) (*IPCamera, error) {
	if width == 0 {
		width = DefaultWidth
	}
	if height == 0 {
		height = DefaultHeight
	}
	if fps == 0 {
		fps = DefaultFPS
	}

	name := "IP Camera"
	if strings.Contains(url, "/video") {
		name = "DroidCam IP"
	}

	return &IPCamera{
		url:       url,
		name:      name,
		width:     width,
		height:    height,
		fps:       fps,
		connected: false,
		running:   false,
	}, nil
}

func (c *IPCamera) Start() error {
	c.connected = true
	c.running = true
	return nil
}

func (c *IPCamera) Stop() {
	c.running = false
	c.connected = false
}

func (c *IPCamera) GetFrame() (*Frame, error) {
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

func (c *IPCamera) GetName() string {
	return c.name
}

func (c *IPCamera) IsConnected() bool {
	return c.connected && c.running
}

func (c *IPCamera) GetWidth() int {
	return c.width
}

func (c *IPCamera) GetHeight() int {
	return c.height
}

func (c *IPCamera) GetFPS() int {
	return c.fps
}

func (c *IPCamera) GetURL() string {
	return c.url
}

type ConnectionTimeoutError struct {
	URL string
}

func (e *ConnectionTimeoutError) Error() string {
	return fmt.Sprintf("connection to %s timed out", e.URL)
}
