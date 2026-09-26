//go:build gocv

package detection

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"

	"gocv.io/x/gocv"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

type AprilTagDetector struct {
	detector  *gocv.ArucoDetector
	family    string
	decimate  float32
	quadSigma float32
}

func NewAprilTagDetector(config AprilTagConfig) (*AprilTagDetector, error) {
	var dictCode gocv.ArucoDictionaryCode
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
	params.SetAprilTagQuadSigma(float32(config.QuadSigma))

	// Relax quad-detection defaults for distant/angled tags
	// 0.01 (a ~13 px perimeter) made the carpet texture spawn huge numbers of
	// candidate quads: ~400 ms/frame versus ~19 ms at the default 0.03 on the
	// same frames, with identical detections and corners. 0.03 still finds tags
	// down to a ~10 px edge.
	params.SetMinMarkerPerimeterRate(0.03)
	params.SetPolygonalApproxAccuracyRate(0.08)            // default 0.03; tolerate perspective distortion
	params.SetMinCornerDistanceRate(0.02)                  // default 0.05; tolerate compressed corners
	params.SetAprilTagCriticalRad(30.0 * math.Pi / 180.0) // default 10°; accept steeper angles
	params.SetAprilTagMaxLineFitMse(20.0)                  // default 10.0; tolerate worse line fit from angle
	params.SetAprilTagMinWhiteBlackDiff(3)                 // default 5; accept lower contrast between cells

	// Expand adaptive thresholding to try more window sizes
	params.SetAdaptiveThreshWinSizeMax(53) // default 23; try larger windows for distant tags
	params.SetAdaptiveThreshWinSizeStep(4) // default 10; finer search across window sizes

	// Tolerate lower contrast and more decoding errors
	params.SetMinOtsuStdDev(3.0)                       // default 5.0; accept low-contrast regions
	params.SetMaxErroneousBitsInBorderRate(0.5)         // default 0.35; tolerate perspective-distorted borders
	params.SetErrorCorrectionRate(1.0)                  // default 0.6; maximum error correction
	params.SetPerspectiveRemovePixelPerCell(8)           // default 4; higher resolution bit sampling
	params.SetPerspectiveRemoveIgnoredMarginPerCell(0.2) // default 0.13; ignore more cell margin for blurry tags

	if config.RefineEdges != 0 {
		params.SetCornerRefinementMethod(1) // 1 = CORNER_REFINE_SUBPIX
	}

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
	defer func() { _ = img.Close() }()

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

func (d *AprilTagDetector) DrawTags(imgData []byte, width, height int, tags []AprilTag) []byte {
	if len(imgData) == 0 || len(tags) == 0 {
		return imgData
	}

	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < width*height; i++ {
		b := imgData[i*3]
		g := imgData[i*3+1]
		r := imgData[i*3+2]
		rgba.Pix[i*4] = r
		rgba.Pix[i*4+1] = g
		rgba.Pix[i*4+2] = b
		rgba.Pix[i*4+3] = 255
	}

	borderColor := color.RGBA{0, 255, 0, 255}
	labelColor := color.RGBA{0, 0, 0, 255}
	bgColor := color.RGBA{0, 255, 0, 200}

	for _, tag := range tags {
		points := make([]image.Point, 4)
		for j := 0; j < 4; j++ {
			points[j] = image.Point{
				X: int(tag.Corners[j][0]),
				Y: int(tag.Corners[j][1]),
			}
		}

		lineWidth := 3
		for j := 0; j < 4; j++ {
			p1 := points[j]
			p2 := points[(j+1)%4]
			drawLine(rgba, p1, p2, borderColor, lineWidth)
		}

		cx := int(tag.CenterX)
		cy := int(tag.CenterY)

		label := string(rune('0' + tag.TagID%10))
		if tag.TagID >= 10 {
			label = "T"
		}

		drawLabel(rgba, cx, cy-25, label, labelColor, bgColor)
	}

	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, rgba, &jpeg.Options{Quality: 85}); err != nil {
		return imgData
	}

	return buf.Bytes()
}

func drawLine(img *image.RGBA, p1, p2 image.Point, c color.RGBA, width int) {
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y

	if utils.Abs(dx) > utils.Abs(dy) {
		if p1.X > p2.X {
			p1, p2 = p2, p1
		}
		for x := p1.X; x <= p2.X; x++ {
			y := p1.Y + dy*(x-p1.X)/dx
			drawCircle(img, x, y, width/2, c)
		}
	} else {
		if p1.Y > p2.Y {
			p1, p2 = p2, p1
		}
		for y := p1.Y; y <= p2.Y; y++ {
			x := p1.X + dx*(y-p1.Y)/dy
			drawCircle(img, x, y, width/2, c)
		}
	}
}

func drawCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				x := cx + dx
				y := cy + dy
				if x >= 0 && x < img.Rect.Max.X && y >= 0 && y < img.Rect.Max.Y {
					img.Set(x, y, c)
				}
			}
		}
	}
}

func drawLabel(img *image.RGBA, x, y int, text string, textColor, bgColor color.RGBA) {
	fontSize := 20
	boxWidth := fontSize * len(text)
	boxHeight := fontSize

	boxRect := image.Rect(x-boxWidth/2, y, x+boxWidth/2, y+boxHeight)
	draw.Draw(img, boxRect, &image.Uniform{bgColor}, image.Point{}, draw.Src)

	halfWidth := fontSize / 2
	centerX := x - halfWidth + fontSize/4
	centerY := y + fontSize - 2

	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx != 0 || dy != 0 {
				px := centerX + dx
				py := centerY + dy
				if px >= 0 && px < img.Rect.Max.X && py >= 0 && py < img.Rect.Max.Y {
					img.Set(px, py, textColor)
				}
			}
		}
	}

	centerColor := color.RGBA{255, 255, 255, 255}
	for i := 0; i < len(text); i++ {
		px := x - halfWidth + i*fontSize + fontSize/2
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				if px+dx >= 0 && px+dx < img.Rect.Max.X && centerY+dy >= 0 && centerY+dy < img.Rect.Max.Y {
					img.Set(px+dx, centerY+dy, centerColor)
				}
			}
		}
	}
}

func (d *AprilTagDetector) Close() error {
	if d.detector != nil {
		_ = d.detector.Close()
		d.detector = nil
	}
	return nil
}
