//go:build gocv

package detection

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"gocv.io/x/gocv"
)

type YOLODetector struct {
	config     *YOLOConfig
	net        gocv.Net
	classNames map[int]string
	loaded     bool
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

	detector := &YOLODetector{
		config:     config,
		classNames: make(map[int]string),
		loaded:     false,
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
		net.SetPreferableBackend(backend)
		net.SetPreferableTarget(gocv.NetTargetCPU)

		detector.net = net
		detector.loaded = true
	}

	return detector, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (d *YOLODetector) Detect(imageBytes []byte, width, height int) []YOLODetection {
	detections := make([]YOLODetection, 0)

	if !d.loaded || len(imageBytes) == 0 {
		return detections
	}

	return detections
}

func (d *YOLODetector) getOutputNames() []string {
	var outputLayers []string
	for _, i := range d.net.GetUnconnectedOutLayers() {
		layer := d.net.GetLayer(i)
		layerName := layer.GetName()
		if layerName != "_input" {
			outputLayers = append(outputLayers, layerName)
		}
	}
	return outputLayers
}

func (d *YOLODetector) performDetection(outs []gocv.Mat) ([]image.Rectangle, []float32, []int) {
	var classIds []int
	var confidences []float32
	var boxes []image.Rectangle

	if len(outs) == 0 || outs[0].Empty() {
		return boxes, confidences, classIds
	}

	tmp := gocv.NewMat()
	gocv.TransposeND(outs[0], []int{0, 2, 1}, &tmp)
	outs[0].Close()
	outs[0] = tmp

	for _, out := range outs {
		if out.Empty() {
			continue
		}

		out = out.Reshape(1, out.Size()[1])

		for i := 0; i < out.Rows(); i++ {
			cols := out.Cols()
			scoresCol := out.RowRange(i, i+1)
			scores := scoresCol.ColRange(4, cols)
			_, confidence, _, classIDPoint := gocv.MinMaxLoc(scores)

			scores.Close()
			scoresCol.Close()

			if confidence > float32(d.config.ConfThres) {
				centerX := out.GetFloatAt(i, 0)
				centerY := out.GetFloatAt(i, 1)
				width := out.GetFloatAt(i, 2)
				height := out.GetFloatAt(i, 3)

				left := centerX - width/2
				top := centerY - height/2
				right := centerX + width/2
				bottom := centerY + height/2

				classIds = append(classIds, classIDPoint.X)
				confidences = append(confidences, float32(confidence))
				boxes = append(boxes, image.Rect(int(left), int(top), int(right), int(bottom)))
			}
		}

		out.Close()
	}

	return boxes, confidences, classIds
}

func (d *YOLODetector) DrawDetections(imgData []byte, width, height int, detections []YOLODetection) []byte {
	if len(imgData) == 0 || len(detections) == 0 {
		return imgData
	}

	img, err := gocv.NewMatFromBytes(height, width, gocv.MatTypeCV8UC3, imgData)
	if err != nil || img.Empty() {
		return imgData
	}
	defer img.Close()

	for _, det := range detections {
		if det.Bbox != nil {
			rect := image.Rect(det.Bbox.X1, det.Bbox.Y1, det.Bbox.X2, det.Bbox.Y2)
			gocv.Rectangle(&img, rect, color.RGBA{0, 255, 0, 0}, 2)
		}
	}

	buf, err := gocv.IMEncode(".png", img)
	if err != nil {
		return imgData
	}

	return buf.GetBytes()
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
		d.net.Close()
		d.loaded = false
	}
	return nil
}
