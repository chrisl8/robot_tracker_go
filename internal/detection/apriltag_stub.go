//go:build !gocv

package detection

import "image"

// AprilTagDetector is unavailable without OpenCV; it detects nothing.
type AprilTagDetector struct{}

// NewAprilTagDetector returns a detector that never finds a tag.
func NewAprilTagDetector(config AprilTagConfig) (*AprilTagDetector, error) {
	return &AprilTagDetector{}, nil
}

// Detect returns no tags.
func (d *AprilTagDetector) Detect(image []byte, width, height int) []AprilTag {
	return make([]AprilTag, 0)
}

// DrawTagsOn draws nothing.
func (d *AprilTagDetector) DrawTagsOn(dst *image.RGBA, tags []AprilTag) {}

// Close does nothing.
func (d *AprilTagDetector) Close() error {
	return nil
}
