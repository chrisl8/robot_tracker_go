//go:build gocv

package detection

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"

	"gocv.io/x/gocv"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

type YOLODetector struct {
	config     *YOLOConfig
	net        gocv.Net
	classNames map[int]string
	loaded     bool
}

func NewYOLODetector(config *YOLOConfig) (*YOLODetector, error) {
	if config.ModelPath != "" && !utils.FileExists(config.ModelPath) {
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

	detector := &YOLODetector{
		config:     config,
		classNames: make(map[int]string),
		loaded:     false,
	}

	if detector.config.MinObstacleSize <= 0 {
		detector.config.MinObstacleSize = 0.05
	}

	if detector.config.RelevantClasses == nil {
		detector.config.RelevantClasses = map[int]string{
			0:  "person",
			27: "backpack",
			28: "umbrella",
			31: "handbag",
			39: "cup",
			44: "bowl",
			52: "potted plant",
			56: "chair",
			60: "dining table",
			62: "laptop",
			65: "keyboard",
			66: "cell phone",
		}
	}

	if config.ModelPath != "" {
		net := gocv.ReadNetFromONNX(config.ModelPath)
		if net.Empty() {
			return nil, fmt.Errorf("failed to load ONNX model: %s", config.ModelPath)
		}

		backend := gocv.NetBackendDefault
		if config.Device != "" {
			backend = gocv.ParseNetBackend(config.Device)
		}
		_ = net.SetPreferableBackend(backend)
		_ = net.SetPreferableTarget(gocv.NetTargetCPU)

		detector.net = net
		detector.loaded = true
	}

	return detector, nil
}

func (d *YOLODetector) Detect(imageBytes []byte, width, height int) []YOLODetection {
	detections := make([]YOLODetection, 0)

	if !d.loaded || len(imageBytes) == 0 {
		return detections
	}

	blob, err := d.preprocessImage(imageBytes, width, height)
	if err != nil {
		return detections
	}
	defer func() { _ = blob.Close() }()

	d.net.SetInput(blob, "")
	out := d.net.Forward("")

	boxes, confidences, classIDs := d.performDetection(out)
	_ = out.Close()

	if len(boxes) == 0 {
		return detections
	}

	selectedIndices := d.nonMaxSuppression(boxes, confidences, d.config.IOUThres)

	scaleX := float64(width) / float64(d.config.InputSize)
	scaleY := float64(height) / float64(d.config.InputSize)

	for _, idx := range selectedIndices {
		classID := classIDs[idx]

		if d.config.RelevantClasses != nil {
			if _, ok := d.config.RelevantClasses[classID]; !ok {
				continue
			}
		}

		box := boxes[idx]
		scaledBox := BoundingBox{
			X1: int(float64(box.Min.X) * scaleX),
			Y1: int(float64(box.Min.Y) * scaleY),
			X2: int(float64(box.Max.X) * scaleX),
			Y2: int(float64(box.Max.Y) * scaleY),
		}

		boxWidth := scaledBox.X2 - scaledBox.X1
		worldWidth := float64(boxWidth) / d.config.PixelsPerMeter

		if d.config.MinObstacleSize > 0 && worldWidth < d.config.MinObstacleSize {
			continue
		}

		className := d.GetClassName(classID)
		detection := YOLODetection{
			Bbox:       &scaledBox,
			Confidence: float64(confidences[idx]),
			ClassID:    classID,
			ClassName:  className,
		}
		detections = append(detections, detection)
	}

	return detections
}

func (d *YOLODetector) preprocessImage(imageBytes []byte, width, height int) (gocv.Mat, error) {
	img, err := gocv.NewMatFromBytes(height, width, gocv.MatTypeCV8UC3, imageBytes)
	if err != nil || img.Empty() {
		return gocv.Mat{}, fmt.Errorf("failed to create image from bytes")
	}
	defer func() { _ = img.Close() }()

	resized := gocv.NewMat()
	_ = gocv.Resize(img, &resized, image.Point{d.config.InputSize, d.config.InputSize}, 0, 0, gocv.InterpolationArea)
	defer func() { _ = resized.Close() }()

	blob := gocv.BlobFromImage(resized, 1.0/255.0, image.Point{d.config.InputSize, d.config.InputSize}, gocv.Scalar{}, true, false)

	return blob, nil
}

func (d *YOLODetector) nonMaxSuppression(
	boxes []image.Rectangle,
	scores []float32,
	iouThreshold float64,
) []int {
	if len(boxes) == 0 {
		return nil
	}

	var selected []int

	for i := 0; i < len(boxes); i++ {
		keep := true
		for _, sel := range selected {
			iou := boxIoU(boxes[i], boxes[sel])
			if iou >= iouThreshold {
				keep = false
				break
			}
		}
		if keep {
			selected = append(selected, i)
		}
	}

	return selected
}

func boxIoU(a, b image.Rectangle) float64 {
	interX1 := maxInt(a.Min.X, b.Min.X)
	interY1 := maxInt(a.Min.Y, b.Min.Y)
	interX2 := minInt(a.Max.X, b.Max.X)
	interY2 := minInt(a.Max.Y, b.Max.Y)

	if interX2 <= interX1 || interY2 <= interY1 {
		return 0
	}

	interArea := (interX2 - interX1) * (interY2 - interY1)

	areaA := (a.Max.X - a.Min.X) * (a.Max.Y - a.Min.Y)
	areaB := (b.Max.X - b.Min.X) * (b.Max.Y - b.Min.Y)

	unionArea := float64(areaA + areaB - interArea)

	if unionArea <= 0 {
		return 0
	}

	return float64(interArea) / unionArea
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (d *YOLODetector) performDetection(out gocv.Mat) ([]image.Rectangle, []float32, []int) {
	var classIds []int
	var confidences []float32
	var boxes []image.Rectangle

	if out.Empty() {
		return boxes, confidences, classIds
	}

	tmp := gocv.NewMat()
	_ = gocv.TransposeND(out, []int{0, 2, 1}, &tmp)

	reshaped := tmp.Reshape(1, tmp.Size()[1])

	for i := 0; i < reshaped.Rows(); i++ {
		row := reshaped.RowRange(i, i+1)
		scoresCol := row.ColRange(4, reshaped.Cols())
		_, confidence, _, classIDPoint := gocv.MinMaxLoc(scoresCol)
		_ = scoresCol.Close()
		_ = row.Close()

		if confidence > float32(d.config.ConfThres) {
			centerX := reshaped.GetFloatAt(i, 0)
			centerY := reshaped.GetFloatAt(i, 1)
			width := reshaped.GetFloatAt(i, 2)
			height := reshaped.GetFloatAt(i, 3)

			left := centerX - width/2
			top := centerY - height/2
			right := centerX + width/2
			bottom := centerY + height/2

			classIds = append(classIds, classIDPoint.X)
			confidences = append(confidences, float32(confidence))
			boxes = append(boxes, image.Rect(int(left), int(top), int(right), int(bottom)))
		}
	}

	_ = reshaped.Close()
	_ = tmp.Close()

	return boxes, confidences, classIds
}

func (d *YOLODetector) DrawDetections(imgData []byte, width, height int, detections []YOLODetection) []byte {
	if len(imgData) == 0 || len(detections) == 0 {
		return imgData
	}

	reader := bytes.NewReader(imgData)
	img, _, err := image.Decode(reader)
	if err != nil {
		return imgData
	}

	rgba, ok := img.(*image.RGBA)
	if !ok {
		b := img.Bounds()
		newImg := image.NewRGBA(b)
		draw.Draw(newImg, b, img, b.Min, draw.Src)
		rgba = newImg
	}

	borderColor := color.RGBA{0, 255, 0, 255}

	for _, det := range detections {
		if det.Bbox != nil {
			drawYOLORectangle(rgba, det.Bbox.X1, det.Bbox.Y1, det.Bbox.X2, det.Bbox.Y2, borderColor, 3)
		}
	}

	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, rgba, &jpeg.Options{Quality: 85}); err != nil {
		return imgData
	}

	return buf.Bytes()
}

func drawYOLORectangle(img *image.RGBA, x1, y1, x2, y2 int, c color.RGBA, width int) {
	drawLine(img, image.Point{x1, y1}, image.Point{x2, y1}, c, width)
	drawLine(img, image.Point{x2, y1}, image.Point{x2, y2}, c, width)
	drawLine(img, image.Point{x2, y2}, image.Point{x1, y2}, c, width)
	drawLine(img, image.Point{x1, y2}, image.Point{x1, y1}, c, width)
}

func (d *YOLODetector) IsAvailable() bool {
	return d.loaded
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

func (d *YOLODetector) Close() error {
	if d.loaded {
		_ = d.net.Close()
		d.loaded = false
	}
	return nil
}
