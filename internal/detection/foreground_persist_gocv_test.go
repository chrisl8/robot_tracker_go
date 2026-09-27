//go:build gocv

package detection

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// waitForFile polls for path to exist, for tests where a save happens on a
// background goroutine (maybePersist) rather than synchronously.
func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s to be written", path)
}

func TestForegroundDetector_SaveNowThenFreshDetectorRestoresAndSkipsWarmUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bg.bin")

	h := newHarness(t, 640, 360)
	h.d.EnablePersistence(path, time.Hour) // long interval: this test drives saving via SaveNow
	h.warm()

	obj := image.Rect(200, 100, 260, 160)
	h.object = &obj
	var res ForegroundResult
	for i := 0; i < 3; i++ {
		res = h.step(ForegroundMasks{})
	}
	if len(res.Blobs) != 1 {
		t.Fatalf("setup: got %d blobs, want 1", len(res.Blobs))
	}
	h.d.SaveNow()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected SaveNow to write %s: %v", path, err)
	}

	// A fresh detector pointed at the same file, with the object still in
	// view, should come up already warm and detect it on its very first
	// frame — no re-learning window during which the object would instead
	// get absorbed into a fresh "empty floor" baseline.
	h2 := newHarness(t, 640, 360)
	h2.d.EnablePersistence(path, time.Hour)
	h2.object = &obj
	res = h2.step(ForegroundMasks{})
	if res.Warming {
		t.Fatal("a detector restored from a saved background must not re-warm")
	}
	if len(res.Blobs) != 1 {
		t.Fatalf("restored detector: got %d blobs on its first frame, want 1", len(res.Blobs))
	}
}

func TestForegroundDetector_SavedBackgroundWithWrongResolutionIsIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bg.bin")
	h := newHarness(t, 640, 360)
	h.d.EnablePersistence(path, time.Hour)
	h.warm()
	h.d.SaveNow()

	// A detector at a different frame resolution (a different camera, or a
	// changed scale config) must not adopt a mismatched save; it should warm
	// up normally instead.
	h2 := newHarness(t, 320, 180)
	h2.d.EnablePersistence(path, time.Hour)
	if res := h2.step(ForegroundMasks{}); !res.Warming {
		t.Error("a resolution mismatch should be ignored, not adopted as if it were a real match")
	}
}

func TestForegroundDetector_ResetDeletesThePersistedBackground(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bg.bin")
	h := newHarness(t, 640, 360)
	h.d.EnablePersistence(path, time.Hour)
	h.warm()
	h.d.SaveNow()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("setup: expected a saved file: %v", err)
	}

	h.d.Reset()
	h.step(ForegroundMasks{}) // applyCommands processes the pending reset
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Reset should delete the persisted background, not just the live state — " +
			"otherwise a restart before the next periodic save would silently undo the reset")
	}
}

func TestForegroundDetector_PersistsInTheBackgroundOnceWarm(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bg.bin")
	h := newHarness(t, 640, 360)
	h.d.EnablePersistence(path, time.Hour) // interval doesn't matter: the first save is unthrottled
	h.warm()                               // the step that finishes warming also triggers the first save
	waitForFile(t, path)
}
