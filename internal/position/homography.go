package position

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/mat"
)

type Homography struct {
	H              [3][3]float64
	HInv           [3][3]float64
	PixelsPerMeter float64
	Valid          bool
}

func NewHomography() *Homography {
	return &Homography{
		Valid: false,
	}
}

// ComputeFromPoints fits the homography mapping srcPixels to dstPixels by a
// Hartley-normalised DLT least-squares solve. It accepts any N >= 4 point
// pairs and returns an error (leaving h untouched) when the points are
// degenerate, e.g. coincident or collinear.
func (h *Homography) ComputeFromPoints(srcPixels, dstPixels []Point2D) error {
	if len(srcPixels) < 4 || len(dstPixels) < 4 {
		return fmt.Errorf("need at least 4 point pairs")
	}
	if len(srcPixels) != len(dstPixels) {
		return fmt.Errorf("mismatched point counts: %d source, %d destination", len(srcPixels), len(dstPixels))
	}

	tSrc, err := newNormalization(srcPixels)
	if err != nil {
		return fmt.Errorf("source points: %w", err)
	}
	tDst, err := newNormalization(dstPixels)
	if err != nil {
		return fmt.Errorf("destination points: %w", err)
	}

	numPoints := len(srcPixels)
	A := make([]float64, numPoints*2*9)

	for i := 0; i < numPoints; i++ {
		sx, sy := tSrc.apply(srcPixels[i])
		dx, dy := tDst.apply(dstPixels[i])

		idx := i * 2 * 9
		A[idx+0] = sx
		A[idx+1] = sy
		A[idx+2] = 1
		A[idx+6] = -sx * dx
		A[idx+7] = -sy * dx
		A[idx+8] = -dx

		idx += 9
		A[idx+3] = sx
		A[idx+4] = sy
		A[idx+5] = 1
		A[idx+6] = -sx * dy
		A[idx+7] = -sy * dy
		A[idx+8] = -dy
	}

	hn, err := solveDLT(A)
	if err != nil {
		return fmt.Errorf("solving homography: %w", err)
	}

	full := mul3(tDst.inverse(), mul3(hn, tSrc.matrix()))
	if math.Abs(full[2][2]) > 1e-12 {
		scale := full[2][2]
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				full[i][j] /= scale
			}
		}
	}

	h.H = full
	h.ComputeInverse()
	h.PixelsPerMeter = h.meanPixelsPerMeter(srcPixels)
	h.Valid = true
	return nil
}

// ReprojectionError maps each src point through the homography and compares
// it with the matching dst point (world metres), returning RMS and maximum
// error and the per-point error, all in centimetres.
func (h *Homography) ReprojectionError(src, dst []Point2D) (rmsCm, maxCm float64, perPointCm []float64) {
	if !h.Valid || len(src) != len(dst) || len(src) == 0 {
		return 0, 0, nil
	}

	perPointCm = make([]float64, len(src))
	sumSq := 0.0
	for i := range src {
		w := h.PixelToWorld(src[i])
		errCm := math.Hypot(w.X-dst[i].X, w.Y-dst[i].Y) * 100
		perPointCm[i] = errCm
		sumSq += errCm * errCm
		if errCm > maxCm {
			maxCm = errCm
		}
	}
	return math.Sqrt(sumSq / float64(len(src))), maxCm, perPointCm
}

// meanPixelsPerMeter averages the local pixel-to-world scale (from the
// homography's Jacobian) over the given pixel points.
func (h *Homography) meanPixelsPerMeter(pixels []Point2D) float64 {
	sum, n := 0.0, 0
	for _, p := range pixels {
		w := h.H[2][0]*p.X + h.H[2][1]*p.Y + h.H[2][2]
		if math.Abs(w) < 1e-12 {
			continue
		}
		x := h.H[0][0]*p.X + h.H[0][1]*p.Y + h.H[0][2]
		y := h.H[1][0]*p.X + h.H[1][1]*p.Y + h.H[1][2]
		w2 := w * w
		a := (h.H[0][0]*w - x*h.H[2][0]) / w2
		b := (h.H[0][1]*w - x*h.H[2][1]) / w2
		c := (h.H[1][0]*w - y*h.H[2][0]) / w2
		d := (h.H[1][1]*w - y*h.H[2][1]) / w2
		det := math.Abs(a*d - b*c)
		if det < 1e-30 {
			continue
		}
		sum += 1 / math.Sqrt(det)
		n++
	}
	if n == 0 {
		return 100.0
	}
	return sum / float64(n)
}

// normalization is the Hartley conditioning transform for a point set:
// translate the centroid to the origin and scale so the mean distance from it
// is sqrt(2).
type normalization struct {
	scale, cx, cy float64
}

func newNormalization(pts []Point2D) (normalization, error) {
	n := float64(len(pts))
	var cx, cy float64
	for _, p := range pts {
		cx += p.X
		cy += p.Y
	}
	cx /= n
	cy /= n

	var meanDist, sxx, sxy, syy float64
	for _, p := range pts {
		dx, dy := p.X-cx, p.Y-cy
		meanDist += math.Hypot(dx, dy)
		sxx += dx * dx
		sxy += dx * dy
		syy += dy * dy
	}
	meanDist /= n
	if meanDist < 1e-12 {
		return normalization{}, fmt.Errorf("points coincide")
	}

	// Eigenvalues of the 2x2 covariance: a near-zero minimum means collinear.
	tr := sxx + syy
	disc := math.Sqrt(math.Max(0, (sxx-syy)*(sxx-syy)/4+sxy*sxy))
	lMax, lMin := tr/2+disc, tr/2-disc
	if lMin <= 1e-9*lMax {
		return normalization{}, fmt.Errorf("points are collinear")
	}

	return normalization{scale: math.Sqrt2 / meanDist, cx: cx, cy: cy}, nil
}

func (t normalization) apply(p Point2D) (float64, float64) {
	return (p.X - t.cx) * t.scale, (p.Y - t.cy) * t.scale
}

func (t normalization) matrix() [3][3]float64 {
	return [3][3]float64{
		{t.scale, 0, -t.scale * t.cx},
		{0, t.scale, -t.scale * t.cy},
		{0, 0, 1},
	}
}

func (t normalization) inverse() [3][3]float64 {
	return [3][3]float64{
		{1 / t.scale, 0, t.cx},
		{0, 1 / t.scale, t.cy},
		{0, 0, 1},
	}
}

func mul3(a, b [3][3]float64) [3][3]float64 {
	var out [3][3]float64
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				out[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return out
}

func (h *Homography) ComputeInverse() {
	det := h.H[0][0]*h.H[1][1]*h.H[2][2] +
		h.H[0][1]*h.H[1][2]*h.H[2][0] +
		h.H[0][2]*h.H[1][0]*h.H[2][1] -
		h.H[0][2]*h.H[1][1]*h.H[2][0] -
		h.H[0][1]*h.H[1][0]*h.H[2][2] -
		h.H[0][0]*h.H[1][2]*h.H[2][1]

	if det == 0 {
		return
	}

	invDet := 1.0 / det

	h.HInv[0][0] = (h.H[1][1]*h.H[2][2] - h.H[1][2]*h.H[2][1]) * invDet
	h.HInv[0][1] = (h.H[0][2]*h.H[2][1] - h.H[0][1]*h.H[2][2]) * invDet
	h.HInv[0][2] = (h.H[0][1]*h.H[1][2] - h.H[0][2]*h.H[1][1]) * invDet
	h.HInv[1][0] = (h.H[1][2]*h.H[2][0] - h.H[1][0]*h.H[2][2]) * invDet
	h.HInv[1][1] = (h.H[0][0]*h.H[2][2] - h.H[0][2]*h.H[2][0]) * invDet
	h.HInv[1][2] = (h.H[0][2]*h.H[1][0] - h.H[0][0]*h.H[1][2]) * invDet
	h.HInv[2][0] = (h.H[1][0]*h.H[2][1] - h.H[1][1]*h.H[2][0]) * invDet
	h.HInv[2][1] = (h.H[0][1]*h.H[2][0] - h.H[0][0]*h.H[2][1]) * invDet
	h.HInv[2][2] = (h.H[0][0]*h.H[1][1] - h.H[0][1]*h.H[1][0]) * invDet
}

// EstimateScale sets PixelsPerMeter to a generic default (100.0). It is a
// last-resort fallback for callers with no way to derive a real scale (e.g.
// degenerate calibration input, or a calibration file with no world_scale) —
// it is never invoked implicitly by SetFromValues, so it must be called
// explicitly wherever that fallback is actually wanted.
func (h *Homography) EstimateScale() {
	h.PixelsPerMeter = 100.0
}

func (h *Homography) PixelToWorld(pixel Point2D) *Point2D {
	if !h.Valid {
		return &Point2D{pixel.X, pixel.Y}
	}

	src := []float64{pixel.X, pixel.Y, 1}

	dst := make([]float64, 3)
	for i := 0; i < 3; i++ {
		dst[i] = h.H[i][0]*src[0] + h.H[i][1]*src[1] + h.H[i][2]*src[2]
	}

	if dst[2] == 0 {
		return &Point2D{pixel.X, pixel.Y}
	}

	return &Point2D{
		X: dst[0] / dst[2],
		Y: dst[1] / dst[2],
	}
}

func (h *Homography) WorldToPixel(world Point2D) (int, int) {
	if !h.Valid {
		return int(world.X), int(world.Y)
	}

	src := []float64{world.X, world.Y, 1}

	dst := make([]float64, 3)
	for i := 0; i < 3; i++ {
		dst[i] = h.HInv[i][0]*src[0] + h.HInv[i][1]*src[1] + h.HInv[i][2]*src[2]
	}

	if dst[2] == 0 {
		return int(world.X), int(world.Y)
	}

	return int(dst[0] / dst[2]), int(dst[1] / dst[2])
}

// SetFromValues loads a raw 3x3 homography matrix (row-major) and marks it
// valid. It intentionally leaves PixelsPerMeter untouched: this is a plain
// matrix setter, not a calibration step, so it must not silently overwrite a
// scale a caller already computed or loaded. Callers that need a scale set it
// explicitly via SetPixelsPerMeter (or EstimateScale as an explicit fallback).
func (h *Homography) SetFromValues(h0, h1, h2, h3, h4, h5, h6, h7, h8 float64) {
	h.H[0][0] = h0
	h.H[0][1] = h1
	h.H[0][2] = h2
	h.H[1][0] = h3
	h.H[1][1] = h4
	h.H[1][2] = h5
	h.H[2][0] = h6
	h.H[2][1] = h7
	h.H[2][2] = h8
	h.ComputeInverse()
	h.Valid = true
}

func (h *Homography) IsValid() bool {
	return h.Valid
}

func (h *Homography) HasTransformation() bool {
	if !h.Valid {
		return false
	}
	h0n0 := h.H[0][0]*h.H[0][0] + h.H[0][1]*h.H[0][1] + h.H[0][2]*h.H[0][2]
	h1n0 := h.H[1][0]*h.H[1][0] + h.H[1][1]*h.H[1][1] + h.H[1][2]*h.H[1][2]
	return h0n0 > 0.001 || h1n0 > 0.001
}

func (h *Homography) SetIdentity() {
	h.H[0][0] = 1.0
	h.H[0][1] = 0.0
	h.H[0][2] = 0.0
	h.H[1][0] = 0.0
	h.H[1][1] = 1.0
	h.H[1][2] = 0.0
	h.H[2][0] = 0.0
	h.H[2][1] = 0.0
	h.H[2][2] = 1.0
	h.Valid = true
	h.ComputeInverse()
	h.PixelsPerMeter = 100.0
}

func (h *Homography) SetPixelsPerMeter(ppm float64) {
	h.PixelsPerMeter = ppm
}

// solveDLT returns the null-space solution of the (rows x 9) DLT matrix a,
// reshaped to a 3x3 homography with H[2][2] normalised to 1 when possible.
func solveDLT(a []float64) ([3][3]float64, error) {
	var H [3][3]float64

	if len(a) == 0 || len(a)%9 != 0 {
		return H, fmt.Errorf("DLT matrix has %d entries, want a multiple of 9", len(a))
	}

	A := mat.NewDense(len(a)/9, 9, a)
	var SVD mat.SVD
	if ok := SVD.Factorize(A, mat.SVDFullV); !ok {
		return H, fmt.Errorf("SVD factorization failed")
	}

	var V mat.Dense
	SVD.VTo(&V)

	// Singular values are in descending order, so the last column of the 9x9 V
	// is the right singular vector for the smallest singular value: the
	// least-squares (null-space) homography solution.
	v := make([]float64, 9)
	for j := 0; j < 9; j++ {
		v[j] = V.At(j, 8)
	}

	norm := 0.0
	for i := 0; i < 9; i++ {
		norm += v[i] * v[i]
	}
	if norm == 0 {
		return H, fmt.Errorf("degenerate DLT solution")
	}
	for i := 0; i < 9; i++ {
		v[i] /= math.Sqrt(norm)
	}

	for j := 0; j < 9; j++ {
		H[j/3][j%3] = v[j]
	}

	scale := H[2][2]
	if math.Abs(scale) > 1e-10 {
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				H[i][j] /= scale
			}
		}
	}

	return H, nil
}
