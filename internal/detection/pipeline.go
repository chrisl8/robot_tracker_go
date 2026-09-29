package detection

import (
	"bytes"
	"image"
	"image/jpeg"
)

// NewDetectionPipeline builds a pipeline around an AprilTag detector.
// NewAprilTagDetector never actually returns an error today, but Detect and
// DrawResults still nil-check tagDetector below so that a future constructor
// failure degrades to "no tags detected" instead of a nil-pointer panic.
func NewDetectionPipeline(tagConfig AprilTagConfig) *DetectionPipeline {
	pipeline := &DetectionPipeline{
		obstacleDrawer: NewObstacleDrawer(),
	}

	tagDetector, err := NewAprilTagDetector(tagConfig)
	if err == nil {
		pipeline.tagDetector = tagDetector
	} else {
		pipeline.tagDetector = nil
	}

	return pipeline
}

func (p *DetectionPipeline) Detect(image []byte, width, height int, timestamp float64, frameIdx int) *DetectionResult {
	result := &DetectionResult{
		Timestamp: timestamp,
		FrameIdx:  frameIdx,
	}

	var tags []AprilTag
	if p.tagDetector != nil {
		tags = p.tagDetector.Detect(image, width, height)
	}
	result.Tags = tags

	result.FusedDetections = p.fuseDetections(tags)

	return result
}

func (p *DetectionPipeline) fuseDetections(tags []AprilTag) []FusedDetection {
	fused := make([]FusedDetection, 0, len(tags))

	for _, tag := range tags {
		tagID := tag.TagID
		fused = append(fused, FusedDetection{
			DetectionType: DetectionTypeAprilTag,
			Bbox:          p.tagToBbox(tag),
			TagID:         &tagID,
			Confidence:    1.0,
			Corners:       tag.Corners,
			Source:        "april_tag",
		})
	}

	return fused
}

func (p *DetectionPipeline) tagToBbox(tag AprilTag) *BoundingBox {
	minX, minY := int(tag.Corners[0][0]), int(tag.Corners[0][1])
	maxX, maxY := minX, minY

	for i := 1; i < 4; i++ {
		x, y := int(tag.Corners[i][0]), int(tag.Corners[i][1])
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}

	return &BoundingBox{X1: minX, Y1: minY, X2: maxX, Y2: maxY}
}

// DrawResults renders the detection overlay (tag outlines, the user's
// obstacle boxes) onto a copy of frame, a packed width*height BGR image, and
// returns it as a JPEG. drawn is false when there is nothing to draw, or frame
// is not a BGR image of that size; the caller then shows the plain frame.
func (p *DetectionPipeline) DrawResults(frame []byte, width, height int, result *DetectionResult) (jpegData []byte, drawn bool) {
	drawTags := len(result.Tags) > 0 && p.tagDetector != nil

	p.obstaclesMu.RLock()
	obstacles := p.obstacles
	p.obstaclesMu.RUnlock()
	drawObstacles := len(obstacles) > 0 && p.obstacleDrawer != nil

	if !drawTags && !drawObstacles {
		return nil, false
	}
	if width <= 0 || height <= 0 || len(frame) != width*height*3 {
		return nil, false
	}

	canvas := bgrToRGBA(frame, width, height)
	if drawTags {
		p.tagDetector.DrawTagsOn(canvas, result.Tags)
	}
	if drawObstacles {
		p.obstacleDrawer.DrawObstaclesOn(canvas, obstacles)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, canvas, &jpeg.Options{Quality: 85}); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// bgrToRGBA converts a packed BGR frame (OpenCV's layout) to an RGBA image.
func bgrToRGBA(frame []byte, width, height int) *image.RGBA {
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < width*height; i++ {
		rgba.Pix[i*4] = frame[i*3+2]
		rgba.Pix[i*4+1] = frame[i*3+1]
		rgba.Pix[i*4+2] = frame[i*3]
		rgba.Pix[i*4+3] = 255
	}
	return rgba
}
