//go:build !gocv

package detection

type AprilTagDetector struct{}

func NewAprilTagDetector(config AprilTagConfig) (*AprilTagDetector, error) {
	return &AprilTagDetector{}, nil
}

func (d *AprilTagDetector) Detect(image []byte, width, height int) []AprilTag {
	return make([]AprilTag, 0)
}

func (d *AprilTagDetector) DrawTags(image []byte, width, height int, tags []AprilTag) []byte {
	return image
}

func (d *AprilTagDetector) Close() error {
	return nil
}
