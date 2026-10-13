//go:build !gocv

package detection

import "time"

// ForegroundDetector is unavailable without OpenCV; it detects nothing.
type ForegroundDetector struct{}

// NewForegroundDetector returns a detector that never reports anything.
func NewForegroundDetector(ForegroundParams) *ForegroundDetector { return &ForegroundDetector{} }

// EnablePersistence is a no-op: there is no background to persist without
// OpenCV.
func (d *ForegroundDetector) EnablePersistence(string, time.Duration) {}

// SaveNow is a no-op without OpenCV.
func (d *ForegroundDetector) SaveNow() {}

// Reset does nothing.
func (d *ForegroundDetector) Reset() {}

// Absorb does nothing.
func (d *ForegroundDetector) Absorb(int, int) {}

// RequestDebug does nothing.
func (d *ForegroundDetector) RequestDebug() {}

// DebugJPEG returns nil: there is no debug image.
func (d *ForegroundDetector) DebugJPEG() []byte { return nil }

// Close does nothing.
func (d *ForegroundDetector) Close() {}

// Process returns an empty result.
func (d *ForegroundDetector) Process([]byte, int, int, time.Time, ForegroundMasks) ForegroundResult {
	return ForegroundResult{}
}
