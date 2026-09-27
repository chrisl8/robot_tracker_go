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
		_, st := m.step(s.frame(), tw, th, tdt, robot, nil)
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
		mask, st := m.step(s.frame(), tw, th, tdt, nil, nil)
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
		mask, _ = m.step(s.frame(), tw, th, tdt, nil, nil)
	}
	if n := countFG(mask); n < want*9/10 || n > want*11/10 {
		t.Fatalf("object area = %d px, want about %d", n, want)
	}

	// Five simulated minutes later it is still an obstacle: never absorbed.
	for i := 0; i < 1200; i++ {
		mask, _ = m.step(s.frame(), tw, th, 0.25, nil, nil)
	}
	if n := countFG(mask); n < want*9/10 {
		t.Errorf("object was absorbed into the background (%d px left of %d)", n, want)
	}

	// It leaves: the floor underneath was never learned away, so it vanishes at once.
	s.addRect(40, 30, 60, 50, -60)
	mask, _ = m.step(s.frame(), tw, th, tdt, nil, nil)
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
				mask, st := m.step(s.frame(), tw, th, dt, nil, nil)
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
				mask, _ = m.step(s.frame(), tw, th, tdt, nil, nil)
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
		mask, _ := m.step(frame, tw, th, tdt, robot, nil)
		if n := countFG(mask); n != 0 {
			t.Fatalf("step %d: %d pixels flagged around a masked robot", step, n)
		}
	}

	// The robot is gone: the floor it crossed was never learned as robot.
	mask, _ := m.step(s.frame(), tw, th, tdt, nil, nil)
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
		if _, st := m.step(frame, tw, th, tdt, robot, nil); !st.Warming {
			break
		}
	}
	// The robot drives away from where it started.
	mask, _ := m.step(s.frame(), tw, th, tdt, nil, nil)
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
	m.step(s.frame(), tw, th, tdt, nil, static)
	mask, _ := m.step(s.frame(), tw, th, tdt, nil, static)
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
		_, st := m.step(s.frame(), tw, th, tdt, nil, nil)
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
	if mask, _ := m.step(s.frame(), tw, th, tdt, nil, nil); countFG(mask) != 0 {
		t.Error("after relearning the new lighting should be the background")
	}
}

func TestForegroundModel_AbsorbAt(t *testing.T) {
	m := testModel(nil)
	s := newScene(9, 120)
	warmUp(t, m, s, nil)
	s.addRect(40, 30, 60, 50, 60)
	for i := 0; i < 3; i++ {
		m.step(s.frame(), tw, th, tdt, nil, nil)
	}

	if got := m.absorbAt(0, 0); got != 0 {
		t.Errorf("absorbing empty floor returned %d, want 0", got)
	}
	if got := m.absorbAt(50, 40); got < 350 || got > 450 {
		t.Errorf("absorbed %d px, want about 400", got)
	}
	mask, _ := m.step(s.frame(), tw, th, tdt, nil, nil)
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
		mask, _ = m.step(s.frame(), tw, th, tdt, nil, nil)
	}
	if countFG(mask) == 0 {
		t.Fatal("object vanished before the absorb timeout")
	}
	for i := 0; i < 40; i++ { // past 5 s
		mask, _ = m.step(s.frame(), tw, th, tdt, nil, nil)
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
	if mask, _ := m.step(s.frame(), tw, th, tdt, nil, nil); countFG(mask) != 0 {
		t.Error("after a reset the current scene should be the background")
	}
}

func TestForegroundModel_RejectsWrongSizedFrames(t *testing.T) {
	m := testModel(nil)
	if mask, _ := m.step(make([]uint8, 10), tw, th, tdt, nil, nil); mask != nil {
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
	m.step(s.frame(), tw, th, tdt, nil, nil) // give gain a non-default value to round-trip too

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
	liveMask, _ := live.step(frame, tw, th, tdt, nil, nil)
	restoredMask, restoredStep := restored.step(frame, tw, th, tdt, nil, nil)
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
