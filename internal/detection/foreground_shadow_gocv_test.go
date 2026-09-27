//go:build gocv

package detection

import (
	"image"
	"math/rand"
	"testing"
	"time"
)

// TestForegroundDetector_SuppressesARealCastShadow is the end-to-end proof,
// through the real OpenCV pipeline (not just the pure model tested in
// foreground_model_test.go), that a cast shadow on a neutral (low-saturation)
// floor is no longer flagged as an obstacle, while a genuinely dark object
// still is — the real-world case that motivated colour-based shadow
// suppression (a robot's shadow on carpet, reported live).
func TestForegroundDetector_SuppressesARealCastShadow(t *testing.T) {
	const w, h = 640, 360
	const floorLevel = 150.0 // neutral gray, zero saturation, like carpet
	shadowRect := image.Rect(100, 100, 200, 200)
	objectRect := image.Rect(400, 150, 480, 230)

	rng := rand.New(rand.NewSource(7))
	d := NewForegroundDetector(fgTestParams())
	t.Cleanup(d.Close)

	frame := func(shadow, object bool) []byte {
		f := bgrFrame(rng, w, h, floorLevel)
		if shadow {
			// A cast shadow: the same neutral colour, scaled down — no
			// change in colour direction, only brightness.
			paintRect(f, w, shadowRect, byte(floorLevel*0.6))
		}
		if object {
			// A real object: much darker than any plausible shadow.
			paintRect(f, w, objectRect, 10)
		}
		return f
	}

	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	step := func(shadow, object bool) ForegroundResult {
		now = now.Add(70 * time.Millisecond)
		return d.Process(frame(shadow, object), w, h, now, ForegroundMasks{})
	}

	for i := 0; i < 60; i++ {
		if !step(false, false).Warming {
			break
		}
	}

	var res ForegroundResult
	for i := 0; i < 5; i++ {
		res = step(true, true)
	}

	shadowFlagged, objectFlagged := false, false
	for _, b := range res.Blobs {
		if b.AABB.Overlaps(shadowRect) {
			shadowFlagged = true
		}
		if b.AABB.Overlaps(objectRect) {
			objectFlagged = true
		}
	}
	if shadowFlagged {
		t.Errorf("the shadow region was flagged as an obstacle: %v", res.Blobs)
	}
	if !objectFlagged {
		t.Errorf("the real dark object was not flagged as an obstacle: %v", res.Blobs)
	}
}
