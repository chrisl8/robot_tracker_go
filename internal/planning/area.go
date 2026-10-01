package planning

import "math"

// Area is the convex region of the floor the robots may be driven on: the
// part of the world the camera can see. A robot that leaves it is tracked by
// a tag that is partly or wholly out of frame, so detection drops and the
// robot is lost. It is the camera frame's four corners projected onto the
// floor (a convex quadrilateral under a homography), in either winding.
type Area struct {
	corners [4][2]float64
}

// NewArea returns the area bounded by the four corners, which must form a
// convex quadrilateral with non-zero area; ok is false otherwise (for example
// when the calibration puts part of the frame above the horizon, so the
// corners cannot describe the visible floor).
func NewArea(corners [4][2]float64) (area *Area, ok bool) {
	var sign float64
	for i := range corners {
		a, b, c := corners[i], corners[(i+1)%4], corners[(i+2)%4]
		cross := (b[0]-a[0])*(c[1]-b[1]) - (b[1]-a[1])*(c[0]-b[0])
		if math.IsNaN(cross) || math.IsInf(cross, 0) || cross == 0 {
			return nil, false
		}
		if sign == 0 {
			sign = cross
		} else if (cross > 0) != (sign > 0) {
			return nil, false
		}
	}
	if sign < 0 {
		// Normalise to counter-clockwise so "inside" is the left of every edge.
		corners[1], corners[3] = corners[3], corners[1]
	}
	return &Area{corners: corners}, true
}

// edgeDistances returns how far p is inside each edge (negative: outside it).
func (a *Area) edgeDistances(p [2]float64) [4]float64 {
	var d [4]float64
	for i := range a.corners {
		c0, c1 := a.corners[i], a.corners[(i+1)%4]
		ex, ey := c1[0]-c0[0], c1[1]-c0[1]
		d[i] = (ex*(p[1]-c0[1]) - ey*(p[0]-c0[0])) / math.Hypot(ex, ey)
	}
	return d
}

// limits returns, per edge, the least distance from that edge a path point may
// have: inset, except that a robot already closer than that to an edge (it was
// driven or placed there) may keep its current distance so it is not trapped,
// and can still be led away from the edge but never further towards it.
func (a *Area) limits(inset float64, start [2]float64) [4]float64 {
	limit := a.edgeDistances(start)
	for i := range limit {
		limit[i] = math.Min(limit[i], inset)
	}
	return limit
}

// Contains reports whether p is at least inset inside every edge of the area.
func (a *Area) Contains(p [2]float64, inset float64) bool {
	for _, d := range a.edgeDistances(p) {
		if d < inset {
			return false
		}
	}
	return true
}

func (a *Area) equal(b *Area, eps float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	for i := range a.corners {
		if math.Abs(a.corners[i][0]-b.corners[i][0]) > eps || math.Abs(a.corners[i][1]-b.corners[i][1]) > eps {
			return false
		}
	}
	return true
}
