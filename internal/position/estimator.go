package position

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"robot_tracker_go/internal/utils"
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

type PositionEstimator struct {
	homography     *Homography
	intrinsics     *CameraIntrinsics
	obstacles      []Obstacle
	smoothing      bool
	smoothingAlpha float64
	positions      map[int]*SmoothedPosition
}

type SmoothedPosition struct {
	X       float64
	Y       float64
	Updated bool
}

func NewPositionEstimator(calibrationPath, obstaclesPath string, smoothing bool, smoothingAlpha float64) (*PositionEstimator, error) {
	est := &PositionEstimator{
		homography:     NewHomography(),
		intrinsics:     nil,
		obstacles:      make([]Obstacle, 0),
		smoothing:      smoothing,
		smoothingAlpha: smoothingAlpha,
		positions:      make(map[int]*SmoothedPosition),
	}

	if calibrationPath != "" && utils.FileExists(calibrationPath) {
		if err := est.LoadCalibration(calibrationPath); err != nil {
			fmt.Printf("Warning: failed to load calibration: %v\n", err)
		}
	}

	if obstaclesPath != "" && utils.FileExists(obstaclesPath) {
		if err := est.LoadObstacles(obstaclesPath); err != nil {
			fmt.Printf("Warning: failed to load obstacles: %v\n", err)
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

	if intrinsicsData, ok := calibration["intrinsics"].(map[string]interface{}); ok {
		e.intrinsics = &CameraIntrinsics{}

		if matrix, ok := intrinsicsData["camera_matrix"].([]interface{}); ok {
			for i := 0; i < 3 && i < len(matrix); i++ {
				row := matrix[i].([]interface{})
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

	if homographyData, ok := calibration["homography"].([]interface{}); ok && len(homographyData) >= 3 {
		row0, _ := homographyData[0].([]interface{})
		row1, _ := homographyData[1].([]interface{})
		row2, _ := homographyData[2].([]interface{})
		if len(row0) >= 3 && len(row1) >= 3 && len(row2) >= 3 {
			e.homography.SetFromValues(
				utils.ToFloat64(row0[0]), utils.ToFloat64(row0[1]), utils.ToFloat64(row0[2]),
				utils.ToFloat64(row1[0]), utils.ToFloat64(row1[1]), utils.ToFloat64(row1[2]),
				utils.ToFloat64(row2[0]), utils.ToFloat64(row2[1]), utils.ToFloat64(row2[2]),
			)
		}
	}

	if scale, ok := calibration["world_scale"].(float64); ok {
		e.homography.SetPixelsPerMeter(scale)
	}

	utils.Logf("Loaded calibration from %s", path)
	return nil
}

func (e *PositionEstimator) LoadObstacles(path string) error {
	// #nosec G304
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read obstacles file: %w", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse obstacles file: %w", err)
	}

	obstaclesData, ok := config["obstacles"]
	if !ok {
		return nil
	}

	obstaclesList, ok := obstaclesData.([]interface{})
	if !ok {
		return nil
	}

	for _, obsData := range obstaclesList {
		obs, ok := obsData.(map[string]interface{})
		if !ok {
			continue
		}

		obstacle := Obstacle{}

		if name, ok := obs["name"].(string); ok {
			obstacle.Name = name
		}

		if world, ok := obs["world"].(map[string]interface{}); ok {
			if tl, ok := world["top_left"].([]interface{}); ok && len(tl) >= 2 {
				obstacle.WorldTopLeft = Point2D{
					X: utils.ToFloat64(tl[0]),
					Y: utils.ToFloat64(tl[1]),
				}
			}
			if br, ok := world["bottom_right"].([]interface{}); ok && len(br) >= 2 {
				obstacle.WorldBottomRight = Point2D{
					X: utils.ToFloat64(br[0]),
					Y: utils.ToFloat64(br[1]),
				}
			}
		}

		if pixels, ok := obs["pixels"].(map[string]interface{}); ok {
			if tl, ok := pixels["top_left"].([]interface{}); ok && len(tl) >= 2 {
				obstacle.PixelsTopLeft = [2]int{
					int(utils.ToFloat64(tl[0])),
					int(utils.ToFloat64(tl[1])),
				}
			}
			if br, ok := pixels["bottom_right"].([]interface{}); ok && len(br) >= 2 {
				obstacle.PixelsBottomRight = [2]int{
					int(utils.ToFloat64(br[0])),
					int(utils.ToFloat64(br[1])),
				}
			}
		}

		e.obstacles = append(e.obstacles, obstacle)
	}

	fmt.Printf("Loaded %d obstacles from %s\n", len(e.obstacles), path)
	return nil
}

func (e *PositionEstimator) PixelToWorld(pixelX, pixelY int) *Point2D {
	if !e.homography.IsValid() {
		return &Point2D{float64(pixelX), float64(pixelY)}
	}
	return e.homography.PixelToWorld(Point2D{float64(pixelX), float64(pixelY)})
}

func (e *PositionEstimator) WorldToPixel(world Point2D) (int, int) {
	if !e.homography.IsValid() {
		return int(world.X), int(world.Y)
	}
	return e.homography.WorldToPixel(world)
}

func (e *PositionEstimator) UpdatePosition(trackID int, x, y float64) {
	if !e.smoothing {
		if e.positions[trackID] == nil {
			e.positions[trackID] = &SmoothedPosition{}
		}
		e.positions[trackID].X = x
		e.positions[trackID].Y = y
		e.positions[trackID].Updated = true
		return
	}

	if e.positions[trackID] == nil {
		e.positions[trackID] = &SmoothedPosition{X: x, Y: y, Updated: true}
	} else {
		sp := e.positions[trackID]
		sp.X = e.smoothingAlpha*x + (1-e.smoothingAlpha)*sp.X
		sp.Y = e.smoothingAlpha*y + (1-e.smoothingAlpha)*sp.Y
		sp.Updated = true
	}
}

func (e *PositionEstimator) GetPosition(trackID int) *Point2D {
	if sp, ok := e.positions[trackID]; ok && sp.Updated {
		return &Point2D{X: sp.X, Y: sp.Y}
	}
	return nil
}

func (e *PositionEstimator) GetObstacles() []Obstacle {
	return e.obstacles
}

func (e *PositionEstimator) AddObstacle(obstacle Obstacle) {
	e.obstacles = append(e.obstacles, obstacle)
}

func (e *PositionEstimator) ClearObstacles() {
	e.obstacles = make([]Obstacle, 0)
}

func (e *PositionEstimator) SaveObstacles(path string, obstacles []Obstacle) error {
	yamlContent := "version: 1\nobstacles:\n"

	for i, obs := range obstacles {
		yamlContent += fmt.Sprintf("  - name: %q\n", obs.Name)
		yamlContent += "    pixels:\n"
		yamlContent += fmt.Sprintf("      top_left: [%d, %d]\n", obs.PixelsTopLeft[0], obs.PixelsTopLeft[1])
		yamlContent += fmt.Sprintf("      bottom_right: [%d, %d]\n", obs.PixelsBottomRight[0], obs.PixelsBottomRight[1])
		yamlContent += "    world:\n"
		yamlContent += fmt.Sprintf("      top_left: [%.4f, %.4f]\n", obs.WorldTopLeft.X, obs.WorldTopLeft.Y)
		yamlContent += fmt.Sprintf("      bottom_right: [%.4f, %.4f]\n", obs.WorldBottomRight.X, obs.WorldBottomRight.Y)
		if i < len(obstacles)-1 {
			yamlContent += "\n"
		}
	}

	// #nosec G304
	// #nosec G306
	return os.WriteFile(path, []byte(yamlContent), 0600)
}

func (e *PositionEstimator) GetHomography() *Homography {
	return e.homography
}

func (e *PositionEstimator) IsCalibrated() bool {
	return e.homography.IsValid()
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
