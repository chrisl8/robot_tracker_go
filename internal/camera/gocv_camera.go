//go:build gocv

package camera

import (
	"fmt"

	"gocv.io/x/gocv"
)

type GoCVCamera struct {
	cap      *gocv.VideoCapture
	device   *gocv.VideoCapture
	width    int
	height   int
	fps      int
	running  bool
	cameraID int
	url      string
	isFile   bool
}

func NewGoCVCamera(config CameraConfig) (*GoCVCamera, error) {
	width := config.Width
	height := config.Height
	fps := config.FPS

	if width == 0 {
		width = DefaultWidth
	}
	if height == 0 {
		height = DefaultHeight
	}
	if fps == 0 {
		fps = DefaultFPS
	}

	cap, err := gocv.VideoCaptureDevice(config.CameraID)
	if err != nil || cap == nil || !cap.IsOpened() {
		return nil, fmt.Errorf("failed to open camera %d: %w", config.CameraID, &CameraError{Message: "camera not available"})
	}

	cap.Set(gocv.VideoCaptureFrameWidth, float64(width))
	cap.Set(gocv.VideoCaptureFrameHeight, float64(height))
	cap.Set(gocv.VideoCaptureFPS, float64(fps))

	actualWidth := int(cap.Get(gocv.VideoCaptureFrameWidth))
	actualHeight := int(cap.Get(gocv.VideoCaptureFrameHeight))

	return &GoCVCamera{
		device:   cap,
		width:    actualWidth,
		height:   actualHeight,
		fps:      fps,
		running:  true,
		cameraID: config.CameraID,
		isFile:   false,
	}, nil
}

func NewGoCVIPCamera(url string, width, height, fps int) (*GoCVCamera, error) {
	if width == 0 {
		width = DefaultWidth
	}
	if height == 0 {
		height = DefaultHeight
	}
	if fps == 0 {
		fps = DefaultFPS
	}

	cap, err := gocv.VideoCaptureFile(url)
	if err != nil || cap == nil || !cap.IsOpened() {
		return nil, fmt.Errorf("failed to open video file: %s: %w", url, &CameraError{Message: "file not accessible"})
	}

	actualWidth := int(cap.Get(gocv.VideoCaptureFrameWidth))
	actualHeight := int(cap.Get(gocv.VideoCaptureFrameHeight))

	return &GoCVCamera{
		cap:     cap,
		width:   actualWidth,
		height:  actualHeight,
		fps:     fps,
		running: true,
		url:     url,
		isFile:  true,
	}, nil
}

func (c *GoCVCamera) Start() error {
	c.running = true
	return nil
}

func (c *GoCVCamera) Stop() {
	c.running = false
	if c.device != nil && c.device.IsOpened() {
		c.device.Close()
	}
	if c.cap != nil && c.cap.IsOpened() {
		c.cap.Close()
	}
}

func (c *GoCVCamera) GetFrame() (*Frame, error) {
	if !c.running {
		return nil, &CameraError{Message: "camera not running"}
	}

	img := gocv.NewMat()
	defer img.Close()

	if c.isFile {
		if !c.cap.Read(&img) {
			return nil, &CameraError{Message: "failed to read frame from video"}
		}
	} else {
		if !c.device.Read(&img) {
			return nil, &CameraError{Message: "failed to read frame from camera"}
		}
	}

	if img.Empty() {
		return nil, &CameraError{Message: "empty frame"}
	}

	return &Frame{
		Data:     img.ToBytes(),
		Width:    img.Cols(),
		Height:   img.Rows(),
		Channels: img.Channels(),
	}, nil
}

func (c *GoCVCamera) GetFrameAsImage() (interface{}, error) {
	if !c.running {
		return nil, &CameraError{Message: "camera not running"}
	}

	img := gocv.NewMat()
	defer img.Close()

	if c.isFile {
		if !c.cap.Read(&img) {
			return nil, &CameraError{Message: "failed to read frame"}
		}
	} else {
		if !c.device.Read(&img) {
			return nil, &CameraError{Message: "failed to read frame"}
		}
	}

	return img, nil
}

func (c *GoCVCamera) GetName() string {
	if c.isFile {
		return fmt.Sprintf("Video: %s", c.url)
	}
	return fmt.Sprintf("Camera %d", c.cameraID)
}

func (c *GoCVCamera) IsConnected() bool {
	if c.isFile {
		return c.cap.IsOpened()
	}
	return c.device.IsOpened()
}

func (c *GoCVCamera) GetWidth() int {
	return c.width
}

func (c *GoCVCamera) GetHeight() int {
	return c.height
}

func (c *GoCVCamera) GetFPS() int {
	return c.fps
}

func (c *GoCVCamera) IsRunning() bool {
	return c.running
}

func (c *GoCVCamera) GetRawJPEG() ([]byte, error) {
	if !c.running {
		return nil, &CameraError{Message: "camera not running"}
	}

	img := gocv.NewMat()
	defer img.Close()

	if c.isFile {
		if !c.cap.Read(&img) {
			return nil, &CameraError{Message: "failed to read frame from video"}
		}
	} else {
		if !c.device.Read(&img) {
			return nil, &CameraError{Message: "failed to read frame from camera"}
		}
	}

	if img.Empty() {
		return nil, &CameraError{Message: "empty frame"}
	}

	jpegBytes, err := gocv.IMEncode(".jpg", img)
	if err != nil {
		return nil, fmt.Errorf("failed to encode frame: %v", err)
	}
	defer jpegBytes.Close()

	return jpegBytes.GetBytes(), nil
}
