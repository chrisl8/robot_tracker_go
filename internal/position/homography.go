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
