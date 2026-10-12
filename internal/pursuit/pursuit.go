// Package pursuit picks the point a robot should steer at along a planned
// path: a "carrot" a fixed distance ahead of its progress along the path, held
// back at bends until the robot is close enough to pivot there.
//
// It is pure geometry on where the robot is now. There is no speed, no time
// and no model of the robot: the camera is the odometry, and a robot that
// slips, stalls or lurches is simply somewhere else next frame. The output is
// a point, so the existing bearing-based steering (controller.PathExecutor,
// with all its turn/burst/nudge/reverse tuning) is used unchanged. See
// docs/pure-pursuit-plan.md.
package pursuit

import "math"

// World is what the tracker needs to know about the obstacles. The planner
// provides it (a robot's clearances are its own).
type World interface {
	// CanDriveStraight reports whether the robot can go straight from a to b
	// without taking its centre closer to an obstacle than a or b already are.
	CanDriveStraight(a, b [2]float64) bool
}

// Params are all in metres or degrees: nothing depends on how fast a given
// robot happens to be.
type Params struct {
	Lookahead     float64 // how far ahead of its progress the carrot sits
	ArrivalRadius float64 // how close the robot must get to a bend before the carrot goes round it
	BendDeg       float64 // turns sharper than this are bends the carrot waits at
}

// DefaultParams scales the look-ahead and the arrival radius to the robot's
// size, so another robot needs no retuning. The arrival radius (0.06 m for a
// 0.30 m robot) is tighter than the old waypoint threshold (0.10 m), which cut
// a corner by that much, and still above the camera's position noise, which is
// a few centimetres. It is fixed, not widened where there is room: in the
// simulator a radius that adapts to the room around the bend did no better, even
// with the camera's noise doubled (docs/pure-pursuit-plan.md).
func DefaultParams(diameter float64) Params {
	return Params{Lookahead: diameter, ArrivalRadius: 0.2 * diameter, BendDeg: 15}
}

const (
	searchBack = 0.15 // how far back along the path progress may move (a robot that reverses)
	losStep    = 0.05 // how much the carrot is pulled back per try when the way is not clear
	epsilon    = 1e-9
)

// Tracker follows one path.
type Tracker struct {
	p       Params
	w       World
	path    [][2]float64
	cum     []float64 // arclength at each vertex
	bends   []int     // interior vertices that are bends, in order
	cleared int       // bends[:cleared] have been arrived at or passed
	s       float64   // progress along the path
}

// New returns a tracker with no path.
func New(p Params, w World) *Tracker { return &Tracker{p: p, w: w} }

// SetPath starts following path from its beginning.
func (t *Tracker) SetPath(path [][2]float64) {
	t.path = path
	t.cum = make([]float64, len(path))
	for i := 1; i < len(path); i++ {
		t.cum[i] = t.cum[i-1] + math.Hypot(path[i][0]-path[i-1][0], path[i][1]-path[i-1][1])
	}
	t.bends = t.bends[:0]
	for i := 1; i+1 < len(path); i++ {
		if turnDeg(path[i-1], path[i], path[i+1]) > t.p.BendDeg {
			t.bends = append(t.bends, i)
		}
	}
	t.cleared = 0
	t.s = 0
}

// Progress is how far along the path the robot is, metres.
func (t *Tracker) Progress() float64 { return t.s }

// Length is the path's length, metres.
func (t *Tracker) Length() float64 {
	if len(t.cum) == 0 {
		return 0
	}
	return t.cum[len(t.cum)-1]
}

// NextVertex is the index of the first path vertex ahead of the robot's
// progress (the last vertex once it is there): the "next waypoint", for
// anything that shows or reports the path.
func (t *Tracker) NextVertex() int {
	for i := 1; i < len(t.cum); i++ {
		if t.cum[i] > t.s+epsilon {
			return i
		}
	}
	return max(0, len(t.cum)-1)
}

// Target is the point to steer at given where the robot is now. ok is false
// when there is no path.
func (t *Tracker) Target(pos [2]float64) (target [2]float64, ok bool) {
	switch len(t.path) {
	case 0:
		return pos, false
	case 1:
		return t.path[0], true
	}
	t.advance(pos)
	t.clearBends(pos)

	sc := math.Min(t.s+t.p.Lookahead, t.Length())
	if t.cleared < len(t.bends) {
		sc = math.Min(sc, t.cum[t.bends[t.cleared]])
	}
	target = t.pointAt(sc)
	for sc > t.s+epsilon && !t.w.CanDriveStraight(pos, target) {
		sc = math.Max(t.s, sc-losStep)
		target = t.pointAt(sc)
	}
	return target, true
}

// advance moves progress to the point of the path nearest pos, looking only a
// short way back and a good way ahead of where it was. A path that passes
// close to itself (a detour round a block) must not snap the robot to the
// wrong leg.
func (t *Tracker) advance(pos [2]float64) {
	lo := math.Max(0, t.s-searchBack)
	hi := math.Min(t.Length(), t.s+math.Max(1.0, 3*t.p.Lookahead))
	bestS, bestD := t.s, math.Inf(1)
	for i := 0; i+1 < len(t.path); i++ {
		if t.cum[i+1] < lo || t.cum[i] > hi {
			continue
		}
		a, b := t.path[i], t.path[i+1]
		abx, aby := b[0]-a[0], b[1]-a[1]
		length := t.cum[i+1] - t.cum[i]
		frac := 0.0
		if length > 0 {
			frac = math.Max(0, math.Min(1, ((pos[0]-a[0])*abx+(pos[1]-a[1])*aby)/(length*length)))
		}
		s := math.Max(lo, math.Min(hi, t.cum[i]+frac*length))
		q := t.pointAt(s)
		d := math.Hypot(pos[0]-q[0], pos[1]-q[1])
		if d < bestD-epsilon || (math.Abs(d-bestD) <= epsilon && s > bestS) {
			bestS, bestD = s, d
		}
	}
	t.s = bestS
}

// clearBends marks bends as done, in order, once the robot has arrived at one
// (within the arrival radius) or has gone past it.
func (t *Tracker) clearBends(pos [2]float64) {
	for t.cleared < len(t.bends) {
		i := t.bends[t.cleared]
		v := t.path[i]
		arrived := math.Hypot(pos[0]-v[0], pos[1]-v[1]) < t.p.ArrivalRadius
		if !arrived && t.s <= t.cum[i] {
			return
		}
		t.cleared++
	}
}

// pointAt is the point at arclength s along the path.
func (t *Tracker) pointAt(s float64) [2]float64 {
	s = math.Max(0, math.Min(t.Length(), s))
	lo, hi := 0, len(t.cum)-1
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if t.cum[mid] <= s {
			lo = mid
		} else {
			hi = mid
		}
	}
	seg := t.cum[lo+1] - t.cum[lo]
	if seg <= 0 {
		return t.path[lo]
	}
	f := (s - t.cum[lo]) / seg
	return [2]float64{
		t.path[lo][0] + f*(t.path[lo+1][0]-t.path[lo][0]),
		t.path[lo][1] + f*(t.path[lo+1][1]-t.path[lo][1]),
	}
}

// turnDeg is how sharply the path turns at b, degrees.
func turnDeg(a, b, c [2]float64) float64 {
	h1 := math.Atan2(b[1]-a[1], b[0]-a[0])
	h2 := math.Atan2(c[1]-b[1], c[0]-b[0])
	d := math.Abs(h2 - h1)
	if d > math.Pi {
		d = 2*math.Pi - d
	}
	return d * 180 / math.Pi
}

// SamePath reports whether two paths have identical points: how a follower
// notices the planner has replanned.
func SamePath(a, b [][2]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
