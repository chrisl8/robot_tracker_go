package detection

import "math"

// foregroundModel is the pixel-level background model behind the foreground
// detector. It works on small grayscale frames and is deliberately free of
// OpenCV so it can be unit tested with synthetic data.
//
// Design points (see the plan in the repo history):
//   - A slowly adapting float running average, updated only where the pixel is
//     currently background and not covered by a robot, so a placed object stays
//     an obstacle until removed instead of being absorbed.
//   - Pixels covered by a robot during warm-up are "unknown" and learned from
//     the first frame they are uncovered, so a robot leaving its start position
//     leaves no ghost.
//   - Global exposure normalisation from pixels that were background last frame.
//   - Asymmetric threshold: darkening (shadows) must change more than brightening.
type foregroundModel struct {
	p    ForegroundParams
	w, h int

	bg             []float32
	bgB, bgG, bgR  []float32 // background colour (BGR), for shadow suppression; see isShadowColor
	known          []bool
	warmCount      []uint16
	prevRaw        []bool
	lastGray       []uint8
	lastColor      []uint8 // last frame's BGR, valid only when lastColorValid
	lastColorValid bool
	fgSecs         []float32 // only when AbsorbAfterSec > 0
	fg             []uint8   // output mask, 0 or 255

	warmElapsed float64
	warmFrames  int
	warm        bool
	guardSecs   float64
	gain        float64
}

// modelState is the subset of a warm foregroundModel that is worth persisting
// across a restart: everything else (prevRaw, lastGray, warmCount, fgSecs) is
// either purely transient or safe to lose (see snapshot/restore).
type modelState struct {
	W, H  int
	Bg    []float32
	Known []bool
	Gain  float64
}

type modelStep struct {
	Warming  bool
	Guarded  bool
	Fraction float64
	Gain     float64
	// Rewarmed is true on the frame the guard gave up and restarted learning.
	Rewarmed bool
	// ShadowSuppressed is how many pixels this frame darkened enough to look
	// like foreground but were reclassified as a cast shadow by
	// isShadowColor — purely diagnostic (see foregroundGlue.perfSummary),
	// not used by any detection logic itself.
	ShadowSuppressed int
}

const (
	minWarmFrames = 3
	gainStride    = 4
	minGainSample = 200
	minGain       = 0.6
	maxGain       = 1.6
	maxDt         = 1.0
)

func newForegroundModel(p ForegroundParams) *foregroundModel {
	return &foregroundModel{p: p, gain: 1}
}

func (m *foregroundModel) alloc(w, h int) {
	n := w * h
	m.w, m.h = w, h
	m.bg = make([]float32, n)
	m.bgB = make([]float32, n)
	m.bgG = make([]float32, n)
	m.bgR = make([]float32, n)
	m.known = make([]bool, n)
	m.warmCount = make([]uint16, n)
	m.prevRaw = make([]bool, n)
	m.lastGray = make([]uint8, n)
	m.lastColor = make([]uint8, n*3)
	m.fg = make([]uint8, n)
	m.fgSecs = nil
	if m.p.AbsorbAfterSec > 0 {
		m.fgSecs = make([]float32, n)
	}
	m.reset()
}

// reset forgets the background and starts warm-up again.
func (m *foregroundModel) reset() {
	for i := range m.bg {
		m.bg[i] = 0
		m.bgB[i] = 0
		m.bgG[i] = 0
		m.bgR[i] = 0
		m.known[i] = false
		m.warmCount[i] = 0
		m.prevRaw[i] = false
		m.fg[i] = 0
	}
	for i := range m.fgSecs {
		m.fgSecs[i] = 0
	}
	m.warmElapsed, m.warmFrames, m.warm = 0, 0, false
	m.guardSecs = 0
	m.gain = 1
	m.lastColorValid = false
}

// snapshot returns the model's persistable state, or ok == false if the model
// is not warm yet (nothing worth saving).
func (m *foregroundModel) snapshot() (s modelState, ok bool) {
	if !m.warm {
		return modelState{}, false
	}
	s = modelState{W: m.w, H: m.h, Gain: m.gain}
	s.Bg = append([]float32(nil), m.bg...)
	s.Known = append([]bool(nil), m.known...)
	return s, true
}

// restore replaces the model's state with a previously-saved snapshot,
// marking it warm immediately (skipping warm-up). It is the mirror image of
// alloc, deliberately not sharing its implementation: alloc always ends by
// calling reset, which is exactly what must not happen to restored state.
// It returns false (leaving the model untouched) if s is not a usable size.
func (m *foregroundModel) restore(s modelState) bool {
	if s.W <= 0 || s.H <= 0 || len(s.Bg) != s.W*s.H || len(s.Known) != s.W*s.H {
		return false
	}
	n := s.W * s.H
	m.w, m.h = s.W, s.H
	m.bg = append([]float32(nil), s.Bg...)
	m.known = append([]bool(nil), s.Known...)
	m.gain = s.Gain
	if m.gain == 0 {
		m.gain = 1
	}
	m.warmCount = make([]uint16, n)
	m.prevRaw = make([]bool, n)
	m.lastGray = make([]uint8, n)
	m.lastColor = make([]uint8, n*3)
	m.lastColorValid = false
	m.fg = make([]uint8, n)
	m.fgSecs = nil
	if m.p.AbsorbAfterSec > 0 {
		m.fgSecs = make([]float32, n)
	}
	// bgB/bgG/bgR (colour, for shadow suppression) are not part of a
	// snapshot yet, so they start zeroed here — isShadowColor's bDotB guard
	// means the shadow gate simply stays inactive until they ramp back up
	// through ordinary background adaptation, rather than misfiring on
	// stale/absent colour data.
	m.bgB = make([]float32, n)
	m.bgG = make([]float32, n)
	m.bgR = make([]float32, n)
	m.warmElapsed, m.warmFrames = 0, 0
	m.warm = true
	m.guardSecs = 0
	return true
}

// step advances the model by one frame. gray is w*h working-resolution
// pixels; robotMask and staticMask (nil or w*h, nonzero = covered) exclude
// areas. color, if not nil, is the same pixels' BGR colour (w*h*3, packed
// BGRBGR...) at the same exposure gain as gray — supplying it enables shadow
// suppression (see isShadowColor); nil (or a mismatched length, treated the
// same as nil) leaves darkening classified exactly as before it existed. The
// returned mask (owned by the model, valid until the next call) is 255 where
// a pixel counts as foreground outside the masks.
func (m *foregroundModel) step(gray []uint8, w, h int, dt float64, robotMask, staticMask []uint8, color []uint8) ([]uint8, modelStep) {
	if len(gray) != w*h {
		return nil, modelStep{}
	}
	if len(color) != w*h*3 {
		color = nil
	}
	if m.w != w || m.h != h || m.bg == nil {
		m.alloc(w, h)
	}
	dt = math.Min(math.Max(dt, 0), maxDt)
	copy(m.lastGray, gray)
	m.lastColorValid = color != nil
	if color != nil {
		copy(m.lastColor, color)
	}

	if !m.warm {
		return m.warmStep(gray, dt, robotMask, color)
	}

	gain := m.estimateGain(gray, robotMask)
	m.gain = gain

	alpha := float32(1 - math.Exp(-dt/math.Max(m.p.TauSec, 0.001)))
	thrUp := float32(m.p.Threshold)
	thrDown := float32(m.p.Threshold * m.p.DarkFactor)
	absorb := float32(m.p.AbsorbAfterSec)
	fdt := float32(dt)
	border := m.p.BorderPx
	g := float32(gain)
	shadowAlphaMin := float32(m.p.ShadowAlphaMin)
	shadowAlphaMax := float32(m.p.ShadowAlphaMax)
	shadowChromaMax := float32(m.p.ShadowChromaMax)

	fgCount := 0
	shadowCount := 0
	for y := 0; y < h; y++ {
		inBorderY := y < border || y >= h-border
		row := y * w
		for x := 0; x < w; x++ {
			i := row + x
			robot := robotMask != nil && robotMask[i] != 0

			cur := float32(gray[i]) * g
			var curB, curG, curR float32
			if color != nil {
				curB = float32(color[i*3]) * g
				curG = float32(color[i*3+1]) * g
				curR = float32(color[i*3+2]) * g
			}
			if !m.known[i] {
				// Unknown (covered at warm-up): learn as soon as it is uncovered.
				if !robot {
					m.bg[i] = cur
					m.known[i] = true
					if color != nil {
						m.bgB[i], m.bgG[i], m.bgR[i] = curB, curG, curR
					}
				}
				m.fg[i] = 0
				m.prevRaw[i] = false
				continue
			}

			d := cur - m.bg[i]
			raw := d > thrUp || d < -thrDown
			if raw && d < 0 && color != nil &&
				isShadowColor(curB, curG, curR, m.bgB[i], m.bgG[i], m.bgR[i], shadowAlphaMin, shadowAlphaMax, shadowChromaMax) {
				// Darker, but still just the background colour scaled down: a
				// cast shadow, not a real change. Leave it classified as
				// background rather than foreground.
				raw = false
				shadowCount++
			}
			if inBorderY || x < border || x >= w-border {
				raw = false
			}
			m.prevRaw[i] = raw

			masked := robot || (staticMask != nil && staticMask[i] != 0)
			if raw && !masked {
				m.fg[i] = 255
				fgCount++
			} else {
				m.fg[i] = 0
			}

			switch {
			case robot:
				// Never learn a robot into the background.
			case !raw:
				m.bg[i] += alpha * d
				if color != nil {
					m.bgB[i] += alpha * (curB - m.bgB[i])
					m.bgG[i] += alpha * (curG - m.bgG[i])
					m.bgR[i] += alpha * (curR - m.bgR[i])
				}
				if m.fgSecs != nil {
					m.fgSecs[i] = 0
				}
			case m.fgSecs != nil:
				m.fgSecs[i] += fdt
				if m.fgSecs[i] > absorb {
					m.bg[i] = cur
					if color != nil {
						m.bgB[i], m.bgG[i], m.bgR[i] = curB, curG, curR
					}
					m.fgSecs[i] = 0
				}
			}
		}
	}

	st := modelStep{Gain: gain, Fraction: float64(fgCount) / float64(w*h), ShadowSuppressed: shadowCount}
	if st.Fraction > m.p.GuardFraction {
		st.Guarded = true
		m.guardSecs += dt
		if m.p.GuardMaxSec > 0 && m.guardSecs > m.p.GuardMaxSec {
			// A lasting lighting change: relearn everything.
			m.reset()
			st.Rewarmed = true
			st.Warming = true
			st.Guarded = false
		}
	} else {
		m.guardSecs = 0
	}
	return m.fg, st
}

func (m *foregroundModel) warmStep(gray []uint8, dt float64, robotMask []uint8, color []uint8) ([]uint8, modelStep) {
	for i, v := range gray {
		if robotMask != nil && robotMask[i] != 0 {
			continue
		}
		c := m.warmCount[i]
		if c < math.MaxUint16 {
			c++
			m.warmCount[i] = c
		}
		m.bg[i] += (float32(v) - m.bg[i]) / float32(c)
		if color != nil {
			m.bgB[i] += (float32(color[i*3]) - m.bgB[i]) / float32(c)
			m.bgG[i] += (float32(color[i*3+1]) - m.bgG[i]) / float32(c)
			m.bgR[i] += (float32(color[i*3+2]) - m.bgR[i]) / float32(c)
		}
		m.known[i] = true
	}
	m.warmElapsed += dt
	m.warmFrames++
	if m.warmElapsed >= m.p.WarmupSec && m.warmFrames >= minWarmFrames {
		m.warm = true
	}
	for i := range m.fg {
		m.fg[i] = 0
	}
	return m.fg, modelStep{Warming: !m.warm, Gain: 1}
}

// estimateGain returns the multiplier that brings the current frame's
// brightness to the background's, measured on pixels that were background last
// frame (so objects do not skew it). It absorbs auto-exposure changes.
func (m *foregroundModel) estimateGain(gray []uint8, robotMask []uint8) float64 {
	var sumBg, sumCur float64
	n := 0
	for y := 0; y < m.h; y += gainStride {
		row := y * m.w
		for x := 0; x < m.w; x += gainStride {
			i := row + x
			if !m.known[i] || m.prevRaw[i] || (robotMask != nil && robotMask[i] != 0) {
				continue
			}
			sumBg += float64(m.bg[i])
			sumCur += float64(gray[i])
			n++
		}
	}
	if n < minGainSample || sumCur <= 0 {
		return 1
	}
	return math.Min(math.Max(sumBg/sumCur, minGain), maxGain)
}

// isShadowColor reports whether current colour (curB, curG, curR) — a pixel
// that has already darkened enough to look like foreground — is explained by
// the background colour (bgB, bgG, bgR) scaled down by some factor, rather
// than a real colour change. This is the standard background-subtraction
// shadow test (Horprasert et al.): a cast shadow scales a pixel's colour down
// roughly uniformly (same direction, lower magnitude); a real object usually
// changes the colour direction too. Working in raw BGR (rather than HSV hue)
// matters on a low-saturation floor, where hue is numerically unstable near
// the achromatic axis and would be noisy exactly where this needs to work.
//
// scale is the least-squares best fit of curr ~= scale*bg (the projection of
// curr onto bg); it must land in [scaleMin, scaleMax] — darker, but not
// implausibly darker for a shadow. chroma is the leftover colour error after
// removing that scale, normalised by the background colour's own magnitude;
// it must be at most chromaMaxFrac. A near-black background (bg's squared
// magnitude below 1) never counts as a shadow: the scale factor is
// meaningless there, and the plain brightness threshold governs instead.
func isShadowColor(curB, curG, curR, bgB, bgG, bgR, scaleMin, scaleMax, chromaMaxFrac float32) bool {
	bDotB := bgB*bgB + bgG*bgG + bgR*bgR
	if bDotB < 1 {
		return false
	}
	scale := (curB*bgB + curG*bgG + curR*bgR) / bDotB
	if scale < scaleMin || scale > scaleMax {
		return false
	}
	rb, rg, rr := curB-scale*bgB, curG-scale*bgG, curR-scale*bgR
	chroma2 := rb*rb + rg*rg + rr*rr
	return chroma2 <= chromaMaxFrac*chromaMaxFrac*bDotB
}

// absorbAt folds the connected foreground region containing working-resolution
// pixel (x, y) into the background (the operator declares it part of the floor).
// It reports how many pixels were absorbed.
func (m *foregroundModel) absorbAt(x, y int) int {
	if m.bg == nil || x < 0 || y < 0 || x >= m.w || y >= m.h || !m.prevRaw[y*m.w+x] {
		return 0
	}
	g := float32(m.gain)
	visited := make(map[int]bool)
	queue := []int{y*m.w + x}
	visited[queue[0]] = true
	count := 0
	for len(queue) > 0 {
		i := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		m.bg[i] = float32(m.lastGray[i]) * g
		if m.lastColorValid {
			m.bgB[i] = float32(m.lastColor[i*3]) * g
			m.bgG[i] = float32(m.lastColor[i*3+1]) * g
			m.bgR[i] = float32(m.lastColor[i*3+2]) * g
		}
		m.prevRaw[i] = false
		m.fg[i] = 0
		count++
		px, py := i%m.w, i/m.w
		for _, nb := range [4][2]int{{px + 1, py}, {px - 1, py}, {px, py + 1}, {px, py - 1}} {
			if nb[0] < 0 || nb[1] < 0 || nb[0] >= m.w || nb[1] >= m.h {
				continue
			}
			j := nb[1]*m.w + nb[0]
			if !visited[j] && m.prevRaw[j] {
				visited[j] = true
				queue = append(queue, j)
			}
		}
	}
	return count
}
