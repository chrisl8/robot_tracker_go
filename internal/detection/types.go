package detection

import (
	"image"
	"sync"
)

// DetectionType says where a FusedDetection came from.
type DetectionType int

const (
	// DetectionTypeAprilTag marks a detection made from an AprilTag. It is the only kind the pipeline currently produces.
	DetectionTypeAprilTag DetectionType = iota
	// DetectionTypeFused is reserved for detections that combine several sources.
	DetectionTypeFused
)

// BoundingBox is an axis-aligned rectangle in pixels: top-left (X1, Y1), bottom-right (X2, Y2).
type BoundingBox struct {
	X1, Y1, X2, Y2 int
}

// AprilTag is one detected tag: its ID and family, its four corners and centre in pixels, and its size. Rotation is always 0; a tag's heading comes from its Corners.
type AprilTag struct {
	TagID    int
	Family   string
	Corners  [4][2]float64
	CenterX  float64
	CenterY  float64
	Size     float64
	Rotation float64
}

// FusedDetection is a detection in the form the tracker consumes: a bounding box, the tag ID if it came from a tag, and the tag's corners. ClassName is a legacy field.
type FusedDetection struct {
	DetectionType DetectionType
	Bbox          *BoundingBox
	TagID         *int
	Confidence    float64
	Corners       [4][2]float64
	Source        string
	ClassName     string
}

// DetectionResult is everything found in one frame, with the frame's timestamp and index.
type DetectionResult struct {
	Tags            []AprilTag
	FusedDetections []FusedDetection
	Timestamp       float64
	FrameIdx        int
}

// AprilTagConfig is the detector's input. The service fills only Family and QuadDecimate from the configuration; the other fields are not read from it.
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

// DetectionPipeline runs tag detection on a frame and keeps the static obstacles to draw over it.
type DetectionPipeline struct {
	tagDetector    TagDetector
	obstacleDrawer *ObstacleDrawer

	// obstaclesMu guards obstacles: SetObstacles is called from HTTP handler
	// goroutines while DrawResults runs on the frame loop.
	obstaclesMu sync.RWMutex
	obstacles   []Obstacle
}

// SetObstacles replaces the obstacles drawn over frames. It is safe to call while the frame loop is drawing.
func (p *DetectionPipeline) SetObstacles(obstacles []Obstacle) {
	p.obstaclesMu.Lock()
	p.obstacles = obstacles
	p.obstaclesMu.Unlock()
}

// TagDetector finds tags in an image and draws them. AprilTagDetector implements it.
type TagDetector interface {
	Detect(image []byte, width, height int) []AprilTag
	// DrawTagsOn draws the tags' outlines and labels onto dst.
	DrawTagsOn(dst *image.RGBA, tags []AprilTag)
	Close() error
}
