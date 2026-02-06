//go:build gocv

package detection

import (
	"fmt"

	"gocv.io/x/gocv"
)

type AprilTagDetector struct {
	detector  *gocv.ArucoDetector
	family    string
	decimate  float32
	quadSigma float32
}

func NewAprilTagDetector(config AprilTagConfig) (*AprilTagDetector, error) {
	dictCode := gocv.ArucoDictAprilTag_36h11
	switch config.Family {
	case "tag16h5":
		dictCode = gocv.ArucoDictAprilTag_16h5
	case "tag25h9":
		dictCode = gocv.ArucoDictAprilTag_25h9
	case "tag36h10":
		dictCode = gocv.ArucoDictAprilTag_36h10
	case "tag36h11":
		dictCode = gocv.ArucoDictAprilTag_36h11
	default:
		dictCode = gocv.ArucoDictAprilTag_36h11
	}

	dictionary := gocv.GetPredefinedDictionary(dictCode)
	params := gocv.NewArucoDetectorParameters()

	decimate := float32(config.QuadDecimate)
	if decimate == 0 {
		decimate = 2.0
	}
	params.SetAprilTagQuadDecimate(decimate)

	if config.Family != "" {
		config.Family = "tag36h11"
	}

	detector := gocv.NewArucoDetectorWithParams(dictionary, params)

	return &AprilTagDetector{
		detector:  &detector,
		family:    config.Family,
		decimate:  decimate,
		quadSigma: 0.0,
	}, nil
}

func (d *AprilTagDetector) Detect(image []byte, width, height int) []AprilTag {
	tags := make([]AprilTag, 0)

	if len(image) == 0 {
		return tags
	}

	img, err := gocv.NewMatFromBytes(height, width, gocv.MatTypeCV8UC3, image)
	if err != nil || img.Empty() {
		return tags
	}
	defer img.Close()

	markerCorners, markerIds, _ := d.detector.DetectMarkers(img)

	if len(markerIds) == 0 {
		return tags
	}

	for i := 0; i < len(markerIds); i++ {
		corners := markerCorners[i]
		if len(corners) != 4 {
			continue
		}

		var cx, cy float64
		for _, corner := range corners {
			cx += float64(corner.X)
			cy += float64(corner.Y)
		}
		cx /= 4.0
		cy /= 4.0

		size := 0.0
		for j := 0; j < 4; j++ {
			dx := float64(corners[j].X) - cx
			dy := float64(corners[j].Y) - cy
			dist := dx*dx + dy*dy
			if dist > size {
				size = dist
			}
		}
		size = 2.0 * size

		tags = append(tags, AprilTag{
			TagID:  markerIds[i],
			Family: d.family,
			Corners: [4][2]float64{
				{float64(corners[0].X), float64(corners[0].Y)},
				{float64(corners[1].X), float64(corners[1].Y)},
				{float64(corners[2].X), float64(corners[2].Y)},
				{float64(corners[3].X), float64(corners[3].Y)},
			},
			CenterX:  cx,
			CenterY:  cy,
			Size:     size,
			Rotation: 0,
		})
	}

	return tags
}

func (d *AprilTagDetector) DrawTags(image []byte, width, height int, tags []AprilTag) []byte {
	if len(image) == 0 || len(tags) == 0 {
		return image
	}

	img, err := gocv.NewMatFromBytes(height, width, gocv.MatTypeCV8UC3, image)
	if err != nil || img.Empty() {
		return image
	}
	defer img.Close()

	borderColor := gocv.Scalar{Val1: 0, Val2: 255, Val3: 0, Val4: 0}

	markerIds := make([]int, len(tags))
	markerCorners := make([][]gocv.Point2f, len(tags))

	for i, tag := range tags {
		markerIds[i] = tag.TagID
		markerCorners[i] = make([]gocv.Point2f, 4)
		for j := 0; j < 4; j++ {
			markerCorners[i][j] = gocv.Point2f{
				X: float32(tag.Corners[j][0]),
				Y: float32(tag.Corners[j][1]),
			}
		}
	}

	gocv.ArucoDrawDetectedMarkers(img, markerCorners, markerIds, borderColor)

	buf, err := gocv.IMEncode(".png", img)
	if err != nil {
		return image
	}

	return buf.GetBytes()
}

func (d *AprilTagDetector) Close() error {
	if d.detector != nil {
		d.detector.Close()
		d.detector = nil
	}
	return nil
}

func familyFromString(family string) string {
	switch family {
	case "tag16h5", "tag25h9", "tag36h10", "tag36h11":
		return family
	default:
		return "tag36h11"
	}
}

func dictionaryCodeFromFamily(family string) (gocv.ArucoDictionaryCode, error) {
	switch family {
	case "tag16h5":
		return gocv.ArucoDictAprilTag_16h5, nil
	case "tag25h9":
		return gocv.ArucoDictAprilTag_25h9, nil
	case "tag36h10":
		return gocv.ArucoDictAprilTag_36h10, nil
	case "tag36h11":
		return gocv.ArucoDictAprilTag_36h11, nil
	default:
		return 0, fmt.Errorf("unknown AprilTag family: %s", family)
	}
}
