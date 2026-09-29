package detection

import (
	"bytes"
	"image/jpeg"
	"testing"
)

func drawTestPipeline() *DetectionPipeline {
	return NewDetectionPipeline(AprilTagConfig{Family: "tag36h11", QuadDecimate: 2.0})
}

func TestDrawResults_NothingToDrawIsNotDrawn(t *testing.T) {
	p := drawTestPipeline()
	frame := make([]byte, 64*48*3)

	if out, drawn := p.DrawResults(frame, 64, 48, &DetectionResult{}); drawn || out != nil {
		t.Errorf("no tags and no obstacles: drawn=%v len=%d, want not drawn", drawn, len(out))
	}

	p.SetObstacles([]Obstacle{{ID: "was-there", PixelTopLeft: [2]int{5, 5}, PixelBottomRight: [2]int{20, 20}}})
	p.SetObstacles(nil)
	if _, drawn := p.DrawResults(frame, 64, 48, &DetectionResult{}); drawn {
		t.Error("cleared obstacles were still drawn")
	}
}

// A tag with no detector to draw it, and nothing else to draw, is not an error.
func TestDrawResults_NilTagDetectorDoesNotPanic(t *testing.T) {
	p := &DetectionPipeline{obstacleDrawer: NewObstacleDrawer()}
	_, drawn := p.DrawResults(make([]byte, 64*48*3), 64, 48, &DetectionResult{Tags: []AprilTag{{TagID: 1}}})
	if drawn {
		t.Error("drawn = true with nothing that can draw")
	}
}

func TestDrawResults_WithObstaclesReturnsAJPEGOfTheFrameSize(t *testing.T) {
	p := drawTestPipeline()
	p.SetObstacles([]Obstacle{{ID: "box", PixelTopLeft: [2]int{10, 10}, PixelBottomRight: [2]int{40, 30}}})

	out, drawn := p.DrawResults(make([]byte, 64*48*3), 64, 48, &DetectionResult{})
	if !drawn {
		t.Fatal("obstacles were set but nothing was drawn")
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil || img == nil {
		t.Fatalf("output is not a JPEG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 48 {
		t.Errorf("decoded size %dx%d, want 64x48", b.Dx(), b.Dy())
	}
}

// A frame that is not width*height BGR (a wrong-size buffer, or another pixel
// format) must be refused rather than misread.
func TestDrawResults_WrongSizedFrameIsNotDrawn(t *testing.T) {
	p := drawTestPipeline()
	p.SetObstacles([]Obstacle{{ID: "box", PixelTopLeft: [2]int{1, 1}, PixelBottomRight: [2]int{9, 9}}})

	for name, frame := range map[string][]byte{
		"empty":       nil,
		"too short":   make([]byte, 100),
		"extra bytes": make([]byte, 64*48*3+1000),
		"RGBA":        make([]byte, 64*48*4),
	} {
		if _, drawn := p.DrawResults(frame, 64, 48, &DetectionResult{}); drawn {
			t.Errorf("%s: a %d-byte buffer was treated as a 64x48 BGR frame", name, len(frame))
		}
	}
}
