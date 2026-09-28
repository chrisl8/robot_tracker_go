package detection

import "log"

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

func (p *DetectionPipeline) DrawResults(image []byte, width, height int, result *DetectionResult) []byte {
	output := image

	if len(result.Tags) > 0 && p.tagDetector != nil {
		output = p.tagDetector.DrawTags(output, width, height, result.Tags)
	}

	if len(p.obstacles) > 0 && p.obstacleDrawer != nil {
		output = p.obstacleDrawer.DrawObstacles(output, width, height, p.obstacles)
	} else if len(p.obstacles) > 0 {
		log.Printf("DRAW_RESULTS: obstacleDrawer is nil, skipping obstacles!")
	}

	return output
}
