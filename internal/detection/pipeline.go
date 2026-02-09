package detection

import "log"

func NewDetectionPipeline(yoloConfig *YOLOConfig, tagConfig AprilTagConfig) *DetectionPipeline {
	pipeline := &DetectionPipeline{
		yoloEnabled:    false,
		obstacleDrawer: NewObstacleDrawer(),
	}

	tagDetector, err := NewAprilTagDetector(tagConfig)
	if err == nil {
		pipeline.tagDetector = tagDetector
	} else {
		pipeline.tagDetector = nil
	}

	if yoloConfig != nil && yoloConfig.ModelPath != "" {
		detector, err := NewYOLODetector(yoloConfig)
		if err == nil {
			pipeline.yoloDetector = detector
			pipeline.yoloEnabled = true
		}
	}

	pipeline.setDefaultClassNames()

	return pipeline
}

func (p *DetectionPipeline) setDefaultClassNames() {
	classNames := map[int]string{
		0: "person",
		1: "bicycle",
		2: "car",
		3: "motorcycle",
		5: "bus",
		7: "truck",
	}
	if p.yoloDetector != nil {
		p.yoloDetector.SetClassNames(classNames)
	}
}

func (p *DetectionPipeline) Detect(image []byte, width, height int, timestamp float64, frameIdx int) *DetectionResult {
	result := &DetectionResult{
		Timestamp: timestamp,
		FrameIdx:  frameIdx,
	}

	tags := p.tagDetector.Detect(image, width, height)
	result.Tags = tags

	var yoloDetections []YOLODetection
	if p.yoloEnabled {
		yoloDetections = p.yoloDetector.Detect(image, width, height)
	}
	result.YOLODetections = yoloDetections

	result.FusedDetections = p.fuseDetections(tags, yoloDetections)

	return result
}

func (p *DetectionPipeline) fuseDetections(tags []AprilTag, yoloDetections []YOLODetection) []FusedDetection {
	fused := make([]FusedDetection, 0)

	yoloMatched := make(map[int]bool)

	for _, tag := range tags {
		matchedIdx := p.findMatchingYOLO(tag, yoloDetections)

		var bbox *BoundingBox
		confidence := 1.0

		if matchedIdx >= 0 {
			bbox = yoloDetections[matchedIdx].Bbox
			confidence = yoloDetections[matchedIdx].Confidence
			yoloMatched[matchedIdx] = true
		}

		if bbox == nil {
			bbox = p.tagToBbox(tag)
		}

		detectionType := DetectionTypeAprilTag
		if matchedIdx >= 0 {
			detectionType = DetectionTypeFused
		}

		tagID := tag.TagID
		fused = append(fused, FusedDetection{
			DetectionType: detectionType,
			Bbox:          bbox,
			TagID:         &tagID,
			Confidence:    confidence,
			Corners:       tag.Corners,
			Source:        "april_tag",
		})
	}

	for i, yoloDet := range yoloDetections {
		if !yoloMatched[i] {
			fused = append(fused, FusedDetection{
				DetectionType: DetectionTypeYOLO,
				Bbox:          yoloDet.Bbox,
				Confidence:    yoloDet.Confidence,
				ClassName:     yoloDet.ClassName,
				Source:        "yolo",
			})
		}
	}

	return fused
}

func (p *DetectionPipeline) findMatchingYOLO(tag AprilTag, yoloDetections []YOLODetection) int {
	tagCenterX, tagCenterY := int(tag.CenterX), int(tag.CenterY)

	for i, det := range yoloDetections {
		if det.Bbox != nil && det.Bbox.Contains(tagCenterX, tagCenterY) {
			return i
		}
	}

	return -1
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

	log.Printf("DRAW_RESULTS: Called frame=%dx%d tags=%d yolo=%d stored_obstacles=%d",
		width, height, len(result.Tags), len(result.YOLODetections), len(p.obstacles))

	if len(result.Tags) > 0 {
		log.Printf("DRAW_RESULTS: Drawing %d tags", len(result.Tags))
		output = p.tagDetector.DrawTags(output, width, height, result.Tags)
	}

	if len(result.YOLODetections) > 0 && p.yoloDetector != nil {
		log.Printf("DRAW_RESULTS: Drawing %d YOLO detections", len(result.YOLODetections))
		output = p.yoloDetector.DrawDetections(output, width, height, result.YOLODetections)
	}

	if len(p.obstacles) > 0 && p.obstacleDrawer != nil {
		log.Printf("DRAW_RESULTS: Drawing %d stored obstacles", len(p.obstacles))
		output = p.obstacleDrawer.DrawObstacles(output, width, height, p.obstacles)
	} else if len(p.obstacles) > 0 {
		log.Printf("DRAW_RESULTS: obstacleDrawer is nil, skipping obstacles!")
	} else {
		log.Printf("DRAW_RESULTS: No stored obstacles")
	}

	return output
}

func (p *DetectionPipeline) IsYOLOEnabled() bool {
	return p.yoloEnabled
}
