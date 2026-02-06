package position

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type CalibrationConfig struct {
	Version      int              `yaml:"version"`
	Camera       CameraInfo       `yaml:"camera"`
	Intrinsics   CameraIntrinsics `yaml:"intrinsics"`
	Homography   [][]float64      `yaml:"homography"`
	WorldScale   float64          `yaml:"world_scale"`
	TagSize      float64          `yaml:"tag_size"`
	CalibratedAt string           `yaml:"calibrated_at"`
}

type CameraInfo struct {
	Name       string `yaml:"name"`
	Resolution [2]int `yaml:"resolution"`
}

type CalibrationState int

const (
	CalibrationNotCalibrated CalibrationState = iota
	CalibrationDetecting
	CalibrationComputing
	CalibrationComplete
)

type CalibrationResult struct {
	State          CalibrationState `json:"state"`
	TagDetected    bool             `json:"tagDetected"`
	TagID          int              `json:"tagId,omitempty"`
	TagCorners     [4][2]float64    `json:"tagCorners,omitempty"`
	TagSize        float64          `json:"tagSize"`
	ComputedWidth  float64          `json:"computedWidth"`
	ComputedHeight float64          `json:"computedHeight"`
	PixelsPerMeter float64          `json:"pixelsPerMeter"`
	Message        string           `json:"message"`
	Error          string           `json:"error,omitempty"`
}

type CalibrationManager struct {
	result     *CalibrationResult
	tagSize    float64
	cameraName string
	cameraW    int
	cameraH    int
}

func NewCalibrationManager(cameraName string, cameraW, cameraH int) *CalibrationManager {
	return &CalibrationManager{
		result: &CalibrationResult{
			State: CalibrationNotCalibrated,
		},
		cameraName: cameraName,
		cameraW:    cameraW,
		cameraH:    cameraH,
	}
}

func sanitizeCameraName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := re.ReplaceAllString(name, "_")
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		sanitized = "unknown"
	}
	return sanitized
}

func GetCalibrationFilename(cameraName string) string {
	sanitized := sanitizeCameraName(cameraName)
	return fmt.Sprintf("config/calibration_%s.yaml", sanitized)
}

func (cm *CalibrationManager) GetFilename() string {
	return GetCalibrationFilename(cm.cameraName)
}

func (cm *CalibrationManager) Start(tagSize float64) {
	cm.tagSize = tagSize
	cm.result = &CalibrationResult{
		State:   CalibrationDetecting,
		TagSize: tagSize,
		Message: "Place an AprilTag in the camera view",
	}
}

func (cm *CalibrationManager) ComputeHomography(tagID int, corners [4][2]float64) *CalibrationResult {
	cm.result.State = CalibrationComputing
	cm.result.TagDetected = true
	cm.result.TagID = tagID
	cm.result.TagCorners = corners

	halfSize := cm.tagSize / 2.0

	worldCorners := [][3]float64{
		{-halfSize, -halfSize, 0},
		{halfSize, -halfSize, 0},
		{halfSize, halfSize, 0},
		{-halfSize, halfSize, 0},
	}

	imgCorners := make([][2]float64, 4)
	for i := 0; i < 4; i++ {
		imgCorners[i] = corners[i]
	}

	h := NewHomography()
	err := h.ComputeFromAprilTag(imgCorners, worldCorners)
	if err != nil {
		cm.result.State = CalibrationNotCalibrated
		cm.result.Error = fmt.Sprintf("Failed to compute homography: %v", err)
		return cm.result
	}

	cm.result.PixelsPerMeter = h.GetPixelsPerMeter()

	cm.ComputeDimensions(corners, h)

	cm.result.State = CalibrationComplete
	cm.result.Message = fmt.Sprintf("Calibration complete! Area: %.2fm x %.2fm",
		cm.result.ComputedWidth, cm.result.ComputedHeight)

	return cm.result
}

func (cm *CalibrationManager) ComputeDimensions(corners [4][2]float64, h *Homography) {
	_ = corners

	topLeft := h.PixelToWorld(Point2D{X: corners[0][0], Y: corners[0][1]})
	bottomRight := h.PixelToWorld(Point2D{X: corners[2][0], Y: corners[2][1]})

	width := bottomRight.X - topLeft.X
	if width < 0 {
		width = -width
	}
	height := bottomRight.Y - topLeft.Y
	if height < 0 {
		height = -height
	}

	cm.result.ComputedWidth = width
	cm.result.ComputedHeight = height
}

func (cm *CalibrationManager) SaveToFile(filename string) error {
	if cm.result.State != CalibrationComplete {
		return fmt.Errorf("cannot save: calibration not complete")
	}

	h := NewHomography()

	h.SetPixelsPerMeter(cm.result.PixelsPerMeter)
	h.Valid = true

	calib := &CalibrationConfig{
		Version: 1,
		Camera: CameraInfo{
			Name:       cm.cameraName,
			Resolution: [2]int{cm.cameraW, cm.cameraH},
		},
		Intrinsics: CameraIntrinsics{
			CameraMatrix: [3][3]float64{
				{cm.result.PixelsPerMeter * 800, 0, float64(cm.cameraW / 2)},
				{0, cm.result.PixelsPerMeter * 800, float64(cm.cameraH / 2)},
				{0, 0, 1},
			},
			DistortionCoeffs: [5]float64{0, 0, 0, 0, 0},
			Width:            cm.cameraW,
			Height:           cm.cameraH,
		},
		Homography: [][]float64{
			{h.H[0][0], h.H[0][1], h.H[0][2]},
			{h.H[1][0], h.H[1][1], h.H[1][2]},
			{h.H[2][0], h.H[2][1], h.H[2][2]},
		},
		WorldScale:   cm.result.PixelsPerMeter,
		TagSize:      cm.tagSize,
		CalibratedAt: "2026-02-06",
	}

	data, err := yaml.Marshal(calib)
	if err != nil {
		return fmt.Errorf("failed to marshal calibration: %w", err)
	}

	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (cm *CalibrationManager) LoadExisting() (bool, error) {
	filename := cm.GetFilename()
	return LoadCalibrationFromFile(filename)
}

func LoadCalibrationFromFile(filename string) (bool, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read calibration file: %w", err)
	}

	var calib CalibrationConfig
	if err := yaml.Unmarshal(data, &calib); err != nil {
		return false, fmt.Errorf("failed to parse calibration file: %w", err)
	}

	if len(calib.Homography) < 3 {
		return false, fmt.Errorf("invalid homography matrix")
	}

	return true, nil
}

func (cm *CalibrationManager) GetResult() *CalibrationResult {
	return cm.result
}

func (cm *CalibrationManager) Cancel() {
	cm.result = &CalibrationResult{
		State:   CalibrationNotCalibrated,
		Message: "Calibration cancelled",
	}
}

type CalibrationAPIResponse struct {
	Calibrated bool    `json:"calibrated"`
	State      string  `json:"state"`
	TagSize    float64 `json:"tagSize,omitempty"`
	Width      float64 `json:"width,omitempty"`
	Height     float64 `json:"height,omitempty"`
	Message    string  `json:"message,omitempty"`
	Filename   string  `json:"filename"`
}

func (cm *CalibrationManager) GetAPIResponse() *CalibrationAPIResponse {
	resp := &CalibrationAPIResponse{
		Calibrated: cm.result.State == CalibrationComplete,
		Filename:   cm.GetFilename(),
	}

	switch cm.result.State {
	case CalibrationNotCalibrated:
		resp.State = "not_calibrated"
		resp.Message = "Click Settings to calibrate"
	case CalibrationDetecting:
		resp.State = "detecting"
		resp.Message = "Place AprilTag in view"
	case CalibrationComputing:
		resp.State = "computing"
		resp.Message = "Computing homography..."
	case CalibrationComplete:
		resp.State = "calibrated"
		resp.TagSize = cm.result.TagSize
		resp.Width = cm.result.ComputedWidth
		resp.Height = cm.result.ComputedHeight
		resp.Message = cm.result.Message
	}

	return resp
}
