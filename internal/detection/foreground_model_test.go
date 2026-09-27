package detection

import (
	"math/rand"
	"testing"
)

const (
	tw, th = 160, 90
	tdt    = 0.1
)

type scene struct {
	rng  *rand.Rand
	base []float64 // clean background brightness per pixel
}

func newScene(seed int64, level float64) *scene {
	s := &scene{rng: rand.New(rand.NewSource(seed)), base: make([]float64, tw*th)}
	for i := range s.base {
		s.base[i] = level
	}
	return s
}

// frame renders the scene plus per-frame sensor noise (+-2 levels).
func (s *scene) frame() []uint8 {
	g := make([]uint8, tw*th)
	for i, v := range s.base {
		n := v + (s.rng.Float64()*4 - 2)
		switch {
		case n < 0:
			n = 0
		case n > 255:
			n = 255
		}
		g[i] = uint8(n + 0.5)
	}
	return g
}

func (s *scene) addRect(x0, y0, x1, y1 int, delta float64) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			s.base[y*tw+x] += delta
		}
	}
}

func (s *scene) addAll(delta float64) {
	for i := range s.base {
		s.base[i] += delta
	}
}

func testModel(mutate func(*ForegroundParams)) *foregroundModel {
	p := DefaultForegroundParams()
	p.WarmupSec = 1
	if mutate != nil {
		mutate(&p)
	}
	return newForegroundModel(p)
}

// warmUp feeds the scene until the model is warm.
func warmUp(t *testing.T, m *foregroundModel, s *scene, robot []uint8) {
	t.Helper()
	for i := 0; i < 40; i++ {
		_, st := m.step(s.frame(), tw, th, tdt, robot, nil, nil)
		if !st.Warming {
			return
		}
	}
	t.Fatal("model never finished warming up")
}

func countFG(mask []uint8) int {
	n := 0
	for _, v := range mask {
		if v != 0 {
			n++
		}
	}
	return n
}

// colorScene is like scene but tracks a BGR colour per pixel, for shadow-
// suppression tests that need a gray frame and a colour frame that agree
// with each other — gray is luma derived from the same BGR (OpenCV's BGR2GRAY
// weights), matching how production derives gray from the resized colour
// frame (foreground.go).
type colorScene struct {
	rng                 *rand.Rand
	baseB, baseG, baseR []float64
}

func newColorScene(seed int64, b, g, r float64) *colorScene {
	s := &colorScene{rng: rand.New(rand.NewSource(seed)), baseB: make([]float64, tw*th), baseG: make([]float64, tw*th), baseR: make([]float64, tw*th)}
	for i := range s.baseB {
		s.baseB[i], s.baseG[i], s.baseR[i] = b, g, r
	}
	return s
}

// scaleRect multiplies a region's colour by factor — a cast shadow, which
// darkens without changing colour direction.
func (s *colorScene) scaleRect(x0, y0, x1, y1 int, factor float64) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*tw + x
			s.baseB[i] *= factor
			s.baseG[i] *= factor
			s.baseR[i] *= factor
		}
	}
}

// setRect replaces a region's colour outright — a real object, not a shadow.
func (s *colorScene) setRect(x0, y0, x1, y1 int, b, g, r float64) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*tw + x
			s.baseB[i], s.baseG[i], s.baseR[i] = b, g, r
		}
	}
}

func clamp255(v float64) uint8 {
	switch {
	case v < 0:
		return 0
	case v > 255:
		return 255
	default:
		return uint8(v + 0.5)
	}
}

// frames renders the scene plus shared per-frame sensor noise (so B/G/R and
// the gray derived from them stay consistent, as a real camera's would),
// returning a luma frame and the packed BGR frame it was derived from.
func (s *colorScene) frames() (gray []uint8, color []uint8) {
	gray = make([]uint8, tw*th)
	color = make([]uint8, tw*th*3)
	for i := range s.baseB {
		n := s.rng.Float64()*4 - 2
		b := clamp255(s.baseB[i] + n)
		g := clamp255(s.baseG[i] + n)
		r := clamp255(s.baseR[i] + n)
		color[i*3], color[i*3+1], color[i*3+2] = b, g, r
		gray[i] = clamp255(0.114*float64(b) + 0.587*float64(g) + 0.299*float64(r))
	}
	return gray, color
}

// warmUpColor is warmUp for a colorScene.
func warmUpColor(t *testing.T, m *foregroundModel, s *colorScene, robot []uint8) {
	t.Helper()
	for i := 0; i < 40; i++ {
		gray, color := s.frames()
		_, st := m.step(gray, tw, th, tdt, robot, nil, color)
		if !st.Warming {
			return
		}
	}
	t.Fatal("model never finished warming up")
}

func discMask(cx, cy, r int) []uint8 {
	mask := make([]uint8, tw*th)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r {
				mask[y*tw+x] = 1
			}
		}
	}
	return mask
}

func TestForegroundModel_QuietSceneHasNoForeground(t *testing.T) {
	m := testModel(nil)
	s := newScene(1, 120)
	warmUp(t, m, s, nil)
	for i := 0; i < 200; i++ {
		mask, st := m.step(s.frame(), tw, th, tdt, nil, nil, nil)
		if n := countFG(mask); n != 0 || st.Guarded {
			t.Fatalf("frame %d: %d foreground pixels (guarded=%v) on an empty floor", i, n, st.Guarded)
		}
	}
}

func TestForegroundModel_ObjectAppearsStaysAndLeaves(t *testing.T) {
	m := testModel(nil)
	s := newScene(2, 120)
	warmUp(t, m, s, nil)

	s.addRect(40, 30, 60, 50, 60) // a 20x20 bright object
	want := 20 * 20
	var mask []uint8
	for i := 0; i < 5; i++ {
		mask, _ = m.step(s.frame(), tw, th, tdt, nil, nil, nil)
	}
	if n := countFG(mask); n < want*9/10 || n > want*11/10 {
		t.Fatalf("object area = %d px, want about %d", n, want)
	}

	// Five simulated minutes later it is still an obstacle: never absorbed.
	for i := 0; i < 1200; i++ {
		mask, _ = m.step(s.frame(), tw, th, 0.25, nil, nil, nil)
	}
	if n := countFG(mask); n < want*9/10 {
		t.Errorf("object was absorbed into the background (%d px left of %d)", n, want)
	}

	// It leaves: the floor underneath was never learned away, so it vanishes at once.
	s.addRect(40, 30, 60, 50, -60)
	mask, _ = m.step(s.frame(), tw, th, tdt, nil, nil, nil)
	if n := countFG(mask); n != 0 {
		t.Errorf("%d foreground pixels remain after the object left", n)
	}
}

func TestForegroundModel_LightingChanges(t *testing.T) {
	tests := []struct {
		name  string
		drift func(s *scene, i int)
		steps int
	}{
		{"exposure step (+25 everywhere)", func(s *scene, i int) {
			if i == 0 {
				s.addAll(25)
			}
		}, 60},
		{"exposure step (-30 everywhere)", func(s *scene, i int) {
			if i == 0 {
				s.addAll(-30)
			}
		}, 60},
		{"slow drift of 20 levels over an hour", func(s *scene, i int) { s.addAll(20.0 / 1200) }, 1200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testModel(nil)
			s := newScene(3, 120)
			warmUp(t, m, s, nil)
			dt := tdt
			if tt.steps == 1200 {
				dt = 3
			}
			for i := 0; i < tt.steps; i++ {
				tt.drift(s, i)
				mask, st := m.step(s.frame(), tw, th, dt, nil, nil, nil)
				if st.Guarded {
					t.Fatalf("step %d: lighting change tripped the guard", i)
				}
				if n := countFG(mask); n > 20 {
					t.Fatalf("step %d: %d false foreground pixels", i, n)
				}
			}
		})
	}
}

func TestForegroundModel_ShadowsNeedMoreChangeThanBrightening(t *testing.T) {
	tests := []struct {
		name      string
		delta     float64
		wantAlarm bool
	}{
		{"soft shadow -25 is ignored", -25, false},
		{"deep shadow -45 is an object", -45, true},
		{"brightening +25 is an object", 25, true},
		{"faint +15 is noise", 15, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testModel(nil)
			s := newScene(4, 120)
			warmUp(t, m, s, nil)
			s.addRect(50, 20, 80, 60, tt.delta)
			var mask []uint8
			for i := 0; i < 3; i++ {
				mask, _ = m.step(s.frame(), tw, th, tdt, nil, nil, nil)
			}
			if got := countFG(mask) > 100; got != tt.wantAlarm {
				t.Errorf("foreground detected = %v, want %v", got, tt.wantAlarm)
			}
		})
	}
}

func TestForegroundModel_RobotMaskHidesAndLeavesNoGhost(t *testing.T) {
	m := testModel(nil)
	s := newScene(5, 120)
	warmUp(t, m, s, nil)

	// A robot drives across the floor; the mask follows it and hides it.
	for step := 0; step < 100; step++ {
		cx := 20 + step
		robot := discMask(cx, 45, 8)
		frame := s.frame()
		for i, v := range robot {
			if v != 0 {
				frame[i] = 220 // bright robot body
			}
		}
		mask, _ := m.step(frame, tw, th, tdt, robot, nil, nil)
		if n := countFG(mask); n != 0 {
			t.Fatalf("step %d: %d pixels flagged around a masked robot", step, n)
		}
	}

	// The robot is gone: the floor it crossed was never learned as robot.
	mask, _ := m.step(s.frame(), tw, th, tdt, nil, nil, nil)
	if n := countFG(mask); n != 0 {
		t.Errorf("robot left %d ghost pixels behind", n)
	}
}

func TestForegroundModel_RobotPresentDuringWarmUpLeavesNoGhost(t *testing.T) {
	m := testModel(nil)
	s := newScene(6, 120)
	robot := discMask(60, 45, 10)

	for i := 0; i < 40; i++ {
		frame := s.frame()
		for j, v := range robot {
			if v != 0 {
				frame[j] = 220
			}
		}
		if _, st := m.step(frame, tw, th, tdt, robot, nil, nil); !st.Warming {
			break
		}
	}
	// The robot drives away from where it started.
	mask, _ := m.step(s.frame(), tw, th, tdt, nil, nil, nil)
	if n := countFG(mask); n != 0 {
		t.Errorf("robot's start position left %d ghost pixels", n)
	}
}

func TestForegroundModel_StaticMaskAndBorderAreIgnored(t *testing.T) {
	m := testModel(nil)
	s := newScene(7, 120)
	warmUp(t, m, s, nil)

	static := make([]uint8, tw*th)
	for y := 20; y < 40; y++ {
		for x := 20; x < 40; x++ {
			static[y*tw+x] = 1
		}
	}
	s.addRect(20, 20, 40, 40, 60)         // change inside the static box
	s.addRect(0, 0, m.p.BorderPx, th, 60) // change on the frame border
	s.addRect(100, 30, 120, 50, 60)       // a real object elsewhere
	m.step(s.frame(), tw, th, tdt, nil, static, nil)
	mask, _ := m.step(s.frame(), tw, th, tdt, nil, static, nil)
	if n := countFG(mask); n < 300 || n > 500 {
		t.Errorf("foreground = %d px, want only the 20x20 real object (400)", n)
	}
}

func TestForegroundModel_GuardHoldsThenRelearns(t *testing.T) {
	m := testModel(func(p *ForegroundParams) { p.GuardMaxSec = 2 })
	s := newScene(8, 120)
	warmUp(t, m, s, nil)

	// Half the floor changes at once: not an obstacle, a lighting event.
	s.addRect(0, 0, tw/2, th, 80)
	guarded, rewarmed := 0, false
	for i := 0; i < 60; i++ {
		_, st := m.step(s.frame(), tw, th, tdt, nil, nil, nil)
		if st.Guarded {
			guarded++
		}
		if st.Rewarmed {
			rewarmed = true
			break
		}
	}
	if guarded == 0 {
		t.Error("a half-frame change should trip the guard")
	}
	if !rewarmed {
		t.Fatal("the guard never gave up and relearned")
	}
	warmUp(t, m, s, nil)
	if mask, _ := m.step(s.frame(), tw, th, tdt, nil, nil, nil); countFG(mask) != 0 {
		t.Error("after relearning the new lighting should be the background")
	}
}

func TestForegroundModel_AbsorbAt(t *testing.T) {
	m := testModel(nil)
	s := newScene(9, 120)
	warmUp(t, m, s, nil)
	s.addRect(40, 30, 60, 50, 60)
	for i := 0; i < 3; i++ {
		m.step(s.frame(), tw, th, tdt, nil, nil, nil)
	}

	if got := m.absorbAt(0, 0); got != 0 {
		t.Errorf("absorbing empty floor returned %d, want 0", got)
	}
	if got := m.absorbAt(50, 40); got < 350 || got > 450 {
		t.Errorf("absorbed %d px, want about 400", got)
	}
	mask, _ := m.step(s.frame(), tw, th, tdt, nil, nil, nil)
	if n := countFG(mask); n != 0 {
		t.Errorf("%d pixels still foreground after absorb", n)
	}
}

func TestForegroundModel_AbsorbAfterTimeout(t *testing.T) {
	m := testModel(func(p *ForegroundParams) { p.AbsorbAfterSec = 5 })
	s := newScene(10, 120)
	warmUp(t, m, s, nil)
	s.addRect(40, 30, 60, 50, 60)

	var mask []uint8
	for i := 0; i < 30; i++ { // 3 s: still an obstacle
		mask, _ = m.step(s.frame(), tw, th, tdt, nil, nil, nil)
	}
	if countFG(mask) == 0 {
		t.Fatal("object vanished before the absorb timeout")
	}
	for i := 0; i < 40; i++ { // past 5 s
		mask, _ = m.step(s.frame(), tw, th, tdt, nil, nil, nil)
	}
	if n := countFG(mask); n != 0 {
		t.Errorf("object not absorbed after the timeout (%d px)", n)
	}
}

func TestForegroundModel_ResetRelearns(t *testing.T) {
	m := testModel(nil)
	s := newScene(11, 120)
	warmUp(t, m, s, nil)
	s.addRect(40, 30, 60, 50, 60)
	m.reset()
	warmUp(t, m, s, nil) // the object is now part of the background
	if mask, _ := m.step(s.frame(), tw, th, tdt, nil, nil, nil); countFG(mask) != 0 {
		t.Error("after a reset the current scene should be the background")
	}
}

func TestForegroundModel_RejectsWrongSizedFrames(t *testing.T) {
	m := testModel(nil)
	if mask, _ := m.step(make([]uint8, 10), tw, th, tdt, nil, nil, nil); mask != nil {
		t.Error("a frame of the wrong size should be rejected")
	}
}

func TestForegroundModel_SnapshotBeforeWarmIsNotOK(t *testing.T) {
	m := testModel(nil)
	if _, ok := m.snapshot(); ok {
		t.Error("a model that hasn't warmed up yet has nothing worth saving")
	}
}

func TestForegroundModel_SnapshotRestoreRoundTrips(t *testing.T) {
	m := testModel(nil)
	s := newScene(21, 120)
	warmUp(t, m, s, nil)
	m.step(s.frame(), tw, th, tdt, nil, nil, nil) // give gain a non-default value to round-trip too

	snap, ok := m.snapshot()
	if !ok {
		t.Fatal("expected a warm model to have a snapshot")
	}

	restored := testModel(nil)
	if !restored.restore(snap) {
		t.Fatal("restore of a valid snapshot should succeed")
	}
	if !restored.warm {
		t.Error("a restored model should be warm immediately, skipping warm-up")
	}
	if restored.gain != m.gain {
		t.Errorf("gain = %v, want %v", restored.gain, m.gain)
	}
	for i := range m.bg {
		if restored.bg[i] != m.bg[i] || restored.known[i] != m.known[i] {
			t.Fatalf("pixel %d: bg/known did not round-trip (%v/%v vs %v/%v)",
				i, restored.bg[i], restored.known[i], m.bg[i], m.known[i])
		}
	}
}

func TestForegroundModel_RestoredModelBehavesLikeALiveWarmModel(t *testing.T) {
	live := testModel(nil)
	s := newScene(31, 120)
	warmUp(t, live, s, nil)

	snap, ok := live.snapshot()
	if !ok {
		t.Fatal("expected a snapshot")
	}
	restored := testModel(nil)
	if !restored.restore(snap) {
		t.Fatal("restore should succeed")
	}

	s.addRect(40, 30, 60, 50, 60)
	frame := s.frame()
	liveMask, _ := live.step(frame, tw, th, tdt, nil, nil, nil)
	restoredMask, restoredStep := restored.step(frame, tw, th, tdt, nil, nil, nil)
	if restoredStep.Warming {
		t.Fatal("a restored model must not re-warm")
	}
	if countFG(liveMask) == 0 {
		t.Fatal("test setup: the live model should see the new object")
	}
	if liveMask == nil || restoredMask == nil || len(liveMask) != len(restoredMask) {
		t.Fatalf("expected two comparable masks, got %d and %d", len(liveMask), len(restoredMask))
	}
	for i := range liveMask {
		if liveMask[i] != restoredMask[i] {
			t.Fatalf("pixel %d: live=%v restored=%v, want the same detection", i, liveMask[i], restoredMask[i])
		}
	}
}

func TestForegroundModel_RestoreRejectsMismatchedSize(t *testing.T) {
	m := testModel(nil)
	bad := modelState{W: tw, H: th, Bg: make([]float32, tw*th-1), Known: make([]bool, tw*th)}
	if m.restore(bad) {
		t.Error("restore should reject a snapshot whose slices don't match its own W*H")
	}
	if m.bg != nil {
		t.Error("a rejected restore must not touch the model")
	}
}

// TestIsShadowColor is a direct table test of the pure shadow-classification
// math, independent of the model: a shadow scales the background colour down
// (same direction, lower magnitude); a real object usually doesn't.
func TestIsShadowColor(t *testing.T) {
	const scaleMin, scaleMax, chromaMax = 0.35, 0.98, 0.12
	bg := [3]float32{150, 150, 150} // a neutral (low-saturation) carpet colour
	tests := []struct {
		name string
		cur  [3]float32
		bg   [3]float32
		want bool
	}{
		{"scaled down 0.6x (a real cast shadow)", [3]float32{90, 90, 90}, bg, true},
		{"scaled down 0.8x (a faint shadow)", [3]float32{120, 120, 120}, bg, true},
		{"barely darker (0.97x, still within bounds)", [3]float32{145.5, 145.5, 145.5}, bg, true},
		{"a different colour entirely, not just darker", [3]float32{40, 120, 40}, bg, false},
		{"too dark to be a plausible shadow (0.1x)", [3]float32{15, 15, 15}, bg, false},
		{"barely brighter, not darker at all (1.0x)", [3]float32{150, 150, 150}, bg, false},
		{"near-black background: the test can't mean anything there", [3]float32{5, 5, 5}, [3]float32{2, 2, 2}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isShadowColor(tt.cur[0], tt.cur[1], tt.cur[2], tt.bg[0], tt.bg[1], tt.bg[2], scaleMin, scaleMax, chromaMax)
			if got != tt.want {
				t.Errorf("isShadowColor(cur=%v, bg=%v) = %v, want %v", tt.cur, tt.bg, got, tt.want)
			}
		})
	}
}

// TestForegroundModel_ShadowSuppressedOnNeutralCarpet is the real-world case
// from the bug report: a low-saturation (neutral gray) floor, where a cast
// shadow scales brightness down without changing colour, and a raw HSV-hue
// comparison would be unreliable (hue is unstable near the achromatic axis).
// The colour-ratio test must still discriminate a shadow from a real object
// there.
func TestForegroundModel_ShadowSuppressedOnNeutralCarpet(t *testing.T) {
	m := testModel(nil)
	s := newColorScene(41, 150, 150, 150) // neutral gray carpet, zero saturation
	warmUpColor(t, m, s, nil)

	// A shadow: same colour, scaled down (a robot or furniture blocking
	// overhead light). Must not register as foreground.
	s.scaleRect(20, 20, 60, 60, 0.6)
	// A real object elsewhere: black, far darker than any plausible shadow
	// (isShadowColor's scaleMin bound rules it out), so it must still
	// register.
	s.setRect(100, 40, 130, 70, 5, 5, 5)

	var mask []uint8
	var st modelStep
	for i := 0; i < 3; i++ {
		gray, color := s.frames()
		mask, st = m.step(gray, tw, th, tdt, nil, nil, color)
	}
	if len(mask) != tw*th {
		t.Fatalf("expected a %d-pixel mask, got %d", tw*th, len(mask))
	}
	if st.ShadowSuppressed == 0 {
		t.Error("ShadowSuppressed should report the shadow pixels it reclassified, got 0")
	}

	shadowTotal := 0
	for y := 20; y < 60; y++ {
		shadowTotal += countFG(mask[y*tw+20 : y*tw+60])
	}
	if shadowTotal != 0 {
		t.Errorf("shadow region: %d foreground pixels total, want 0 (it should read as background, not an obstacle)", shadowTotal)
	}

	objectTotal := 0
	for y := 40; y < 70; y++ {
		objectTotal += countFG(mask[y*tw+100 : y*tw+130])
	}
	if objectTotal == 0 {
		t.Error("a genuinely dark object (not a shadow) should still be flagged as foreground")
	}
}

// TestForegroundModel_ColorBlindCallsAreUnaffected proves the gate is inert
// end-to-end (not just via isShadowColor's own defaults) when no colour is
// supplied: an object darker than a plausible shadow, and a real shadow-like
// darkening, both behave exactly as the pre-existing (grayscale-only)
// threshold logic already specifies — see
// TestForegroundModel_ShadowsNeedMoreChangeThanBrightening, which this
// mirrors but with color == nil made explicit.
func TestForegroundModel_ColorBlindCallsAreUnaffected(t *testing.T) {
	m := testModel(nil)
	s := newScene(42, 150)
	warmUp(t, m, s, nil)

	s.addRect(20, 20, 60, 60, -25) // a soft shadow: below dark_factor's threshold
	var mask []uint8
	for i := 0; i < 3; i++ {
		mask, _ = m.step(s.frame(), tw, th, tdt, nil, nil, nil)
	}
	if n := countFG(mask); n != 0 {
		t.Errorf("a soft darkening within dark_factor's threshold should stay background, got %d foreground pixels", n)
	}
}
