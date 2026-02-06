//go:build !gocv

package detection

import (
	"fmt"
)

type YOLODetector struct {
	config     interface{}
	classNames map[int]string
}

func NewYOLODetector(config *YOLOConfig) (*YOLODetector, error) {
	return &YOLODetector{
		config:     nil,
		classNames: make(map[int]string),
	}, nil
}

func (d *YOLODetector) Detect(image []byte, width, height int) []YOLODetection {
	return make([]YOLODetection, 0)
}

func (d *YOLODetector) DrawDetections(image []byte, width, height int, detections []YOLODetection) []byte {
	return image
}

func (d *YOLODetector) IsAvailable() bool {
	return false
}

func (d *YOLODetector) SetClassNames(names map[int]string) {
	d.classNames = make(map[int]string)
	for k, v := range names {
		d.classNames[k] = v
	}
}

func (d *YOLODetector) GetClassName(classID int) string {
	if name, ok := d.classNames[classID]; ok {
		return name
	}
	return fmt.Sprintf("class_%d", classID)
}
