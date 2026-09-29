package position

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

type CameraIntrinsics struct {
	CameraMatrix     [3][3]float64
	DistortionCoeffs [5]float64
	Width            int
	Height           int
}

type CalibrationData struct {
	Intrinsics CameraIntrinsics
	Homography *Homography
	WorldScale float64
}

type CalibrationConfig struct {
	Version      int               `yaml:"version"`
	Camera       CameraInfo        `yaml:"camera"`
	Intrinsics   *CameraIntrinsics `yaml:"intrinsics,omitempty"`
	Homography   [][]float64       `yaml:"homography"`
	WorldScale   float64           `yaml:"world_scale"`
	TagSize      float64           `yaml:"tag_size"`
	CalibratedAt string            `yaml:"calibrated_at"`
	Fit          *CalibrationFit   `yaml:"fit,omitempty"`
}

// CalibrationFit records how well the fitted homography reproduced the
// calibration target tags.
type CalibrationFit struct {
	RMSCm  float64 `yaml:"rms_cm"`
	MaxCm  float64 `yaml:"max_cm"`
	Rating string  `yaml:"rating"`
	Tags   int     `yaml:"tags"`
}

// NewCalibrationConfig builds the file contents for a fitted calibration
// target, recording the actual frame resolution it was made at.
func NewCalibrationConfig(cameraName string, frameWidth, frameHeight int, fit *FitResult, at time.Time) *CalibrationConfig {
	h := fit.Homography
	return &CalibrationConfig{
		Version: 2,
		Camera: CameraInfo{
			Name:       cameraName,
			Resolution: [2]int{frameWidth, frameHeight},
		},
		Homography: [][]float64{
			{h.H[0][0], h.H[0][1], h.H[0][2]},
			{h.H[1][0], h.H[1][1], h.H[1][2]},
			{h.H[2][0], h.H[2][1], h.H[2][2]},
		},
		WorldScale:   h.PixelsPerMeter,
		TagSize:      TargetTagSize,
		CalibratedAt: at.UTC().Format(time.RFC3339),
		Fit: &CalibrationFit{
			RMSCm:  fit.RMSCm,
			MaxCm:  fit.MaxCm,
			Rating: fit.Rating,
			Tags:   len(fit.PerTag),
		},
	}
}

// calibrationBackupsToKeep is how many timestamped copies of a replaced
// calibration file are retained next to it.
const calibrationBackupsToKeep = 5

const calibrationBackupTimeFormat = "20060102-150405"

// SaveCalibration writes cfg as YAML to path, creating the directory. If a
// calibration file already exists it is first copied to
// "<path>.bak-<timestamp>" (the newest calibrationBackupsToKeep are kept), so
// an overwrite, deliberate or not, never destroys the previous calibration.
// A failed backup is logged but does not block saving the new calibration.
func SaveCalibration(path string, cfg *CalibrationConfig) error {
	return saveCalibrationAt(path, cfg, time.Now())
}

func saveCalibrationAt(path string, cfg *CalibrationConfig, now time.Time) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal calibration: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("failed to create calibration directory: %w", err)
	}
	if err := backUpCalibration(path, now); err != nil {
		utils.Logf("Warning: could not back up existing calibration %s: %v", path, err)
	}
	// #nosec G304
	// #nosec G306
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write calibration file: %w", err)
	}
	return nil
}

// backUpCalibration copies an existing calibration file to a timestamped
// backup and prunes the oldest backups. It does nothing if path does not exist.
func backUpCalibration(path string, now time.Time) error {
	// #nosec G304
	existing, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading existing calibration: %w", err)
	}

	backup := fmt.Sprintf("%s.bak-%s", path, now.UTC().Format(calibrationBackupTimeFormat))
	// #nosec G306
	// #nosec G703 -- path is the calibration file name built by the app, not user input
	if err := os.WriteFile(backup, existing, 0600); err != nil {
		return fmt.Errorf("writing backup: %w", err)
	}

	old, err := filepath.Glob(path + ".bak-*")
	if err != nil {
		return fmt.Errorf("listing backups: %w", err)
	}
	sort.Strings(old) // timestamps sort chronologically
	for len(old) > calibrationBackupsToKeep {
		if err := os.Remove(old[0]); err != nil {
			return fmt.Errorf("pruning old backup: %w", err)
		}
		old = old[1:]
	}
	return nil
}

type CameraInfo struct {
	Name       string `yaml:"name"`
	Resolution [2]int `yaml:"resolution"`
}

type PositionEstimator struct {
	// mu guards homography, intrinsics, calibratedRes, and frameRes. Those
	// are mutated by LoadCalibration/SetFrameSize (e.g. from an HTTP handler
	// goroutine when an operator recalibrates live) while PixelToWorld/
	// WorldToPixel/IsCalibrated read them every frame from the
	// frame-processing goroutine.
	mu            sync.Mutex
	homography    *Homography
	calibratedRes [2]int
	frameRes      [2]int
	intrinsics    *CameraIntrinsics
}

func NewPositionEstimator(calibrationPath string) (*PositionEstimator, error) {
	est := &PositionEstimator{
		homography: NewHomography(),
		intrinsics: nil,
	}

	if calibrationPath != "" && utils.FileExists(calibrationPath) {
		if err := est.LoadCalibration(calibrationPath); err != nil {
			fmt.Printf("Warning: failed to load calibration: %v\n", err)
		}
	}

	return est, nil
}

func (e *PositionEstimator) LoadCalibration(path string) error {
	// #nosec G304
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read calibration file: %w", err)
	}

	var calibration map[string]interface{}
	if err := yaml.Unmarshal(data, &calibration); err != nil {
		return fmt.Errorf("failed to parse calibration file: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if intrinsicsData, ok := calibration["intrinsics"].(map[string]interface{}); ok {
		e.intrinsics = &CameraIntrinsics{}

		if matrix, ok := intrinsicsData["camera_matrix"].([]interface{}); ok {
			for i := 0; i < 3 && i < len(matrix); i++ {
				row, ok := matrix[i].([]interface{})
				if !ok {
					continue
				}
				for j := 0; j < 3 && j < len(row); j++ {
					e.intrinsics.CameraMatrix[i][j] = utils.ToFloat64(row[j])
				}
			}
		}

		if width, ok := intrinsicsData["width"].(int); ok {
			e.intrinsics.Width = width
		}
		if height, ok := intrinsicsData["height"].(int); ok {
			e.intrinsics.Height = height
		}
	}

	e.calibratedRes = [2]int{}
	if camera, ok := calibration["camera"].(map[string]interface{}); ok {
		if res, ok := camera["resolution"].([]interface{}); ok && len(res) >= 2 {
			e.calibratedRes = [2]int{int(utils.ToFloat64(res[0])), int(utils.ToFloat64(res[1]))}
		}
	}

	// Build the new homography on a fresh instance rather than mutating
	// e.homography in place, so a concurrent reader always sees either the
	// fully-old or fully-new homography, never a partially-updated one.
	newHomography := NewHomography()

	if homographyData, ok := calibration["homography"].([]interface{}); ok && len(homographyData) >= 3 {
		row0, _ := homographyData[0].([]interface{})
		row1, _ := homographyData[1].([]interface{})
		row2, _ := homographyData[2].([]interface{})
		if len(row0) >= 3 && len(row1) >= 3 && len(row2) >= 3 {
			newHomography.SetFromValues(
				utils.ToFloat64(row0[0]), utils.ToFloat64(row0[1]), utils.ToFloat64(row0[2]),
				utils.ToFloat64(row1[0]), utils.ToFloat64(row1[1]), utils.ToFloat64(row1[2]),
				utils.ToFloat64(row2[0]), utils.ToFloat64(row2[1]), utils.ToFloat64(row2[2]),
			)
		}
	}

	if !newHomography.IsValid() || !newHomography.HasTransformation() {
		utils.Logf("WARNING: Invalid or identity homography in calibration file")
		utils.Logf("Using fallback: identity transformation (pixel=world for testing)")
		newHomography.SetIdentity()
	}

	if scale, ok := calibration["world_scale"].(float64); ok {
		newHomography.SetPixelsPerMeter(scale)
	} else {
		// No world_scale in the file (e.g. an older calibration): fall back
		// to the documented default rather than leaving PixelsPerMeter at 0.
		newHomography.EstimateScale()
	}

	e.homography = newHomography

	utils.Logf("Loaded calibration from %s", path)
	return nil
}

// SetFrameSize records the live camera frame size so a calibration made at a
// different resolution can be flagged.
func (e *PositionEstimator) SetFrameSize(width, height int) {
	e.mu.Lock()
	e.frameRes = [2]int{width, height}
	e.mu.Unlock()
}

// CalibratedResolution returns the resolution stored in the loaded
// calibration file, if any.
func (e *PositionEstimator) CalibratedResolution() (width, height int, ok bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calibratedRes[0], e.calibratedRes[1], e.calibratedRes[0] > 0 && e.calibratedRes[1] > 0
}

// ResolutionMismatch reports whether the loaded calibration was made at a
// different resolution than the camera is currently delivering. Pixel
// coordinates only map to the floor correctly at the calibrated resolution.
func (e *PositionEstimator) ResolutionMismatch() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.calibratedRes[0] == 0 || e.frameRes[0] == 0 {
		return false
	}
	return e.calibratedRes != e.frameRes
}

func (e *PositionEstimator) PixelToWorld(pixelX, pixelY int) *Point2D {
	h := e.currentHomography()
	if !h.IsValid() {
		return &Point2D{float64(pixelX), float64(pixelY)}
	}
	return h.PixelToWorld(Point2D{float64(pixelX), float64(pixelY)})
}

func (e *PositionEstimator) PixelToWorldFloat(pixelX, pixelY float64) *Point2D {
	h := e.currentHomography()
	if !h.IsValid() {
		return &Point2D{pixelX, pixelY}
	}
	return h.PixelToWorld(Point2D{pixelX, pixelY})
}

func (e *PositionEstimator) WorldToPixel(world Point2D) (int, int) {
	h := e.currentHomography()
	if !h.IsValid() {
		return int(world.X), int(world.Y)
	}
	return h.WorldToPixel(world)
}

// currentHomography returns the homography in effect at the time of the
// call. LoadCalibration swaps e.homography to a freshly-built instance
// rather than mutating the existing one in place, so a locked pointer read
// here is enough to make concurrent recalibration and lookups safe: callers
// always see a fully-formed old or new homography, never a half-updated one.
func (e *PositionEstimator) currentHomography() *Homography {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.homography
}

func (e *PositionEstimator) GetHomography() *Homography {
	return e.currentHomography()
}

func (e *PositionEstimator) IsCalibrated() bool {
	return e.currentHomography().IsValid()
}

type PositionResult struct {
	Positions []RobotPosition
	Timestamp float64
	FrameIdx  int
}

type RobotPosition struct {
	TrackID    int
	X, Y       float64
	Confidence float64
	TagID      int
}
