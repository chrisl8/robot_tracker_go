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
	tests := []struct {
		name        string
		srcPixels   []Point2D
		dstPixels   []Point2D
		expectError bool
	}{
		{
			name:        "too few points",
			srcPixels:   []Point2D{{0, 0}, {1, 0}, {1, 1}},
			dstPixels:   []Point2D{{0, 0}, {100, 0}, {100, 100}},
			expectError: true,
		},
		{
			name:        "exactly 4 points",
			srcPixels:   []Point2D{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
			dstPixels:   []Point2D{{0, 0}, {100, 0}, {100, 100}, {0, 100}},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHomography()
			err := h.ComputeFromPoints(tt.srcPixels, tt.dstPixels)
			if tt.expectError && err == nil {
				t.Errorf("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
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
