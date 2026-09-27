package detection

import (
	"math"
	"sort"
	"time"
)

// TemporalParams tunes how raw per-frame detections become stable obstacles.
type TemporalParams struct {
	// Appear is how long an object must be continuously present before it is
	// published; Vanish is how long it must be absent before it is dropped.
	Appear, Vanish time.Duration
	// MoveEps (metres) is how far any edge must move from the last published
	// box before a new box is published.
	MoveEps float64
	// Quantum (metres) is the grid published edges are rounded outward to.
	Quantum float64
	// MinSize (metres) is the smallest width and height of a detection.
	MinSize float64
	// MergeGap (metres) merges detections that overlap or come this close.
	MergeGap float64
	// MaxTracks caps how many boxes are published (the largest are kept).
	MaxTracks int
}

// DefaultTemporalParams returns the tuned defaults.
func DefaultTemporalParams() TemporalParams {
	return TemporalParams{
		Appear:    400 * time.Millisecond,
		Vanish:    1500 * time.Millisecond,
		MoveEps:   0.02,
		Quantum:   0.02,
		MinSize:   0.05,
		MergeGap:  0.03,
		MaxTracks: 8,
	}
}

// TrackedBox is a published, stable obstacle.
type TrackedBox struct {
	ID  int
	Box WorldBox
}

const (
	// assocMinIoU is the smallest overlap that associates a detection with a
	// track; below it the nearest centre within assocMaxDist is used instead.
	assocMinIoU  = 0.05
	assocMaxDist = 0.15
	// absorbSuppress is how long a detection inside an absorbed obstacle is
	// ignored, measured on the timestamps given to Update.
	absorbSuppress = 5 * time.Second
	// tolerance for quantisation and comparisons on floating point grids.
	gridEpsilon = 1e-9
)

type track struct {
	id        int
	box       WorldBox // latest detection
	firstSeen time.Time
	lastSeen  time.Time
	published bool
	pubBox    WorldBox // quantised box that was last published
	pubRaw    WorldBox // raw box at the moment pubBox was published
}

type suppression struct {
	box   WorldBox
	until time.Time
	set   bool
}

// TemporalFilter turns noisy per-frame boxes into stable obstacles: an object
// must persist to appear and be absent for a while to vanish, its published box
// only changes when it really moves, and published boxes sit on a coarse grid.
// It is not safe for concurrent use.
type TemporalFilter struct {
	p        TemporalParams
	tracks   []*track
	nextID   int
	suppress []suppression
}

// NewTemporalFilter creates a filter with the given parameters.
func NewTemporalFilter(p TemporalParams) *TemporalFilter {
	return &TemporalFilter{p: p, nextID: 1}
}

// Reset forgets all tracks and suppressions. Track IDs keep increasing, so an
// ID is never reused.
func (f *TemporalFilter) Reset() {
	f.tracks = nil
	f.suppress = nil
}

// Absorb removes the track whose box contains the world point and ignores
// detections centred inside that box for absorbSuppress (measured from the next
// Update). It reports whether a track was found.
func (f *TemporalFilter) Absorb(x, y float64) bool {
	for i, t := range f.tracks {
		box := t.box
		if t.published {
			box = t.pubBox
		}
		if !box.Contains(x, y) && !t.box.Contains(x, y) {
			continue
		}
		f.suppress = append(f.suppress, suppression{box: box.Union(t.box)})
		f.tracks = append(f.tracks[:i], f.tracks[i+1:]...)
		return true
	}
	return false
}

// Update feeds one frame's detections observed at now and returns the currently
// published obstacles, sorted by ID.
func (f *TemporalFilter) Update(now time.Time, detections []WorldBox) []TrackedBox {
	f.armSuppressions(now)

	// Drop tracks that have been unseen for Vanish.
	alive := f.tracks[:0]
	for _, t := range f.tracks {
		if now.Sub(t.lastSeen) < f.p.Vanish {
			alive = append(alive, t)
		}
	}
	f.tracks = alive

	dets := f.candidates(detections)
	trackDet, detTrack := f.associate(dets)

	for ti, t := range f.tracks {
		if di := trackDet[ti]; di >= 0 {
			f.observe(t, dets[di], now)
		}
	}
	for di, d := range dets {
		if detTrack[di] >= 0 {
			continue
		}
		t := &track{id: f.nextID, box: d, firstSeen: now, lastSeen: now}
		f.nextID++
		f.tracks = append(f.tracks, t)
		f.observe(t, d, now)
	}

	return f.published()
}

// armSuppressions starts the expiry clock of suppressions created by Absorb
// (which has no clock) and discards expired ones.
func (f *TemporalFilter) armSuppressions(now time.Time) {
	kept := f.suppress[:0]
	for _, s := range f.suppress {
		if !s.set {
			s.until = now.Add(absorbSuppress)
			s.set = true
		}
		if now.Before(s.until) {
			kept = append(kept, s)
		}
	}
	f.suppress = kept
}

// candidates filters and merges the raw detections.
func (f *TemporalFilter) candidates(detections []WorldBox) []WorldBox {
	kept := make([]WorldBox, 0, len(detections))
	for _, d := range detections {
		if d.Width() < f.p.MinSize || d.Height() < f.p.MinSize {
			continue
		}
		cx, cy := d.Center()
		if f.suppressed(cx, cy) {
			continue
		}
		kept = append(kept, d)
	}
	return MergeWorldBoxes(kept, f.p.MergeGap)
}

func (f *TemporalFilter) suppressed(x, y float64) bool {
	for _, s := range f.suppress {
		if s.box.Contains(x, y) {
			return true
		}
	}
	return false
}

// associate matches detections to tracks: greedily by best overlap first, then
// by nearest centre. It returns, per track, the matched detection index (or -1)
// and, per detection, the matched track index (or -1).
func (f *TemporalFilter) associate(dets []WorldBox) (trackDet, detTrack []int) {
	trackDet = make([]int, len(f.tracks))
	for i := range trackDet {
		trackDet[i] = -1
	}
	detTrack = make([]int, len(dets))
	for i := range detTrack {
		detTrack[i] = -1
	}

	type pair struct {
		ti, di int
		iou    float64
	}
	pairs := make([]pair, 0, len(f.tracks)*len(dets))
	for ti, t := range f.tracks {
		for di, d := range dets {
			if iou := t.box.IoU(d); iou >= assocMinIoU {
				pairs = append(pairs, pair{ti, di, iou})
			}
		}
	}
	sort.SliceStable(pairs, func(a, b int) bool {
		if pairs[a].iou != pairs[b].iou {
			return pairs[a].iou > pairs[b].iou
		}
		if pairs[a].ti != pairs[b].ti {
			return pairs[a].ti < pairs[b].ti
		}
		return pairs[a].di < pairs[b].di
	})
	for _, p := range pairs {
		if trackDet[p.ti] == -1 && detTrack[p.di] == -1 {
			trackDet[p.ti], detTrack[p.di] = p.di, p.ti
		}
	}

	for di, d := range dets {
		if detTrack[di] != -1 {
			continue
		}
		dx, dy := d.Center()
		best, bestDist := -1, assocMaxDist
		for ti, t := range f.tracks {
			if trackDet[ti] != -1 {
				continue
			}
			tx, ty := t.box.Center()
			if dist := math.Hypot(dx-tx, dy-ty); dist <= bestDist {
				best, bestDist = ti, dist
			}
		}
		if best >= 0 {
			trackDet[best], detTrack[di] = di, best
		}
	}
	return trackDet, detTrack
}

// observe records that track t was seen at now with box d, publishing it once it
// has been present for Appear and republishing only on real movement.
func (f *TemporalFilter) observe(t *track, d WorldBox, now time.Time) {
	t.box = d
	t.lastSeen = now

	if !t.published {
		if now.Sub(t.firstSeen) >= f.p.Appear {
			t.published = true
			t.pubRaw = d
			t.pubBox = quantise(d, f.p.Quantum)
		}
		return
	}
	if movedBeyond(d, t.pubRaw, f.p.MoveEps) {
		t.pubRaw = d
		t.pubBox = quantise(d, f.p.Quantum)
	}
}

// published returns the published tracks, capped at MaxTracks (largest kept),
// sorted by ID.
func (f *TemporalFilter) published() []TrackedBox {
	out := make([]TrackedBox, 0, len(f.tracks))
	for _, t := range f.tracks {
		if t.published {
			out = append(out, TrackedBox{ID: t.id, Box: t.pubBox})
		}
	}
	if f.p.MaxTracks > 0 && len(out) > f.p.MaxTracks {
		sort.SliceStable(out, func(a, b int) bool {
			aa, ab := out[a].Box.Area(), out[b].Box.Area()
			if aa != ab {
				return aa > ab
			}
			return out[a].ID < out[b].ID
		})
		out = out[:f.p.MaxTracks]
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

func movedBeyond(a, b WorldBox, eps float64) bool {
	limit := eps + gridEpsilon
	return math.Abs(a.MinX-b.MinX) > limit || math.Abs(a.MinY-b.MinY) > limit ||
		math.Abs(a.MaxX-b.MaxX) > limit || math.Abs(a.MaxY-b.MaxY) > limit
}

// quantise rounds the box outward onto a q-metre grid (min edges down, max
// edges up), so a quantised box always contains the raw one. A non-positive q
// leaves the box unchanged.
func quantise(b WorldBox, q float64) WorldBox {
	if q <= 0 {
		return b
	}
	down := func(v float64) float64 { return math.Floor(v/q+gridEpsilon) * q }
	up := func(v float64) float64 { return math.Ceil(v/q-gridEpsilon) * q }
	return WorldBox{MinX: down(b.MinX), MinY: down(b.MinY), MaxX: up(b.MaxX), MaxY: up(b.MaxY)}
}
