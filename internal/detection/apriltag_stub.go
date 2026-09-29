//go:build !gocv

package detection

import "image"

type AprilTagDetector struct{}

func NewAprilTagDetector(config AprilTagConfig) (*AprilTagDetector, error) {
	return &AprilTagDetector{}, nil
}

func (d *AprilTagDetector) Detect(image []byte, width, height int) []AprilTag {
	return make([]AprilTag, 0)
}

func (d *AprilTagDetector) DrawTagsOn(dst *image.RGBA, tags []AprilTag) {}

func (d *AprilTagDetector) Close() error {
	return nil
}
