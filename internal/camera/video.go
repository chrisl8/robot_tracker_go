package camera

import (
	"path"
)

type VideoFileCamera struct {
	filepath   string
	name       string
	width      int
	height     int
	fps        int
	frameCount int
	loop       bool
	finished   bool
	running    bool
}

func NewVideoFileCamera(filePath string, loop bool) *VideoFileCamera {
	return &VideoFileCamera{
		filepath: filePath,
		name:     path.Base(filePath),
		loop:     loop,
		finished: false,
		running:  false,
	}
}

func (c *VideoFileCamera) Start() error {
	c.running = true
	c.width = 640
	c.height = 480
	c.fps = 30
	c.frameCount = 0
	return nil
}

func (c *VideoFileCamera) Stop() {
	c.running = false
	c.finished = false
}

func (c *VideoFileCamera) GetFrame() (*Frame, error) {
	if !c.running {
		return nil, &CameraError{Message: "video file not open"}
	}

	if c.finished {
		if c.loop {
			c.finished = false
		} else {
			return nil, &CameraError{Message: "end of video"}
		}
	}

	return &Frame{
		Data:     make([]byte, c.width*c.height*3),
		Width:    c.width,
		Height:   c.height,
		Channels: 3,
	}, nil
}

func (c *VideoFileCamera) GetName() string {
	return c.name
}

func (c *VideoFileCamera) IsConnected() bool {
	return c.running && !c.finished
}

func (c *VideoFileCamera) GetWidth() int {
	return c.width
}

func (c *VideoFileCamera) GetHeight() int {
	return c.height
}

func (c *VideoFileCamera) GetFPS() int {
	return c.fps
}

func (c *VideoFileCamera) GetFrameCount() int {
	return c.frameCount
}

func (c *VideoFileCamera) GetPosition() float64 {
	return 0
}

func (c *VideoFileCamera) IsLooping() bool {
	return c.loop
}

func (c *VideoFileCamera) IsFinished() bool {
	return c.finished
}

func (c *VideoFileCamera) GetFilePath() string {
	return c.filepath
}

func (c *VideoFileCamera) GetFileName() string {
	return path.Base(c.filepath)
}
