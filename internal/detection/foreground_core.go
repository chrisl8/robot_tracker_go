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

// PixelToWorldFunc maps a full-resolution frame pixel to floor metres.
type PixelToWorldFunc func(px, py float64) (wx, wy float64)

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

// coverageGrid is the number of sample cells per side used to estimate how
// much of a blob a mask covers.
const coverageGrid = 12

// FilterMaskedBlobs drops blobs that the masks cover by at least frac of
// their area, and always drops empty blobs. Coverage is estimated at the
// centres of a fixed coverageGrid x coverageGrid grid inside each blob, so the
// result is deterministic. Order is preserved.
func FilterMaskedBlobs(blobs []image.Rectangle, discs []Disc, rects []image.Rectangle, frac float64) []image.Rectangle {
	kept := make([]image.Rectangle, 0, len(blobs))
	for _, b := range blobs {
		if b.Empty() {
			continue
		}
		if len(discs) == 0 && len(rects) == 0 {
			kept = append(kept, b)
			continue
		}
		covered := 0
		w, h := float64(b.Dx()), float64(b.Dy())
		for i := 0; i < coverageGrid; i++ {
			y := float64(b.Min.Y) + (float64(i)+0.5)*h/coverageGrid
			for j := 0; j < coverageGrid; j++ {
				x := float64(b.Min.X) + (float64(j)+0.5)*w/coverageGrid
				if pointMasked(x, y, discs, rects) {
					covered++
				}
			}
		}
		if float64(covered)/(coverageGrid*coverageGrid) >= frac {
			continue
		}
		kept = append(kept, b)
	}
	return kept
}

func pointMasked(x, y float64, discs []Disc, rects []image.Rectangle) bool {
	for _, d := range discs {
		if math.Hypot(x-d.X, y-d.Y) <= d.R {
			return true
		}
	}
	for _, r := range rects {
		if x >= float64(r.Min.X) && x < float64(r.Max.X) && y >= float64(r.Min.Y) && y < float64(r.Max.Y) {
			return true
		}
	}
	return false
}

// BlobTouchesDisc reports whether the rectangle comes within d.R+pad pixels of
// the disc centre (closest-point distance), i.e. the blob touches or overlaps
// the disc once padded.
func BlobTouchesDisc(r image.Rectangle, d Disc, pad float64) bool {
	if r.Empty() {
		return false
	}
	cx := math.Max(float64(r.Min.X), math.Min(d.X, float64(r.Max.X)))
	cy := math.Max(float64(r.Min.Y), math.Min(d.Y, float64(r.Max.Y)))
	return math.Hypot(d.X-cx, d.Y-cy) <= d.R+pad
}

// MergeWorldBoxes repeatedly merges boxes that overlap or lie within gap
// metres of each other (along both axes) into their union, until none do. The
// result is sorted by (MinX, MinY) so it is deterministic.
func MergeWorldBoxes(boxes []WorldBox, gap float64) []WorldBox {
	out := make([]WorldBox, len(boxes))
	copy(out, boxes)

	for merged := true; merged; {
		merged = false
	scan:
		for i := 0; i < len(out); i++ {
			for j := i + 1; j < len(out); j++ {
				if out[i].Expand(gap).Intersects(out[j]) {
					out[i] = out[i].Union(out[j])
					out = append(out[:j], out[j+1:]...)
					merged = true
					break scan
				}
			}
		}
	}

	sort.Slice(out, func(a, b int) bool {
		if out[a].MinX != out[b].MinX {
			return out[a].MinX < out[b].MinX
		}
		return out[a].MinY < out[b].MinY
	})
	return out
}
