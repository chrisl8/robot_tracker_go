package detection

import (
	"image"
	"math"
	"sort"
)

// WorldBox is an axis-aligned rectangle on the floor, in metres.
type WorldBox struct{ MinX, MinY, MaxX, MaxY float64 }

// Width is the extent along x.
func (b WorldBox) Width() float64 { return b.MaxX - b.MinX }

// Height is the extent along y.
func (b WorldBox) Height() float64 { return b.MaxY - b.MinY }

// Area is width*height, or 0 for an inverted or degenerate box.
func (b WorldBox) Area() float64 {
	w, h := b.Width(), b.Height()
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

// Center returns the middle of the box.
func (b WorldBox) Center() (x, y float64) {
	return (b.MinX + b.MaxX) / 2, (b.MinY + b.MaxY) / 2
}

// Expand grows the box by m metres on every side.
func (b WorldBox) Expand(m float64) WorldBox {
	return WorldBox{MinX: b.MinX - m, MinY: b.MinY - m, MaxX: b.MaxX + m, MaxY: b.MaxY + m}
}

// Union is the smallest box containing both.
func (b WorldBox) Union(o WorldBox) WorldBox {
	return WorldBox{
		MinX: math.Min(b.MinX, o.MinX), MinY: math.Min(b.MinY, o.MinY),
		MaxX: math.Max(b.MaxX, o.MaxX), MaxY: math.Max(b.MaxY, o.MaxY),
	}
}

// Intersects reports whether the boxes overlap or touch.
func (b WorldBox) Intersects(o WorldBox) bool {
	return b.MinX <= o.MaxX && o.MinX <= b.MaxX && b.MinY <= o.MaxY && o.MinY <= b.MaxY
}

// IoU is intersection area over union area (0 when they do not overlap).
func (b WorldBox) IoU(o WorldBox) float64 {
	iw := math.Min(b.MaxX, o.MaxX) - math.Max(b.MinX, o.MinX)
	ih := math.Min(b.MaxY, o.MaxY) - math.Max(b.MinY, o.MinY)
	if iw <= 0 || ih <= 0 {
		return 0
	}
	inter := iw * ih
	union := b.Area() + o.Area() - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

// Contains reports whether the point lies inside the box (edges included).
func (b WorldBox) Contains(x, y float64) bool {
	return x >= b.MinX && x <= b.MaxX && y >= b.MinY && y <= b.MaxY
}

// Quad is an obstacle's exact footprint: four world-metre corners in a
// consistent winding order. Every Quad this package produces is a rectangle
// (axis-aligned, or oriented via MinAreaRect) — never an arbitrary polygon —
// which ExpandRect relies on.
type Quad [4][2]float64

// signedArea2 returns twice the signed area (its sign gives the winding).
func (q Quad) signedArea2() float64 {
	sum := 0.0
	for i := 0; i < 4; i++ {
		a, b := q[i], q[(i+1)%4]
		sum += a[0]*b[1] - b[0]*a[1]
	}
	return sum
}

// Normalized returns q with a consistent winding, reversed if projecting
// through a reflective transform flipped it.
func (q Quad) Normalized() Quad {
	if q.signedArea2() < 0 {
		return Quad{q[0], q[3], q[2], q[1]}
	}
	return q
}

// ExpandRect grows the quad by m metres along each of its two axes (so a
// rectangle w x h becomes (w+2m) x (h+2m), same as WorldBox.Expand for an
// axis-aligned box). Exact for any rectangle, which is everything this
// package's quads are; degenerate (zero-length) quads are returned unchanged.
func (q Quad) ExpandRect(m float64) Quad {
	if m == 0 {
		return q
	}
	ux, uy := q[1][0]-q[0][0], q[1][1]-q[0][1] // along one edge
	vx, vy := q[3][0]-q[0][0], q[3][1]-q[0][1] // along the adjacent edge
	ulen, vlen := math.Hypot(ux, uy), math.Hypot(vx, vy)
	if ulen < 1e-12 || vlen < 1e-12 {
		return q
	}
	ux, uy = ux/ulen*m, uy/ulen*m
	vx, vy = vx/vlen*m, vy/vlen*m
	return Quad{
		{q[0][0] - ux - vx, q[0][1] - uy - vy},
		{q[1][0] + ux - vx, q[1][1] + uy - vy},
		{q[2][0] + ux + vx, q[2][1] + uy + vy},
		{q[3][0] - ux + vx, q[3][1] - uy + vy},
	}
}

// PixelToWorldFunc maps a full-resolution frame pixel to floor metres.
type PixelToWorldFunc func(px, py float64) (wx, wy float64)

// BlobQuadToWorld projects each of the blob's four (possibly rotated) corners
// individually onto the floor, giving its exact footprint rather than an
// axis-aligned bounding box of it.
func BlobQuadToWorld(corners [4]image.Point, fn PixelToWorldFunc) Quad {
	var q Quad
	for i, c := range corners {
		x, y := fn(float64(c.X), float64(c.Y))
		q[i] = [2]float64{x, y}
	}
	return q.Normalized()
}

// BlobToWorld projects all four corners of r onto the floor and returns their
// bounding box. Under perspective or rotation the box through two opposite
// corners can miss the other two, so all four are used.
func BlobToWorld(r image.Rectangle, fn PixelToWorldFunc) WorldBox {
	corners := [4][2]float64{
		{float64(r.Min.X), float64(r.Min.Y)},
		{float64(r.Max.X), float64(r.Min.Y)},
		{float64(r.Max.X), float64(r.Max.Y)},
		{float64(r.Min.X), float64(r.Max.Y)},
	}
	box := WorldBox{MinX: math.Inf(1), MinY: math.Inf(1), MaxX: math.Inf(-1), MaxY: math.Inf(-1)}
	for _, c := range corners {
		wx, wy := fn(c[0], c[1])
		box.MinX = math.Min(box.MinX, wx)
		box.MinY = math.Min(box.MinY, wy)
		box.MaxX = math.Max(box.MaxX, wx)
		box.MaxY = math.Max(box.MaxY, wy)
	}
	return box
}

// pixelsPerMeterStep is the pixel step used to measure the local scale.
const pixelsPerMeterStep = 10.0

// LocalPixelsPerMeter measures the image scale at (px, py): pixels per metre,
// averaged over a step in x and a step in y. It returns 0 if the mapping is
// degenerate there.
func LocalPixelsPerMeter(fn PixelToWorldFunc, px, py float64) float64 {
	x0, y0 := fn(px, py)
	x1, y1 := fn(px+pixelsPerMeterStep, py)
	x2, y2 := fn(px, py+pixelsPerMeterStep)
	dx := math.Hypot(x1-x0, y1-y0)
	dy := math.Hypot(x2-x0, y2-y0)
	if !(dx > 0) || !(dy > 0) || math.IsInf(dx, 0) || math.IsInf(dy, 0) {
		return 0
	}
	return (pixelsPerMeterStep/dx + pixelsPerMeterStep/dy) / 2
}

// RobotDisc returns the disc, in frame pixels, covering a robot of the given
// diameter plus margin (metres) centred on (cx, cy). The radius uses the local
// image scale; it is zero if that scale is degenerate.
func RobotDisc(fn PixelToWorldFunc, cx, cy, robotDiameterM, marginM float64) Disc {
	ppm := LocalPixelsPerMeter(fn, cx, cy)
	if ppm <= 0 {
		return Disc{X: cx, Y: cy}
	}
	return Disc{X: cx, Y: cy, R: (robotDiameterM/2 + marginM) * ppm}
}

// MergeWorldBoxesWithGroups repeatedly merges boxes that overlap or lie within
// gap metres of each other, and also returns,
// for each returned box (same order, same index), the indices into the input
// slice that were merged into it (a single-element slice when that box did
// not merge with any other). A caller that has per-input data alongside each
// box — such as an oriented quad — needs this: the returned boxes are sorted,
// so their order does not match the input's, and pairing per-input data back
// up positionally (assuming "same count in, same count out" means "same
// order") silently mismatches data across boxes whenever the input wasn't
// already sorted.
func MergeWorldBoxesWithGroups(boxes []WorldBox, gap float64) ([]WorldBox, [][]int) {
	out := make([]WorldBox, len(boxes))
	copy(out, boxes)
	groups := make([][]int, len(boxes))
	for i := range groups {
		groups[i] = []int{i}
	}

	for merged := true; merged; {
		merged = false
	scan:
		for i := 0; i < len(out); i++ {
			for j := i + 1; j < len(out); j++ {
				if out[i].Expand(gap).Intersects(out[j]) {
					out[i] = out[i].Union(out[j])
					groups[i] = append(groups[i], groups[j]...)
					out = append(out[:j], out[j+1:]...)
					groups = append(groups[:j], groups[j+1:]...)
					merged = true
					break scan
				}
			}
		}
	}

	idx := make([]int, len(out))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool {
		ba, bb := out[idx[a]], out[idx[b]]
		if ba.MinX != bb.MinX {
			return ba.MinX < bb.MinX
		}
		return ba.MinY < bb.MinY
	})
	sortedOut := make([]WorldBox, len(out))
	sortedGroups := make([][]int, len(out))
	for i, gi := range idx {
		sortedOut[i] = out[gi]
		sortedGroups[i] = groups[gi]
	}
	return sortedOut, sortedGroups
}
