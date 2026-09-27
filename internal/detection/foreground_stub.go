//go:build !gocv

package detection

import "time"

// ForegroundDetector is unavailable without OpenCV; it detects nothing.
type ForegroundDetector struct{}

func NewForegroundDetector(ForegroundParams) *ForegroundDetector { return &ForegroundDetector{} }

func (d *ForegroundDetector) IsAvailable() bool { return false }

// EnablePersistence is a no-op: there is no background to persist without
// OpenCV.
func (d *ForegroundDetector) EnablePersistence(string, time.Duration) {}

// SaveNow is a no-op without OpenCV.
func (d *ForegroundDetector) SaveNow() {}

func (d *ForegroundDetector) Reset() {}

func (d *ForegroundDetector) Absorb(int, int) {}

func (d *ForegroundDetector) RequestDebug() {}

func (d *ForegroundDetector) DebugJPEG() []byte { return nil }

func (d *ForegroundDetector) Close() {}

func (d *ForegroundDetector) Process([]byte, int, int, time.Time, ForegroundMasks) ForegroundResult {
	return ForegroundResult{}
}
