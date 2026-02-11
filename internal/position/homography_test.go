package position

import (
	"math"
	"testing"
)

func TestNewHomography(t *testing.T) {
	h := NewHomography()
	if h == nil {
		t.Fatal("NewHomography returned nil")
	}
	if h.Valid {
		t.Error("NewHomography should not be valid by default")
	}
	if h.PixelsPerMeter != 0 {
		t.Errorf("PixelsPerMeter = %f, want 0", h.PixelsPerMeter)
	}
}

func TestHomography_ComputeFromPoints(t *testing.T) {
	t.Run("too few points", func(t *testing.T) {
		h := NewHomography()
		err := h.ComputeFromPoints(
			[]Point2D{{0, 0}, {1, 0}, {1, 1}},
			[]Point2D{{0, 0}, {100, 0}, {100, 100}},
		)
		if err == nil {
			t.Error("Expected error for fewer than 4 points, got nil")
		}
	})

	t.Run("computes non-zero H matrix", func(t *testing.T) {
		h := NewHomography()
		err := h.ComputeFromPoints(
			[]Point2D{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
			[]Point2D{{0, 0}, {100, 0}, {100, 100}, {0, 100}},
		)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !h.Valid {
			t.Fatal("Expected homography to be valid")
		}
		hasNonZero := false
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				if math.Abs(h.H[i][j]) > 1e-10 {
					hasNonZero = true
				}
			}
		}
		if !hasNonZero {
			t.Errorf("H matrix is all zeros: %v", h.H)
		}
	})

	t.Run("round-trip pixel-to-world transform", func(t *testing.T) {
		// src pixel corners map to dst world corners (e.g., 100x100 pixel tag → 1x1 world unit)
		src := []Point2D{{0, 0}, {100, 0}, {100, 100}, {0, 100}}
		dst := []Point2D{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
		h := NewHomography()
		if err := h.ComputeFromPoints(src, dst); err != nil {
			t.Fatalf("ComputeFromPoints failed: %v", err)
		}
		for i, sp := range src {
			world := h.PixelToWorld(sp)
			if math.Abs(world.X-dst[i].X) > 1e-3 || math.Abs(world.Y-dst[i].Y) > 1e-3 {
				t.Errorf("PixelToWorld(%v) = (%.4f, %.4f), want (%.4f, %.4f)",
					sp, world.X, world.Y, dst[i].X, dst[i].Y)
			}
		}
	})

	t.Run("pixels-per-meter estimated from corners", func(t *testing.T) {
		// 100-pixel tag maps to 0.15m tag: diagonal = 141.4px / 0.2121m ≈ 667 px/m
		halfSize := 0.075
		src := []Point2D{{0, 0}, {100, 0}, {100, 100}, {0, 100}}
		dst := []Point2D{
			{-halfSize, -halfSize},
			{halfSize, -halfSize},
			{halfSize, halfSize},
			{-halfSize, halfSize},
		}
		h := NewHomography()
		if err := h.ComputeFromPoints(src, dst); err != nil {
			t.Fatalf("ComputeFromPoints failed: %v", err)
		}
		if h.PixelsPerMeter <= 0 {
			t.Errorf("PixelsPerMeter = %f, want > 0", h.PixelsPerMeter)
		}
	})
}

func TestHomography_ComputeInverse(t *testing.T) {
	h := NewHomography()
	h.SetFromValues(1, 0, 0, 0, 1, 0, 0, 0, 1)
	h.ComputeInverse()

	expected := 1.0
	if math.Abs(h.HInv[0][0]-expected) > 0.001 ||
		math.Abs(h.HInv[1][1]-expected) > 0.001 ||
		math.Abs(h.HInv[2][2]-expected) > 0.001 {
		t.Errorf("Identity matrix inverse failed")
	}
}

func TestHomography_EstimateScale(t *testing.T) {
	h := NewHomography()
	h.EstimateScale()
	if h.PixelsPerMeter != 100.0 {
		t.Errorf("PixelsPerMeter = %f, want 100.0", h.PixelsPerMeter)
	}
}

func TestHomography_PixelToWorld_Invalid(t *testing.T) {
	h := NewHomography()
	pixel := Point2D{X: 100, Y: 200}
	result := h.PixelToWorld(pixel)

	if result.X != pixel.X || result.Y != pixel.Y {
		t.Errorf("Invalid homography should return original pixel coordinates")
	}
}

func TestHomography_PixelToWorld_Valid(t *testing.T) {
	h := NewHomography()
	h.SetFromValues(1, 0, 0, 0, 1, 0, 0, 0, 1)
	h.Valid = true

	pixel := Point2D{X: 100, Y: 200}
	result := h.PixelToWorld(pixel)

	if math.Abs(result.X-100) > 0.001 || math.Abs(result.Y-200) > 0.001 {
		t.Errorf("PixelToWorld() = (%f, %f), want (100, 200)", result.X, result.Y)
	}
}

func TestHomography_PixelToWorld_ZeroDivision(t *testing.T) {
	h := NewHomography()
	h.SetFromValues(0, 0, 0, 0, 0, 0, 0, 0, 0)
	h.Valid = true

	pixel := Point2D{X: 100, Y: 200}
	result := h.PixelToWorld(pixel)

	if math.Abs(result.X-100) > 0.001 || math.Abs(result.Y-200) > 0.001 {
		t.Errorf("Zero division should return original coordinates")
	}
}

func TestHomography_WorldToPixel_Invalid(t *testing.T) {
	h := NewHomography()
	world := Point2D{X: 1.0, Y: 2.0}
	x, y := h.WorldToPixel(world)

	if x != 1 || y != 2 {
		t.Errorf("Invalid homography should return floored world coordinates, got (%d, %d)", x, y)
	}
}

func TestHomography_WorldToPixel_Valid(t *testing.T) {
	h := NewHomography()
	h.SetFromValues(1, 0, 0, 0, 1, 0, 0, 0, 1)
	h.Valid = true
	h.ComputeInverse()

	world := Point2D{X: 100.5, Y: 200.7}
	x, y := h.WorldToPixel(world)

	if x != 100 || y != 200 {
		t.Errorf("WorldToPixel() = (%d, %d), want (100, 200)", x, y)
	}
}

func TestHomography_SetFromValues(t *testing.T) {
	h := NewHomography()
	h.SetFromValues(1, 2, 3, 4, 5, 6, 7, 8, 9)

	if h.H[0][0] != 1 || h.H[0][1] != 2 || h.H[0][2] != 3 {
		t.Errorf("H[0] = [%f, %f, %f], want [1, 2, 3]", h.H[0][0], h.H[0][1], h.H[0][2])
	}
	if h.H[1][0] != 4 || h.H[1][1] != 5 || h.H[1][2] != 6 {
		t.Errorf("H[1] = [%f, %f, %f], want [4, 5, 6]", h.H[1][0], h.H[1][1], h.H[1][2])
	}
	if h.H[2][0] != 7 || h.H[2][1] != 8 || h.H[2][2] != 9 {
		t.Errorf("H[2] = [%f, %f, %f], want [7, 8, 9]", h.H[2][0], h.H[2][1], h.H[2][2])
	}
	if !h.Valid {
		t.Error("SetFromValues should set Valid to true")
	}
}

func TestHomography_IsValid(t *testing.T) {
	h := NewHomography()
	if h.IsValid() {
		t.Error("NewHomography should not be valid")
	}

	h.Valid = true
	if !h.IsValid() {
		t.Error("Homography with Valid=true should return true")
	}
}

func TestHomography_GetPixelsPerMeter(t *testing.T) {
	h := NewHomography()
	if ppm := h.GetPixelsPerMeter(); ppm != 0 {
		t.Errorf("NewHomography GetPixelsPerMeter = %f, want 0", ppm)
	}

	h.SetFromValues(1, 0, 0, 0, 1, 0, 0, 0, 1)
	if ppm := h.GetPixelsPerMeter(); ppm != 100.0 {
		t.Errorf("After SetFromValues GetPixelsPerMeter = %f, want 100.0", ppm)
	}
}

func TestHomography_SetPixelsPerMeter(t *testing.T) {
	h := NewHomography()
	h.SetPixelsPerMeter(150.0)
	if h.PixelsPerMeter != 150.0 {
		t.Errorf("PixelsPerMeter = %f, want 150.0", h.PixelsPerMeter)
	}
}

func TestHomography_IdentityTransform(t *testing.T) {
	h := NewHomography()
	h.SetFromValues(1, 0, 0, 0, 1, 0, 0, 0, 1)

	pixel := Point2D{X: 100, Y: 200}
	world := h.PixelToWorld(pixel)

	if math.Abs(world.X-100) > 0.001 || math.Abs(world.Y-200) > 0.001 {
		t.Errorf("Identity transform should preserve coordinates")
	}

	world2 := Point2D{X: 1.0, Y: 2.0}
	pixelX, pixelY := h.WorldToPixel(world2)

	if pixelX != 1 || pixelY != 2 {
		t.Errorf("Identity inverse transform should preserve coordinates, got (%d, %d)", pixelX, pixelY)
	}
}

func TestHomography_ScaleTransform(t *testing.T) {
	h := NewHomography()
	h.SetFromValues(2, 0, 0, 0, 2, 0, 0, 0, 1)

	pixel := Point2D{X: 100, Y: 200}
	world := h.PixelToWorld(pixel)

	if math.Abs(world.X-200) > 0.001 || math.Abs(world.Y-400) > 0.001 {
		t.Errorf("Scale transform failed, got (%f, %f), want (200, 400)", world.X, world.Y)
	}
}

func TestHomography_PixelScaleTransform(t *testing.T) {
	h := NewHomography()
	h.SetFromValues(0.5, 0, 0, 0, 0.5, 0, 0, 0, 1)

	pixel := Point2D{X: 100, Y: 200}
	world := h.PixelToWorld(pixel)

	if math.Abs(world.X-50) > 0.001 || math.Abs(world.Y-100) > 0.001 {
		t.Errorf("Pixel scale transform failed, got (%f, %f), want (50, 100)", world.X, world.Y)
	}
}

func TestHomography_RotationTransform(t *testing.T) {
	h := NewHomography()
	cos90 := 0.0
	sin90 := 1.0
	h.SetFromValues(cos90, -sin90, 0, sin90, cos90, 0, 0, 0, 1)

	pixel := Point2D{X: 1, Y: 0}
	world := h.PixelToWorld(pixel)

	if math.Abs(world.X) > 0.001 || math.Abs(world.Y-1) > 0.001 {
		t.Errorf("90-degree rotation failed, got (%f, %f), want (0, 1)", world.X, world.Y)
	}
}
