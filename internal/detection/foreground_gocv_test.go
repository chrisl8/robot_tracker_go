//go:build gocv

package detection

import (
	"image"
	"math"
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
	got := res.Blobs[0].AABB
	if !got.Overlaps(obj) || got.Dx() < 55 || got.Dx() > 75 || got.Dy() < 55 || got.Dy() > 75 {
		t.Errorf("blob %v does not match the object %v", got, obj)
	}
	corners := res.Blobs[0].Corners
	if corners == ([4]image.Point{}) {
		t.Error("blob Corners should always be populated, even as an AABB fallback")
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

// TestForegroundDetector_Timing reports the per-frame cost at real camera size
// (about 1.5 ms without the race detector). It only fails if absurdly slow: the
// race detector and a busy machine can make it 50x slower, so a tight bound
// would be flaky.
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
	if per > 2*time.Second {
		t.Errorf("%v per frame is far beyond the budget", per)
	}
}

// paintRotatedRect paints a rectangle of half-length l and half-width w,
// centred at (cx, cy) and rotated by angleDeg degrees, into a BGR frame.
func paintRotatedRect(f []byte, frameW, frameH int, cx, cy, l, w, angleDeg float64, level byte) {
	rad := angleDeg * math.Pi / 180
	c, s := math.Cos(rad), math.Sin(rad)
	for y := 0; y < frameH; y++ {
		for x := 0; x < frameW; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			// Rotate the point into the rectangle's own frame.
			lx := dx*c + dy*s
			ly := -dx*s + dy*c
			if math.Abs(lx) <= l && math.Abs(ly) <= w {
				i := (y*frameW + x) * 3
				f[i], f[i+1], f[i+2] = level, level, level
			}
		}
	}
}

// quadArea returns the area of a quadrilateral given in order (shoelace).
func quadArea(pts [4]image.Point) float64 {
	sum := 0.0
	for i := 0; i < 4; i++ {
		a, b := pts[i], pts[(i+1)%4]
		sum += float64(a.X*b.Y - b.X*a.Y)
	}
	return math.Abs(sum) / 2
}

// TestForegroundDetector_ExtractsATightOrientedBoxForARotatedObject is the
// point of phase 5: the old bounding-box-only extraction inflates a diagonal
// elongated object hugely (up to ~6x its true area at 45 degrees, matching
// what was observed live). The oriented box (via gocv.MinAreaRect on the
// blob's own pixels) should stay close to the object's true area regardless
// of angle, while the AABB blows up exactly as expected.
func TestForegroundDetector_ExtractsATightOrientedBoxForARotatedObject(t *testing.T) {
	h := newHarness(t, 640, 360)
	h.warm()

	const halfLen, halfWidth = 80.0, 8.0 // a 160x16 px stick: 10:1 aspect
	trueArea := (2 * halfLen) * (2 * halfWidth)

	tests := []struct {
		name       string
		angleDeg   float64
		maxOBBArea float64 // generous slack for blur/morphology rounding
	}{
		{"axis aligned", 0, trueArea * 1.6},
		{"45 degrees (worst case for an AABB)", 45, trueArea * 1.6},
		{"30 degrees", 30, trueArea * 1.6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.d.Reset()
			for i := 0; i < 60; i++ {
				if !h.step(ForegroundMasks{}).Warming {
					break
				}
			}
			cx, cy := 320.0, 180.0
			frame := bgrFrame(h.rng, h.w, h.h, 120)
			paintRotatedRect(frame, h.w, h.h, cx, cy, halfLen, halfWidth, tt.angleDeg, 200)
			var res ForegroundResult
			for i := 0; i < 3; i++ {
				h.now = h.now.Add(70 * time.Millisecond)
				res = h.d.Process(frame, h.w, h.h, h.now, ForegroundMasks{})
			}
			if len(res.Blobs) != 1 {
				t.Fatalf("got %d blobs, want 1", len(res.Blobs))
			}
			blob := res.Blobs[0]

			aabbArea := float64(blob.AABB.Dx() * blob.AABB.Dy())
			obbArea := quadArea(blob.Corners)
			t.Logf("angle=%.0f true=%.0f aabb=%.0f (%.2fx) obb=%.0f (%.2fx)",
				tt.angleDeg, trueArea, aabbArea, aabbArea/trueArea, obbArea, obbArea/trueArea)

			if obbArea > tt.maxOBBArea {
				t.Errorf("oriented box area %.0f px, want <= %.0f (%.2fx true area)",
					obbArea, tt.maxOBBArea, tt.maxOBBArea/trueArea)
			}
			if tt.angleDeg == 45 && obbArea >= aabbArea {
				t.Errorf("at 45 degrees the oriented box (%.0f) should be much tighter than the AABB (%.0f)",
					obbArea, aabbArea)
			}
		})
	}
}
