package detection

type AprilTagDetector struct {
	family       string
	quadDecimate float64
}

func NewAprilTagDetector(family string, quadDecimate float64) *AprilTagDetector {
	if family == "" {
		family = "tag36h11"
	}
	if quadDecimate == 0 {
		quadDecimate = 2.0
	}
	return &AprilTagDetector{
		family:       family,
		quadDecimate: quadDecimate,
	}
}

func (d *AprilTagDetector) Detect(image []byte, width, height int) []AprilTag {
	tags := make([]AprilTag, 0)

	_ = image
	_ = width
	_ = height

	return tags
}

func (d *AprilTagDetector) DrawTags(image []byte, width, height int, tags []AprilTag) []byte {
	return image
}
