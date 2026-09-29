package detection

import "sync"

type DetectionType int

const (
	DetectionTypeAprilTag DetectionType = iota
	DetectionTypeFused
)

type BoundingBox struct {
	X1, Y1, X2, Y2 int
}

type AprilTag struct {
	TagID    int
	Family   string
	Corners  [4][2]float64
	CenterX  float64
	CenterY  float64
	Size     float64
	Rotation float64
}

type FusedDetection struct {
	DetectionType DetectionType
	Bbox          *BoundingBox
	TagID         *int
	Confidence    float64
	Corners       [4][2]float64
	Source        string
	ClassName     string
}

type DetectionResult struct {
	Tags            []AprilTag
	FusedDetections []FusedDetection
	Timestamp       float64
	FrameIdx        int
}

type AprilTagConfig struct {
	Family             string
	TagSize            float64
	NThreads           int
	QuadDecimate       float64
	QuadSigma          float64
	RefineEdges        int
	DecodingSharpening float64
	MinTagSize         float64
	MaxHammingDistance int
}

type DetectionPipeline struct {
	tagDetector    TagDetector
	obstacleDrawer *ObstacleDrawer

	// obstaclesMu guards obstacles: SetObstacles is called from HTTP handler
	// goroutines while DrawResults runs on the frame loop.
	obstaclesMu sync.RWMutex
	obstacles   []Obstacle
}

func (p *DetectionPipeline) SetObstacles(obstacles []Obstacle) {
	p.obstaclesMu.Lock()
	p.obstacles = obstacles
	p.obstaclesMu.Unlock()
}

type TagDetector interface {
	Detect(image []byte, width, height int) []AprilTag
	DrawTags(image []byte, width, height int, tags []AprilTag) []byte
	Close() error
}
