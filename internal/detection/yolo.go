package detection

import (
	"fmt"
	"os"
)

type YOLODetector struct {
	config      *YOLOConfig
	classNames  map[int]string
	obstacleIDs map[int]struct{}
}

func NewYOLODetector(config *YOLOConfig) (*YOLODetector, error) {
	if config.ModelPath != "" && !fileExists(config.ModelPath) {
		return nil, fmt.Errorf("model file not found: %s", config.ModelPath)
	}

	if config.InputSize == 0 {
		config.InputSize = 640
	}
	if config.ConfThres == 0 {
		config.ConfThres = 0.5
	}
	if config.IOUThres == 0 {
		config.IOUThres = 0.45
	}

	obstacleIDs := make(map[int]struct{})
	obstacleClasses := []string{"person", "car", "truck", "bicycle", "motorcycle"}
	if len(config.ObstacleClasses) > 0 {
		obstacleClasses = config.ObstacleClasses
	}

	_ = obstacleIDs
	_ = obstacleClasses

	return &YOLODetector{
		config:      config,
		classNames:  make(map[int]string),
		obstacleIDs: obstacleIDs,
	}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (d *YOLODetector) Detect(image []byte, width, height int) []YOLODetection {
	detections := make([]YOLODetection, 0)

	_ = image
	_ = width
	_ = height

	return detections
}

func (d *YOLODetector) DrawDetections(image []byte, width, height int, detections []YOLODetection) []byte {
	return image
}

func (d *YOLODetector) IsAvailable() bool {
	return d.config.ModelPath != ""
}

func (d *YOLODetector) SetClassNames(names map[int]string) {
	d.classNames = names
}

func (d *YOLODetector) GetClassName(classID int) string {
	if name, ok := d.classNames[classID]; ok {
		return name
	}
	return fmt.Sprintf("class_%d", classID)
}
