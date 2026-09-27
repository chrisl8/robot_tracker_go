package planning

import "math"

// Quad is an obstacle's footprint as four corners in world metres, in a
// consistent winding order. A plain axis-aligned rectangle is just a
// degenerate quad (its own four corners), so every Obstacle always carries
// one and downstream code never needs a nil/rectangle special case.
type Quad [4][2]float64

// RectQuad returns the axis-aligned quad for the rectangle spanned by
// topLeft and bottomRight, in a consistent (clockwise, in a Y-down world)
// winding order matching what MinAreaRect-derived quads use.
func RectQuad(topLeft, bottomRight [2]float64) Quad {
	return Quad{
		{topLeft[0], topLeft[1]},
		{bottomRight[0], topLeft[1]},
		{bottomRight[0], bottomRight[1]},
		{topLeft[0], bottomRight[1]},
	}
}

// signedArea2 returns twice the signed area of the quad (positive for
// clockwise winding in a Y-down coordinate system, matching RectQuad).
func (q Quad) signedArea2() float64 {
	sum := 0.0
	for i := 0; i < 4; i++ {
		a, b := q[i], q[(i+1)%4]
		sum += a[0]*b[1] - b[0]*a[1]
	}
	return sum
}

// Normalized returns q with a consistent winding: reversed if projecting its
// source pixels through a reflective transform flipped it. Callers should
// apply this once, right after building a quad from projected points, so every
// later consumer can assume one winding order.
func (q Quad) Normalized() Quad {
	if q.signedArea2() < 0 {
		return Quad{q[0], q[3], q[2], q[1]}
	}
	return q
}

// Bounds returns the quad's axis-aligned bounding box as (topLeft, bottomRight).
func (q Quad) Bounds() (topLeft, bottomRight [2]float64) {
	minX, minY := q[0][0], q[0][1]
	maxX, maxY := minX, minY
	for _, p := range q[1:] {
		minX, maxX = math.Min(minX, p[0]), math.Max(maxX, p[0])
		minY, maxY = math.Min(minY, p[1]), math.Max(maxY, p[1])
	}
	return [2]float64{minX, minY}, [2]float64{maxX, maxY}
}

// quadContains reports whether p lies inside (or on the boundary of) the
// quad, using the standard even-odd ray-casting test. It assumes q is
// convex, which every quad this package builds is (a plain rectangle, or an
// OpenCV MinAreaRect result).
func quadContains(p [2]float64, q Quad) bool {
	inside := false
	for i, j := 0, 3; i < 4; j, i = i, i+1 {
		xi, yi := q[i][0], q[i][1]
		xj, yj := q[j][0], q[j][1]
		if (yi > p[1]) != (yj > p[1]) &&
			p[0] < (xj-xi)*(p[1]-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}

// distToSegment returns the distance from p to the segment a-b.
func distToSegment(p, a, b [2]float64) float64 {
	abx, aby := b[0]-a[0], b[1]-a[1]
	apx, apy := p[0]-a[0], p[1]-a[1]
	abLen2 := abx*abx + aby*aby
	if abLen2 < 1e-18 {
		return math.Hypot(apx, apy)
	}
	t := (apx*abx + apy*aby) / abLen2
	t = math.Max(0, math.Min(1, t))
	dx := p[0] - (a[0] + t*abx)
	dy := p[1] - (a[1] + t*aby)
	return math.Hypot(dx, dy)
}

// distanceToQuad returns the distance from p to the quad: 0 when p is inside
// (or on) it, otherwise the shortest distance to any of its four edges. This
// is the one primitive both the A* grid rasterization and the per-frame
// clearance checks use: a point at distance <= margin is "within margin" of
// the obstacle's exact footprint, replacing the old expand-the-rectangle
// then AABB-clamp approach.
func distanceToQuad(p [2]float64, q Quad) float64 {
	if quadContains(p, q) {
		return 0
	}
	min := math.Inf(1)
	for i := 0; i < 4; i++ {
		d := distToSegment(p, q[i], q[(i+1)%4])
		if d < min {
			min = d
		}
	}
	return min
}
