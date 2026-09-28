//go:build gocv

package detection

import (
	"image"
	"image/color"
	"testing"
)

// drawLine used to integer-divide by dy == 0 when both endpoints were the same
// point (e.g. a zero-size obstacle), which panics.
func TestDrawLine_ZeroLengthDoesNotPanic(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	red := color.RGBA{R: 255, A: 255}

	drawLine(img, image.Pt(10, 10), image.Pt(10, 10), red, 3)

	if img.RGBAAt(10, 10) != red {
		t.Errorf("pixel at (10,10) = %v, want %v", img.RGBAAt(10, 10), red)
	}
}

func TestDrawLine_HorizontalAndVertical(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	tests := []struct {
		name string
		a, b image.Point
		mid  image.Point
	}{
		{"horizontal", image.Pt(2, 10), image.Pt(18, 10), image.Pt(10, 10)},
		{"vertical", image.Pt(10, 2), image.Pt(10, 18), image.Pt(10, 10)},
		{"reversed", image.Pt(18, 10), image.Pt(2, 10), image.Pt(10, 10)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, 20, 20))
			drawLine(img, tt.a, tt.b, red, 3)
			if img.RGBAAt(tt.mid.X, tt.mid.Y) != red {
				t.Errorf("midpoint %v not drawn", tt.mid)
			}
		})
	}
}
