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

// GoCVCamera is the only Camera implementation.
var _ Camera = (*GoCVCamera)(nil)

type GoCVCamera struct {
	device   frameSource // guarded by mu once the capture loop runs
	open     func() (frameSource, error)
	running  bool // guarded by mu
	cameraID int
	url      string

	reopenAfter int
	retryDelay  time.Duration
	backoffMin  time.Duration
	backoffMax  time.Duration

	mu          sync.Mutex
	latestFrame *Frame
	frameSeq    uint64 // sequence number of latestFrame; guarded by mu
	frameErr    error
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
}

// newGoCVCamera wraps an opened device, remembering how to reopen it, and
// starts the capture loop.
func newGoCVCamera(dev frameSource, open func() (frameSource, error), cameraID int, url string) *GoCVCamera {
	cam := &GoCVCamera{
		device:   dev,
		open:     open,
		running:  true,
		cameraID: cameraID,
		url:      url,
		stopCh:   make(chan struct{}),
	}
	cam.setRecoveryDefaults()
	cam.wg.Add(1)
	go cam.captureLoop()
	return cam
}

// NewGoCVCamera opens a local (USB/built-in) camera by device index.
func NewGoCVCamera(config CameraConfig) (*GoCVCamera, error) {
	width, height, fps := config.Width, config.Height, config.FPS
	if width == 0 {
		width = DefaultWidth
	}
	if height == 0 {
		height = DefaultHeight
	}
	if fps == 0 {
		fps = DefaultFPS
	}

	openDevice := func() (frameSource, error) {
		c, err := openUSBCapture(config.CameraID, width, height, fps)
		if err != nil {
			return nil, err
		}
		return c, nil
	}
	dev, err := openDevice()
	if err != nil {
		return nil, err
	}
	return newGoCVCamera(dev, openDevice, config.CameraID, ""), nil
}

// NewGoCVIPCamera opens a network stream or video file by URL.
func NewGoCVIPCamera(url string) (*GoCVCamera, error) {
	openStream := func() (frameSource, error) {
		c, err := openStreamCapture(url)
		if err != nil {
			return nil, err
		}
		return c, nil
	}
	dev, err := openStream()
	if err != nil {
		return nil, err
	}
	return newGoCVCamera(dev, openStream, 0, url), nil
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
		c.frameSeq++
		frame.Seq = c.frameSeq
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
}

// GetFrame returns the most recent frame the capture loop produced. It does
// not block, so it can return the same frame again if no new one has arrived:
// callers must compare Frame.Seq and only process a frame they have not seen,
// or a camera that has hung inside a read would look like a healthy stream.
func (c *GoCVCamera) GetFrame() (*Frame, error) {
	if !c.isRunning() {
		return nil, &CameraError{Message: "camera not running"}
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

func (c *GoCVCamera) GetName() string {
	return DisplayName(c.url, c.cameraID)
}
