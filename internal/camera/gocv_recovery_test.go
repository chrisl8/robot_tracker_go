//go:build gocv

package camera

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gocv.io/x/gocv"
)

// fakeSource delivers a small frame while healthy and fails every read once
// broken, like a USB camera whose stream has stalled.
type fakeSource struct {
	broken atomic.Bool
	closed atomic.Bool
}

func (f *fakeSource) Read(m *gocv.Mat) bool {
	if f.broken.Load() || f.closed.Load() {
		return false
	}
	src := gocv.NewMatWithSize(4, 4, gocv.MatTypeCV8UC3)
	defer func() { _ = src.Close() }()
	_ = src.CopyTo(m)
	return true
}

func (f *fakeSource) Close() error   { f.closed.Store(true); return nil }
func (f *fakeSource) IsOpened() bool { return !f.closed.Load() }

func newTestCamera(dev frameSource, reopenAfter int, open func() (frameSource, error)) *GoCVCamera {
	c := &GoCVCamera{
		device:      dev,
		open:        open,
		running:     true,
		stopCh:      make(chan struct{}),
		reopenAfter: reopenAfter,
		retryDelay:  time.Millisecond,
		backoffMin:  time.Millisecond,
		backoffMax:  4 * time.Millisecond,
	}
	c.wg.Add(1)
	go c.captureLoop()
	return c
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func gotFrame(c *GoCVCamera) bool {
	f, err := c.GetFrame()
	return err == nil && f != nil && f.Width == 4
}

// TestCaptureLoop_ReopensAStalledDevice is the regression for a camera that
// stopped delivering frames and never came back: the capture loop used to exit
// on the first failed read, so video stayed dead until the process restarted.
func TestCaptureLoop_ReopensAStalledDevice(t *testing.T) {
	first := &fakeSource{}
	second := &fakeSource{}
	var opens atomic.Int32
	cam := newTestCamera(first, 5, func() (frameSource, error) {
		opens.Add(1)
		return second, nil
	})
	defer cam.Stop()

	waitFor(t, "first frame", func() bool { return gotFrame(cam) })

	first.broken.Store(true)
	waitFor(t, "frames from the reopened device", func() bool { return opens.Load() == 1 && gotFrame(cam) })

	if !first.closed.Load() {
		t.Error("the stalled device should have been closed before reopening")
	}
	if opens.Load() != 1 {
		t.Errorf("device reopened %d times, want exactly 1", opens.Load())
	}
}

func TestCaptureLoop_RetriesReopenUntilItSucceeds(t *testing.T) {
	first := &fakeSource{}
	healthy := &fakeSource{}
	var attempts atomic.Int32
	cam := newTestCamera(first, 5, func() (frameSource, error) {
		if attempts.Add(1) < 4 {
			return nil, errors.New("camera busy")
		}
		return healthy, nil
	})
	defer cam.Stop()

	waitFor(t, "first frame", func() bool { return gotFrame(cam) })
	first.broken.Store(true)
	waitFor(t, "recovery after failed reopens", func() bool { return attempts.Load() >= 4 && gotFrame(cam) })
}

func TestCaptureLoop_TransientFailureDoesNotReopen(t *testing.T) {
	dev := &fakeSource{}
	var opens atomic.Int32
	cam := newTestCamera(dev, 1000, func() (frameSource, error) {
		opens.Add(1)
		return &fakeSource{}, nil
	})
	defer cam.Stop()

	waitFor(t, "first frame", func() bool { return gotFrame(cam) })

	// A couple of bad reads (fewer than reopenAfter) followed by recovery.
	dev.broken.Store(true)
	time.Sleep(2 * time.Millisecond)
	dev.broken.Store(false)
	waitFor(t, "frames after a blip", func() bool { return gotFrame(cam) })

	if opens.Load() != 0 {
		t.Errorf("a short blip reopened the device %d times, want 0", opens.Load())
	}
}

func TestCaptureLoop_StopEndsARecoveryInProgress(t *testing.T) {
	dev := &fakeSource{}
	var wg sync.WaitGroup
	cam := newTestCamera(dev, 5, func() (frameSource, error) { return nil, errors.New("never available") })

	waitFor(t, "first frame", func() bool { return gotFrame(cam) })
	dev.broken.Store(true)
	time.Sleep(30 * time.Millisecond) // let it enter the reopen loop

	wg.Add(1)
	go func() { defer wg.Done(); cam.Stop() }()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not return while the camera was trying to reopen")
	}
}

func TestStop_IsIdempotent(t *testing.T) {
	cam := newTestCamera(&fakeSource{}, 5, nil)
	waitFor(t, "first frame", func() bool { return gotFrame(cam) })

	cam.Stop()
	cam.Stop() // used to panic: close of closed channel
}

// blockingSource's Read blocks until released and records whether the device
// was closed while a Read was still in flight (a native use-after-free).
type blockingSource struct {
	inRead          atomic.Bool
	closedWhileRead atomic.Bool
	entered         chan struct{}
	release         chan struct{}
	closed          atomic.Bool
}

func (b *blockingSource) Read(m *gocv.Mat) bool {
	b.inRead.Store(true)
	defer b.inRead.Store(false)
	select {
	case b.entered <- struct{}{}:
	default:
	}
	<-b.release
	return false
}

func (b *blockingSource) Close() error {
	if b.inRead.Load() {
		b.closedWhileRead.Store(true)
	}
	b.closed.Store(true)
	return nil
}

func (b *blockingSource) IsOpened() bool { return !b.closed.Load() }

func TestStop_WaitsForInFlightReadBeforeClosingDevice(t *testing.T) {
	src := &blockingSource{entered: make(chan struct{}, 1), release: make(chan struct{})}
	cam := newTestCamera(src, 1000, nil)
	<-src.entered // capture loop is now blocked inside Read

	stopped := make(chan struct{})
	go func() { cam.Stop(); close(stopped) }()

	select {
	case <-stopped:
		t.Fatal("Stop returned while a Read was still in flight")
	case <-time.After(100 * time.Millisecond):
	}

	close(src.release) // let the Read finish
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return after the Read finished")
	}
	if src.closedWhileRead.Load() {
		t.Error("device was closed while Read was in flight")
	}
	if !src.closed.Load() {
		t.Error("device was never closed")
	}
}
