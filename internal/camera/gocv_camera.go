//go:build gocv

package camera

import (
	"fmt"
	"runtime"
	"sync"

	"gocv.io/x/gocv"
)

// cameraPermissionHint returns macOS-specific guidance when a camera fails to open.
// On macOS, cameras require explicit privacy approval for each application.
func cameraPermissionHint() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	return "\n  On macOS, your terminal app must have camera permission." +
		"\n  Fix: System Settings > Privacy & Security > Camera > enable your terminal (Terminal, iTerm2, etc.)" +
		"\n  You may need to restart your terminal after granting permission."
}

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

	mu          sync.Mutex
	latestFrame *Frame
	frameErr    error
	stopCh      chan struct{}
	wg          sync.WaitGroup
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
		hint := cameraPermissionHint()
		return nil, fmt.Errorf("failed to open camera %d: %w%s", config.CameraID, &CameraError{Message: "camera not available"}, hint)
	}

	cap.Set(gocv.VideoCaptureFrameWidth, float64(width))
	cap.Set(gocv.VideoCaptureFrameHeight, float64(height))
	cap.Set(gocv.VideoCaptureFPS, float64(fps))
	cap.Set(gocv.VideoCaptureBufferSize, 1)

	actualWidth := int(cap.Get(gocv.VideoCaptureFrameWidth))
	actualHeight := int(cap.Get(gocv.VideoCaptureFrameHeight))

	cam := &GoCVCamera{
		device:   cap,
		width:    actualWidth,
		height:   actualHeight,
		fps:      fps,
		running:  true,
		cameraID: config.CameraID,
		isFile:   false,
		stopCh:   make(chan struct{}),
	}
	cam.wg.Add(1)
	go cam.captureLoop()
	return cam, nil
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

	// Minimize frame buffering to reduce video lag from network streams
	cap.Set(gocv.VideoCaptureBufferSize, 1)

	actualWidth := int(cap.Get(gocv.VideoCaptureFrameWidth))
	actualHeight := int(cap.Get(gocv.VideoCaptureFrameHeight))

	cam := &GoCVCamera{
		device:  cap,
		width:   actualWidth,
		height:  actualHeight,
		fps:     fps,
		running: true,
		url:     url,
		isFile:  false,
		stopCh:  make(chan struct{}),
	}
	cam.wg.Add(1)
	go cam.captureLoop()
	return cam, nil
}

func (c *GoCVCamera) captureLoop() {
	defer c.wg.Done()
	img := gocv.NewMat()
	defer func() { _ = img.Close() }()

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		if !c.device.Read(&img) {
			select {
			case <-c.stopCh:
				return
			default:
				c.mu.Lock()
				c.frameErr = &CameraError{Message: "failed to read frame from camera"}
				c.mu.Unlock()
				return
			}
		}

		if img.Empty() {
			continue
		}

		frame := &Frame{
			Data:     img.ToBytes(),
			Width:    img.Cols(),
			Height:   img.Rows(),
			Channels: img.Channels(),
		}

		c.mu.Lock()
		c.latestFrame = frame
		c.frameErr = nil
		c.mu.Unlock()
	}
}

func (c *GoCVCamera) Start() error {
	c.running = true
	return nil
}

func (c *GoCVCamera) Stop() {
	c.running = false
	if c.stopCh != nil {
		close(c.stopCh)
	}
	if c.device != nil && c.device.IsOpened() {
		_ = c.device.Close()
	}
	if c.cap != nil && c.cap.IsOpened() {
		_ = c.cap.Close()
	}
	c.wg.Wait()
}

func (c *GoCVCamera) GetFrame() (*Frame, error) {
	if !c.running {
		return nil, &CameraError{Message: "camera not running"}
	}

	if c.isFile {
		img := gocv.NewMat()
		defer func() { _ = img.Close() }()

		if !c.cap.Read(&img) {
			return nil, &CameraError{Message: "failed to read frame from video"}
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

	c.mu.Lock()
	frame := c.latestFrame
	err := c.frameErr
	c.mu.Unlock()

	if err != nil {
		return nil, err
	}
	if frame == nil {
		return nil, &CameraError{Message: "no frame available yet"}
	}
	return frame, nil
}

func (c *GoCVCamera) GetFrameAsImage() (interface{}, error) {
	if !c.running {
		return nil, &CameraError{Message: "camera not running"}
	}

	if c.isFile {
		img := gocv.NewMat()
		defer func() { _ = img.Close() }()
		if !c.cap.Read(&img) {
			return nil, &CameraError{Message: "failed to read frame"}
		}
		return img, nil
	}

	// For USB cameras, use the cached frame from captureLoop to avoid
	// racing with the background goroutine on c.device.Read().
	frame, err := c.GetFrame()
	if err != nil {
		return nil, err
	}

	mat, err := gocv.NewMatFromBytes(frame.Height, frame.Width, gocv.MatTypeCV8UC3, frame.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to create mat from cached frame: %w", err)
	}
	return mat, nil
}

func (c *GoCVCamera) GetName() string {
	if c.url != "" {
		return fmt.Sprintf("Video: %s", c.url)
	}
	return fmt.Sprintf("Camera %d", c.cameraID)
}

func (c *GoCVCamera) IsConnected() bool {
	if c.device != nil {
		return c.device.IsOpened()
	}
	if c.cap != nil {
		return c.cap.IsOpened()
	}
	return false
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

	if c.isFile {
		img := gocv.NewMat()
		defer func() { _ = img.Close() }()
		if !c.cap.Read(&img) {
			return nil, &CameraError{Message: "failed to read frame from video"}
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

	// For USB cameras, use the cached frame from captureLoop to avoid
	// racing with the background goroutine on c.device.Read().
	frame, err := c.GetFrame()
	if err != nil {
		return nil, err
	}

	mat, err := gocv.NewMatFromBytes(frame.Height, frame.Width, gocv.MatTypeCV8UC3, frame.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to create mat from cached frame: %w", err)
	}
	defer func() { _ = mat.Close() }()

	jpegBytes, err := gocv.IMEncode(".jpg", mat)
	if err != nil {
		return nil, fmt.Errorf("failed to encode frame: %w", err)
	}
	defer jpegBytes.Close()
	return jpegBytes.GetBytes(), nil
}
