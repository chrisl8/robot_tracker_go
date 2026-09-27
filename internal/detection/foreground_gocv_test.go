//go:build gocv

package detection

import (
	"image"
	"math/rand"
	"testing"
	"time"
)

// bgrFrame builds a w x h BGR frame of carpet-like texture (level +- noise).
func bgrFrame(rng *rand.Rand, w, h int, level float64) []byte {
	f := make([]byte, w*h*3)
	for i := 0; i < w*h; i++ {
		v := byte(level + rng.Float64()*8 - 4)
		f[i*3], f[i*3+1], f[i*3+2] = v, v, v
	}
	return f
}

func paintRect(f []byte, w int, r image.Rectangle, level byte) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := (y*w + x) * 3
			f[i], f[i+1], f[i+2] = level, level, level
		}
	}
}

func fgTestParams() ForegroundParams {
	p := DefaultForegroundParams()
	p.WarmupSec = 1
	return p
}

type fgHarness struct {
	t      *testing.T
	d      *ForegroundDetector
	rng    *rand.Rand
	w, h   int
	now    time.Time
	object *image.Rectangle
}

func newHarness(t *testing.T, w, h int) *fgHarness {
	t.Helper()
	d := NewForegroundDetector(fgTestParams())
	t.Cleanup(d.Close)
	return &fgHarness{t: t, d: d, rng: rand.New(rand.NewSource(1)), w: w, h: h, now: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
}

func (h *fgHarness) frame() []byte {
	f := bgrFrame(h.rng, h.w, h.h, 120)
	if h.object != nil {
		paintRect(f, h.w, *h.object, 200)
	}
	return f
}

func (h *fgHarness) step(masks ForegroundMasks) ForegroundResult {
	h.now = h.now.Add(70 * time.Millisecond)
	return h.d.Process(h.frame(), h.w, h.h, h.now, masks)
}

func (h *fgHarness) warm() {
	h.t.Helper()
	for i := 0; i < 60; i++ {
		if !h.step(ForegroundMasks{}).Warming {
			return
		}
	}
	h.t.Fatal("never finished warming up")
}

func TestForegroundDetector_ReportsAPlacedObjectAtFullResolutionCoordinates(t *testing.T) {
	h := newHarness(t, 640, 360)
	if res := h.step(ForegroundMasks{}); !res.Warming || len(res.Blobs) != 0 {
		t.Fatal("first frame should be warming up with no blobs")
	}
	h.warm()

	for i := 0; i < 5; i++ {
		if res := h.step(ForegroundMasks{}); len(res.Blobs) != 0 {
			t.Fatalf("empty floor reported %d blobs", len(res.Blobs))
		}
	}

	obj := image.Rect(200, 100, 260, 160)
	h.object = &obj
	var res ForegroundResult
	for i := 0; i < 3; i++ {
		res = h.step(ForegroundMasks{})
	}
	if len(res.Blobs) != 1 {
		t.Fatalf("got %d blobs, want 1 (%v)", len(res.Blobs), res.Blobs)
	}
	got := res.Blobs[0]
	if !got.Overlaps(obj) || got.Dx() < 55 || got.Dx() > 75 || got.Dy() < 55 || got.Dy() > 75 {
		t.Errorf("blob %v does not match the object %v", got, obj)
	}

	h.object = nil
	if res = h.step(ForegroundMasks{}); len(res.Blobs) != 0 {
		t.Errorf("object removed but %d blobs remain", len(res.Blobs))
	}
}

func TestForegroundDetector_MasksAndSuspend(t *testing.T) {
	h := newHarness(t, 640, 360)
	h.warm()

	obj := image.Rect(300, 120, 360, 180)
	h.object = &obj
	robot := ForegroundMasks{Robots: []Disc{{X: 330, Y: 150, R: 60}}}
	if res := h.step(robot); len(res.Blobs) != 0 {
		t.Errorf("an object under a robot mask produced %d blobs", len(res.Blobs))
	}
	static := ForegroundMasks{Statics: []image.Rectangle{image.Rect(290, 110, 370, 190)}}
	if res := h.step(static); len(res.Blobs) != 0 {
		t.Errorf("an object inside a static obstacle produced %d blobs", len(res.Blobs))
	}
	if res := h.step(ForegroundMasks{}); len(res.Blobs) != 1 {
		t.Errorf("unmasked, want 1 blob, got %d", len(res.Blobs))
	}

	if res := h.step(ForegroundMasks{Suspended: true}); len(res.Blobs) != 0 || res.Warming {
		t.Error("a suspended detector should report nothing")
	}
}

func TestForegroundDetector_ResetAndAbsorb(t *testing.T) {
	h := newHarness(t, 640, 360)
	h.warm()
	obj := image.Rect(100, 100, 160, 160)
	h.object = &obj
	for i := 0; i < 3; i++ {
		h.step(ForegroundMasks{})
	}

	h.d.Absorb(130, 130)
	if res := h.step(ForegroundMasks{}); len(res.Blobs) != 0 {
		t.Errorf("absorbed object still reported: %v", res.Blobs)
	}

	obj2 := image.Rect(400, 200, 460, 260)
	h.object = &obj2
	h.step(ForegroundMasks{})
	h.d.Reset()
	if res := h.step(ForegroundMasks{}); !res.Warming {
		t.Error("Reset should restart warm-up")
	}
	h.warm()
	if res := h.step(ForegroundMasks{}); len(res.Blobs) != 0 {
		t.Error("after reset the scene (with the object) is the new background")
	}
}

func TestForegroundDetector_DebugImage(t *testing.T) {
	h := newHarness(t, 640, 360)
	h.warm()
	if h.d.DebugJPEG() != nil {
		t.Fatal("no debug image should be rendered until requested")
	}
	h.d.RequestDebug()
	h.step(ForegroundMasks{})
	if jpeg := h.d.DebugJPEG(); len(jpeg) < 100 || jpeg[0] != 0xFF || jpeg[1] != 0xD8 {
		t.Errorf("debug image is not a JPEG (%d bytes)", len(jpeg))
	}
}

func TestForegroundDetector_HandlesResolutionChangeAndBadFrames(t *testing.T) {
	h := newHarness(t, 640, 360)
	h.warm()
	if res := h.d.Process(make([]byte, 10), 640, 360, h.now, ForegroundMasks{}); len(res.Blobs) != 0 {
		t.Error("a malformed frame should report nothing")
	}
	h.w, h.h = 320, 180
	if res := h.step(ForegroundMasks{}); !res.Warming {
		t.Error("a new resolution should start a fresh background")
	}
}

// TestForegroundDetector_Timing reports the per-frame cost at real camera size.
// It only fails if wildly slow (well beyond the ~5 ms budget) so it is not flaky.
func TestForegroundDetector_Timing(t *testing.T) {
	h := newHarness(t, 1280, 720)
	h.warm()
	obj := image.Rect(600, 300, 700, 400)
	h.object = &obj
	frames := make([][]byte, 5)
	for i := range frames {
		frames[i] = h.frame()
	}
	const n = 60
	start := time.Now()
	for i := 0; i < n; i++ {
		h.now = h.now.Add(70 * time.Millisecond)
		h.d.Process(frames[i%len(frames)], h.w, h.h, h.now, ForegroundMasks{Robots: []Disc{{X: 900, Y: 500, R: 60}}})
	}
	per := time.Since(start) / n
	t.Logf("foreground detection at 1280x720: %.2f ms/frame", float64(per.Microseconds())/1000)
	if per > 60*time.Millisecond {
		t.Errorf("%v per frame is far beyond the budget", per)
	}
}
