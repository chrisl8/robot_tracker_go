//go:build gocv

package detection

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func blankRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	return img
}

func isOutlineColor(c color.RGBA) bool { return c.R == 255 && c.G == 107 && c.B == 107 }

func TestDrawObstaclesOn_OutlinesTheBoxAndLeavesTheInside(t *testing.T) {
	img := blankRGBA(200, 160)
	NewObstacleDrawer().DrawObstaclesOn(img, []Obstacle{
		{ID: "box", PixelTopLeft: [2]int{50, 40}, PixelBottomRight: [2]int{150, 120}},
	})

	for _, p := range []image.Point{{50, 80}, {150, 80}, {100, 40}, {100, 120}} {
		if !isOutlineColor(img.RGBAAt(p.X, p.Y)) {
			t.Errorf("no outline at %v: %v", p, img.RGBAAt(p.X, p.Y))
		}
	}
	if isOutlineColor(img.RGBAAt(100, 80)) {
		t.Error("the inside of the box was painted")
	}
	if isOutlineColor(img.RGBAAt(20, 20)) {
		t.Error("something outside the box was painted")
	}
}

func TestDrawObstaclesOn_ClipsAndSkipsOutOfFrameBoxes(t *testing.T) {
	img := blankRGBA(100, 80)
	NewObstacleDrawer().DrawObstaclesOn(img, []Obstacle{
		{ID: "off", PixelTopLeft: [2]int{-100, -100}, PixelBottomRight: [2]int{-50, -50}},
		{ID: "past", PixelTopLeft: [2]int{200, 10}, PixelBottomRight: [2]int{300, 60}},
		{ID: "partly", PixelTopLeft: [2]int{60, 20}, PixelBottomRight: [2]int{500, 60}}, // must not panic
	})

	if !isOutlineColor(img.RGBAAt(60, 40)) {
		t.Error("the visible edge of a partly-visible box was not drawn")
	}
}

// The regression this file exists for: with obstacles set but no tag in view,
// the frame used to go to the drawer as raw BGR, which it could not decode, so
// the obstacles were silently never drawn.
func TestDrawResults_DrawsObstaclesWithoutAnyTags(t *testing.T) {
	p := NewDetectionPipeline(AprilTagConfig{Family: "tag36h11"})
	p.SetObstacles([]Obstacle{{ID: "box", PixelTopLeft: [2]int{20, 10}, PixelBottomRight: [2]int{80, 60}}})

	out, drawn := p.DrawResults(make([]byte, 100*80*3), 100, 80, &DetectionResult{})
	if !drawn {
		t.Fatal("nothing drawn")
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil || img == nil {
		t.Fatalf("output is not a JPEG: %v", err)
	}
	r, g, b, _ := img.At(20, 35).RGBA() // left edge of the box, on a black frame
	if r>>8 < 150 || g>>8 > 160 || b>>8 > 160 {
		t.Errorf("edge pixel = (%d,%d,%d), want the reddish outline", r>>8, g>>8, b>>8)
	}
}

func TestDrawResults_DrawsTagOutlinesAndObstaclesTogether(t *testing.T) {
	p := NewDetectionPipeline(AprilTagConfig{Family: "tag36h11"})
	p.SetObstacles([]Obstacle{{ID: "box", PixelTopLeft: [2]int{5, 5}, PixelBottomRight: [2]int{30, 30}}})
	tag := AprilTag{TagID: 3, CenterX: 70, CenterY: 50, Corners: [4][2]float64{{55, 35}, {85, 35}, {85, 65}, {55, 65}}}

	out, drawn := p.DrawResults(make([]byte, 100*80*3), 100, 80, &DetectionResult{Tags: []AprilTag{tag}})
	if !drawn {
		t.Fatal("nothing drawn")
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil || img == nil {
		t.Fatalf("output is not a JPEG: %v", err)
	}
	if r, g, _, _ := img.At(55, 50).RGBA(); g>>8 < 150 || r>>8 > 100 { // green tag edge
		t.Errorf("no green tag outline at its left edge: r=%d g=%d", r>>8, g>>8)
	}
	if r, _, _, _ := img.At(5, 18).RGBA(); r>>8 < 150 { // red-ish obstacle edge
		t.Errorf("no obstacle outline: r=%d", r>>8)
	}
}
