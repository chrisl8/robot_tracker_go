//go:build gocv

package camera

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"gocv.io/x/gocv"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// frameSource is the part of gocv.VideoCapture the capture loop uses, so
// recovery can be tested without hardware.
type frameSource interface {
	Read(m *gocv.Mat) bool
	Close() error
	IsOpened() bool
}

// Defaults for recovering from a stalled stream: reads are retried every
// retryDelay, and after reopenAfter consecutive failures (~2 s) the device is
// closed and reopened with backoff.
const (
	defaultReopenAfter = 40
	defaultRetryDelay  = 50 * time.Millisecond
	defaultBackoffMin  = time.Second
	defaultBackoffMax  = 10 * time.Second
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
	device   frameSource // guarded by mu once the capture loop runs
	open     func() (frameSource, error)
	width    int
	height   int
	fps      int
	running  bool // guarded by mu
	cameraID int
	url      string
	isFile   bool

	reopenAfter int
	retryDelay  time.Duration
	backoffMin  time.Duration
	backoffMax  time.Duration

	mu          sync.Mutex
	latestFrame *Frame
	frameErr    error
	stopCh      chan struct{}
	stopOnce    sync.Once
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

	cap, err := openUSBCapture(config.CameraID, width, height, fps)
	if err != nil {
		return nil, err
	}

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
		open: func() (frameSource, error) {
			c, err := openUSBCapture(config.CameraID, width, height, fps)
			if err != nil {
				return nil, err
			}
			return c, nil
		},
	}
	cam.setRecoveryDefaults()
	cam.wg.Add(1)
	go cam.captureLoop()
	return cam, nil
}

func openUSBCapture(cameraID, width, height, fps int) (*gocv.VideoCapture, error) {
	cap, err := gocv.VideoCaptureDevice(cameraID)
	if err != nil || cap == nil || !cap.IsOpened() {
		if cap != nil {
			_ = cap.Close() // a failed open still allocates a native capture
		}
		hint := cameraPermissionHint()
		return nil, fmt.Errorf("failed to open camera %d: %w%s", cameraID, &CameraError{Message: "camera not available"}, hint)
	}

	cap.Set(gocv.VideoCaptureFrameWidth, float64(width))
	cap.Set(gocv.VideoCaptureFrameHeight, float64(height))
	cap.Set(gocv.VideoCaptureFPS, float64(fps))
	cap.Set(gocv.VideoCaptureBufferSize, 1)
	return cap, nil
}

func (c *GoCVCamera) setRecoveryDefaults() {
	c.reopenAfter = defaultReopenAfter
	c.retryDelay = defaultRetryDelay
	c.backoffMin = defaultBackoffMin
	c.backoffMax = defaultBackoffMax
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

	cap, err := openStreamCapture(url)
	if err != nil {
		return nil, err
	}

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
		open: func() (frameSource, error) {
			c, err := openStreamCapture(url)
			if err != nil {
				return nil, err
			}
			return c, nil
		},
	}
	cam.setRecoveryDefaults()
	cam.wg.Add(1)
	go cam.captureLoop()
	return cam, nil
}

func openStreamCapture(url string) (*gocv.VideoCapture, error) {
	cap, err := gocv.VideoCaptureFile(url)
	if err != nil || cap == nil || !cap.IsOpened() {
		if cap != nil {
			_ = cap.Close() // a failed open still allocates a native capture
		}
		return nil, fmt.Errorf("failed to open video file: %s: %w", url, &CameraError{Message: "file not accessible"})
	}

	// Minimize frame buffering to reduce video lag from network streams
	cap.Set(gocv.VideoCaptureBufferSize, 1)
	return cap, nil
}

func (c *GoCVCamera) captureLoop() {
	defer c.wg.Done()
	img := gocv.NewMat()
	defer func() { _ = img.Close() }()

	failures := 0
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		dev := c.currentDevice()
		if dev == nil || !dev.Read(&img) || img.Empty() {
			failures++
			if failures >= 5 {
				c.mu.Lock()
				c.frameErr = &CameraError{Message: "failed to read frame from camera"}
				c.mu.Unlock()
			}
			if c.open != nil && failures >= c.reopenAfter {
				if !c.reopenDevice() {
					return
				}
				failures = 0
				continue
			}
			select {
			case <-c.stopCh:
				return
			case <-time.After(c.retryDelay):
			}
			continue
		}
		failures = 0

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

func (c *GoCVCamera) currentDevice() frameSource {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.device
}

// reopenDevice closes the stalled device and reopens it, retrying with
// backoff until it succeeds. It returns false if the camera was stopped
// meanwhile.
func (c *GoCVCamera) reopenDevice() bool {
	utils.Logf("Camera stopped delivering frames; reopening it...")

	c.mu.Lock()
	old := c.device
	c.device = nil
	c.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}

	backoff := c.backoffMin
	for attempt := 1; ; attempt++ {
		select {
		case <-c.stopCh:
			return false
		default:
		}

		dev, err := c.open()
		if err == nil {
			c.mu.Lock()
			c.device = dev
			c.mu.Unlock()
			utils.Logf("Camera reopened after %d attempt(s)", attempt)
			return true
		}
		if attempt == 1 || attempt%10 == 0 {
			utils.Logf("Camera reopen attempt %d failed: %v", attempt, err)
		}

		select {
		case <-c.stopCh:
			return false
		case <-time.After(backoff):
		}
		if backoff < c.backoffMax {
			backoff *= 2
			if backoff > c.backoffMax {
				backoff = c.backoffMax
			}
		}
	}
}

func (c *GoCVCamera) setRunning(v bool) {
	c.mu.Lock()
	c.running = v
	c.mu.Unlock()
}

func (c *GoCVCamera) isRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

func (c *GoCVCamera) Start() error {
	c.setRunning(true)
	return nil
}

// stopWaitTimeout bounds how long Stop waits for the capture loop to leave a
// (possibly hung) device Read before closing the device anyway.
const stopWaitTimeout = 3 * time.Second

// Stop is safe to call more than once. It waits for the capture loop to exit
// before closing the device, because closing a device another goroutine is
// inside Read on is a native use-after-free; the wait is bounded so a wedged
// camera can't hang shutdown.
func (c *GoCVCamera) Stop() {
	c.setRunning(false)
	c.stopOnce.Do(func() {
		if c.stopCh != nil {
			close(c.stopCh)
		}
	})

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(stopWaitTimeout):
		utils.Logf("Camera capture loop did not exit within %v; closing device anyway", stopWaitTimeout)
	}

	c.mu.Lock()
	dev := c.device
	c.device = nil
	c.mu.Unlock()
	if dev != nil && dev.IsOpened() {
		_ = dev.Close()
	}
	if c.cap != nil && c.cap.IsOpened() {
		_ = c.cap.Close()
	}
}

func (c *GoCVCamera) GetFrame() (*Frame, error) {
	if !c.isRunning() {
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
	if !c.isRunning() {
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
	return DisplayName(c.url, c.cameraID)
}

func (c *GoCVCamera) IsConnected() bool {
	if dev := c.currentDevice(); dev != nil {
		return dev.IsOpened()
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
	return c.isRunning()
}

func (c *GoCVCamera) GetRawJPEG() ([]byte, error) {
	if !c.isRunning() {
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
