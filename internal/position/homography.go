package position

import (
	"fmt"
	"os"
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

func (h *Homography) ComputeFromPoints(srcPixels, dstPixels []Point2D) error {
	if len(srcPixels) < 4 || len(dstPixels) < 4 {
		return fmt.Errorf("need at least 4 point pairs")
	}

	numPoints := len(srcPixels)
	A := make([]float64, numPoints*2*9)

	for i := 0; i < numPoints; i++ {
		sp := srcPixels[i]
		dp := dstPixels[i]

		row := i * 2
		idx := row * 9
		A[idx+0] = sp.X
		A[idx+1] = sp.Y
		A[idx+2] = 1
		A[idx+3] = 0
		A[idx+4] = 0
		A[idx+5] = 0
		A[idx+6] = -sp.X * dp.X
		A[idx+7] = -sp.Y * dp.X
		A[idx+8] = dp.X

		idx += 9
		A[idx+0] = 0
		A[idx+1] = 0
		A[idx+2] = 0
		A[idx+3] = sp.X
		A[idx+4] = sp.Y
		A[idx+5] = 1
		A[idx+6] = -sp.X * dp.Y
		A[idx+7] = -sp.Y * dp.Y
		A[idx+8] = dp.Y
	}

	h.EstimateScale()
	h.Valid = true
	return nil
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
	h.EstimateScale()
	h.Valid = true
}

func (h *Homography) Save(filename string) error {
	if !h.Valid {
		return fmt.Errorf("homography not valid")
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if j > 0 {
				fmt.Fprint(file, " ")
			}
			fmt.Fprint(file, h.H[i][j])
		}
		fmt.Fprintln(file)
	}

	fmt.Fprintln(file, h.PixelsPerMeter)

	return nil
}

func (h *Homography) Load(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	var values [9]float64
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if _, err := fmt.Fscan(file, &values[i*3+j]); err != nil {
				return err
			}
		}
	}

	fmt.Fscan(file, &h.PixelsPerMeter)

	h.SetFromValues(values[0], values[1], values[2], values[3], values[4], values[5], values[6], values[7], values[8])

	return nil
}

func (h *Homography) IsValid() bool {
	return h.Valid
}

func (h *Homography) GetPixelsPerMeter() float64 {
	return h.PixelsPerMeter
}

func (h *Homography) SetPixelsPerMeter(ppm float64) {
	h.PixelsPerMeter = ppm
}

func (h *Homography) ComputeFromAprilTag(imgCorners [][2]float64, worldCorners [][3]float64) error {
	if len(imgCorners) < 4 || len(worldCorners) < 4 {
		return fmt.Errorf("need at least 4 corner points")
	}

	A := make([]float64, 8*9)
	rowIdx := 0

	for i := 0; i < 4; i++ {
		ux := imgCorners[i][0]
		uy := imgCorners[i][1]
		WX := worldCorners[i][0]
		WY := worldCorners[i][1]
		WZ := worldCorners[i][2]

		A[rowIdx*9+0] = WX
		A[rowIdx*9+1] = WY
		A[rowIdx*9+2] = WZ
		A[rowIdx*9+3] = 0
		A[rowIdx*9+4] = 0
		A[rowIdx*9+5] = 0
		A[rowIdx*9+6] = -WX * ux
		A[rowIdx*9+7] = -WY * ux
		A[rowIdx*9+8] = -WZ * ux
		rowIdx++

		A[rowIdx*9+0] = 0
		A[rowIdx*9+1] = 0
		A[rowIdx*9+2] = 0
		A[rowIdx*9+3] = WX
		A[rowIdx*9+4] = WY
		A[rowIdx*9+5] = WZ
		A[rowIdx*9+6] = -WX * uy
		A[rowIdx*9+7] = -WY * uy
		A[rowIdx*9+8] = -WZ * uy
		rowIdx++
	}

	h.H = solveDLT(A)
	h.ComputeInverse()

	avgDiag := (dist3D(worldCorners[0], worldCorners[2]) + dist3D(worldCorners[1], worldCorners[3])) / 2.0
	if avgDiag > 0 {
		expectedPixels := avgDiag * 1000
		scale := h.EstimateScaleFromCorners(imgCorners, expectedPixels)
		h.PixelsPerMeter = scale
	} else {
		h.EstimateScale()
	}

	h.Valid = true
	return nil
}

func solveDLT(A []float64) [3][3]float64 {
	var H [3][3]float64

	_, V := eigenDecomposition(A)

	for j := 0; j < 9; j++ {
		H[j/3][j%3] = V[8][j]
	}

	scale := H[2][2]
	if scale != 0 {
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				H[i][j] /= scale
			}
		}
	}

	return H
}

func eigenDecomposition(A []float64) ([]float64, [][]float64) {
	n := 8
	eigenvalues := make([]float64, n)
	eigenvectors := make([][]float64, n)
	for i := 0; i < n; i++ {
		eigenvectors[i] = make([]float64, n)
	}

	for i := 0; i < n; i++ {
		eigenvalues[i] = 1.0
		for j := 0; j < n; j++ {
			eigenvectors[i][j] = A[i*9+j%9]
		}
	}

	return eigenvalues, eigenvectors
}

func dist3D(p1, p2 [3]float64) float64 {
	dx := p2[0] - p1[0]
	dy := p2[1] - p1[1]
	dz := p2[2] - p1[2]
	return sqrt(dx*dx + dy*dy + dz*dz)
}

func (h *Homography) EstimateScaleFromCorners(corners [][2]float64, expectedPixels float64) float64 {
	diag1 := sqrt(pow(corners[0][0]-corners[2][0], 2) + pow(corners[0][1]-corners[2][1], 2))
	diag2 := sqrt(pow(corners[1][0]-corners[3][0], 2) + pow(corners[1][1]-corners[3][1], 2))
	avgDiag := (diag1 + diag2) / 2.0
	if avgDiag > 0 {
		return expectedPixels / avgDiag
	}
	return 1000.0
}

func pow(x, y float64) float64 {
	if y == 0 {
		return 1
	}
	result := 1.0
	for i := 0; i < int(y); i++ {
		result *= x
	}
	return result
}
