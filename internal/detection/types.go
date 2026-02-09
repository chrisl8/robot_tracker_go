package detection

type DetectionType int

const (
	DetectionTypeAprilTag DetectionType = iota
	DetectionTypeYOLO
	DetectionTypeFused
)

type BoundingBox struct {
	X1, Y1, X2, Y2 int
}

func (b *BoundingBox) Center() (int, int) {
	return (b.X1 + b.X2) / 2, (b.Y1 + b.Y2) / 2
}

func (b *BoundingBox) Width() int {
	return b.X2 - b.X1
}

func (b *BoundingBox) Height() int {
	return b.Y2 - b.Y1
}

func (b *BoundingBox) Area() int {
	return b.Width() * b.Height()
}

func (b *BoundingBox) Contains(x, y int) bool {
	return x >= b.X1 && x <= b.X2 && y >= b.Y1 && y <= b.Y2
}

func (b *BoundingBox) IoU(other *BoundingBox) float64 {
	x1 := max(b.X1, other.X1)
	y1 := max(b.Y1, other.Y1)
	x2 := min(b.X2, other.X2)
	y2 := min(b.Y2, other.Y2)

	if x2 <= x1 || y2 <= y1 {
		return 0
	}

	intersection := (x2 - x1) * (y2 - y1)
	union := b.Area() + other.Area() - intersection

	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

type YOLODetection struct {
	Bbox       *BoundingBox
	Confidence float64
	ClassID    int
	ClassName  string
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
	YOLODetections  []YOLODetection
	FusedDetections []FusedDetection
	Timestamp       float64
	FrameIdx        int
}

type YOLOConfig struct {
	ModelPath       string
	InputSize       int
	ConfThres       float64
	IOUThres        float64
	Device          string
	ObstacleClasses []string
	RelevantClasses map[int]string // Map of classID → name for filtering
	MinObstacleSize float64        // Minimum obstacle size in meters
	PixelsPerMeter  float64        // Scale factor for size filtering
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
	yoloDetector   YOLODetectorInterface
	yoloEnabled    bool
	obstacleDrawer *ObstacleDrawer
	obstacles      []Obstacle
}

func (p *DetectionPipeline) SetObstacles(obstacles []Obstacle) {
	p.obstacles = obstacles
}

func (p *DetectionPipeline) GetObstacles() []Obstacle {
	return p.obstacles
}

type TagDetector interface {
	Detect(image []byte, width, height int) []AprilTag
	DrawTags(image []byte, width, height int, tags []AprilTag) []byte
	Close() error
}

type YOLODetectorInterface interface {
	Detect(image []byte, width, height int) []YOLODetection
	DrawDetections(image []byte, width, height int, detections []YOLODetection) []byte
	IsAvailable() bool
	SetClassNames(names map[int]string)
	GetClassName(classID int) string
}
