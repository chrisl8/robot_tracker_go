package position

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

const (
	// TargetTagSize is the edge length, in metres, of the black square on the
	// printable calibration tags hosted by the app. The tags print at exactly
	// this size, so it is assumed rather than entered by the user.
	TargetTagSize = 0.15

	// TargetIDMin..TargetIDMax are the tag IDs reserved for the calibration
	// target. Robots must not use them.
	TargetIDMin    = 100
	TargetIDMax    = 104
	TargetCenterID = 100

	DefaultTargetWidth = 1.0
	DefaultTargetDepth = 0.6

	minTargetSpread = 0.3
	maxTargetSpread = 10.0

	// Fit quality thresholds on the RMS reprojection error, in centimetres.
	GoodFitRMSCm = 1.0
	OKFitRMSCm   = 2.0
	// MaxFitRMSCm is the limit above which a calibration is rejected and not saved.
	MaxFitRMSCm = 3.0
)

const (
	RatingGood = "good"
	RatingOK   = "ok"
	RatingPoor = "poor"
)

const (
	RoleCenter = "center"
	RoleCorner = "corner"
)

var (
	ErrPoorFit        = errors.New("calibration fit is too inaccurate")
	ErrTargetSpread   = errors.New("invalid target spread")
	ErrMissingTags    = errors.New("calibration target tags missing")
	ErrUnknownTargetT = errors.New("not a calibration target tag")
)

// TargetTag describes one printed calibration tag and where it belongs on the
// floor: Col/Row are -1, 0 or +1 and multiply half the laid-out width/depth.
type TargetTag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
	Role  string `json:"role"`
	Col   int    `json:"col"`
	Row   int    `json:"row"`
}

var targetTags = []TargetTag{
	{ID: 100, Label: "Center", Role: RoleCenter, Col: 0, Row: 0},
	{ID: 101, Label: "Corner 1 (top-left)", Role: RoleCorner, Col: -1, Row: -1},
	{ID: 102, Label: "Corner 2 (top-right)", Role: RoleCorner, Col: 1, Row: -1},
	{ID: 103, Label: "Corner 3 (bottom-right)", Role: RoleCorner, Col: 1, Row: 1},
	{ID: 104, Label: "Corner 4 (bottom-left)", Role: RoleCorner, Col: -1, Row: 1},
}

// TargetTags returns a copy of the fixed calibration target layout.
func TargetTags() []TargetTag {
	return append([]TargetTag(nil), targetTags...)
}

// IsTargetTagID reports whether id is reserved for the calibration target.
func IsTargetTagID(id int) bool {
	return id >= TargetIDMin && id <= TargetIDMax
}

func targetTagByID(id int) (TargetTag, bool) {
	for _, t := range targetTags {
		if t.ID == id {
			return t, true
		}
	}
	return TargetTag{}, false
}

// WorldCorners returns the tag's four corner positions in world metres for a
// target laid out width x depth. The world frame is centred on the Center
// tag with +x to the right and +y down in the camera view; corners are in
// detector order (top-left, top-right, bottom-right, bottom-left).
func (t TargetTag) WorldCorners(width, depth float64) [4]Point2D {
	cx := float64(t.Col) * width / 2
	cy := float64(t.Row) * depth / 2
	h := TargetTagSize / 2
	return [4]Point2D{
		{X: cx - h, Y: cy - h},
		{X: cx + h, Y: cy - h},
		{X: cx + h, Y: cy + h},
		{X: cx - h, Y: cy + h},
	}
}

// TargetCapture is one detected target tag: its ID and pixel corners in
// detector order.
type TargetCapture struct {
	ID      int
	Corners [4]Point2D
}

// TagFit is the fit residual for the four corners of one target tag.
// HeldOutCm is the RMS error of this tag's corners when predicted by a fit
// made WITHOUT this tag; it is what exposes a misplaced tag, because the
// in-sample fit bends toward it. It is zero when there are too few tags.
type TagFit struct {
	ID        int     `json:"id"`
	Label     string  `json:"label"`
	RMSCm     float64 `json:"rmsCm"`
	MaxCm     float64 `json:"maxCm"`
	HeldOutCm float64 `json:"heldOutCm"`
}

// FitResult is the outcome of fitting the target: the homography and how well
// it reproduces the laid-out tag positions.
type FitResult struct {
	Homography *Homography
	RMSCm      float64
	MaxCm      float64
	// QualityCm is the larger of RMSCm and the worst held-out tag error; it
	// drives Rating and the accept/reject decision.
	QualityCm float64
	Rating    string
	// WorstTagID is the tag most likely to be misplaced (largest held-out
	// error, or largest in-sample RMS when held-out errors are unavailable).
	WorstTagID int
	PerTag     []TagFit
	Width      float64
	Depth      float64
}

// minTagsForHeldOut is the number of tags needed before a leave-one-tag-out
// check is meaningful: the remaining tags must still span the floor.
const minTagsForHeldOut = 5

// RateFit converts an RMS reprojection error in centimetres to a rating.
func RateFit(rmsCm float64) string {
	switch {
	case rmsCm < GoodFitRMSCm:
		return RatingGood
	case rmsCm < OKFitRMSCm:
		return RatingOK
	default:
		return RatingPoor
	}
}

// FitTarget fits the pixel-to-world homography to the detected target tags
// laid out as a width x depth rectangle. It needs the Center tag and at least
// three corner tags. When the fit is worse than MaxFitRMSCm it still returns
// the result (so callers can show what was wrong) together with ErrPoorFit.
func FitTarget(captures []TargetCapture, width, depth float64) (*FitResult, error) {
	if width < minTargetSpread || width > maxTargetSpread || depth < minTargetSpread || depth > maxTargetSpread {
		return nil, fmt.Errorf("%w: width and depth must each be between %.1f and %.0f m", ErrTargetSpread, minTargetSpread, maxTargetSpread)
	}

	seen := make(map[int]bool, len(captures))
	tags := make([]TargetTag, 0, len(captures))
	src := make([]Point2D, 0, len(captures)*4)
	dst := make([]Point2D, 0, len(captures)*4)
	owner := make([]int, 0, len(captures)*4) // index into tags for each point
	for _, c := range captures {
		tag, ok := targetTagByID(c.ID)
		if !ok {
			return nil, fmt.Errorf("%w: id %d", ErrUnknownTargetT, c.ID)
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("tag %d captured twice", c.ID)
		}
		seen[c.ID] = true

		tags = append(tags, tag)
		world := tag.WorldCorners(width, depth)
		for i := 0; i < 4; i++ {
			src = append(src, c.Corners[i])
			dst = append(dst, world[i])
			owner = append(owner, len(tags)-1)
		}
	}

	corners := 0
	for _, tag := range tags {
		if tag.Role == RoleCorner {
			corners++
		}
	}
	if !seen[TargetCenterID] || corners < 3 {
		return nil, fmt.Errorf("%w: need the Center tag and at least 3 corner tags (have %d tags)", ErrMissingTags, len(tags))
	}

	h := NewHomography()
	if err := h.ComputeFromPoints(src, dst); err != nil {
		return nil, fmt.Errorf("computing homography: %w", err)
	}

	rms, maxCm, perPoint := h.ReprojectionError(src, dst)

	perTag := make([]TagFit, len(tags))
	sumSq := make([]float64, len(tags))
	for i, e := range perPoint {
		t := owner[i]
		sumSq[t] += e * e
		if e > perTag[t].MaxCm {
			perTag[t].MaxCm = e
		}
	}
	for i, tag := range tags {
		perTag[i].ID = tag.ID
		perTag[i].Label = tag.Label
		perTag[i].RMSCm = sqrt(sumSq[i] / 4)
	}

	quality := rms
	if len(tags) >= minTagsForHeldOut {
		for i := range tags {
			perTag[i].HeldOutCm = heldOutErrorCm(src, dst, owner, i)
		}
	}

	worst := 0
	for i := range perTag {
		if len(tags) >= minTagsForHeldOut {
			if perTag[i].HeldOutCm > perTag[worst].HeldOutCm {
				worst = i
			}
			quality = math.Max(quality, perTag[i].HeldOutCm)
		} else if perTag[i].RMSCm > perTag[worst].RMSCm {
			worst = i
		}
	}
	worstID := perTag[worst].ID

	sort.Slice(perTag, func(a, b int) bool { return perTag[a].ID < perTag[b].ID })

	result := &FitResult{
		Homography: h,
		RMSCm:      rms,
		MaxCm:      maxCm,
		QualityCm:  quality,
		Rating:     RateFit(quality),
		WorstTagID: worstID,
		PerTag:     perTag,
		Width:      width,
		Depth:      depth,
	}
	if quality > MaxFitRMSCm {
		return result, fmt.Errorf("%w: %.1f cm (limit %.1f cm)", ErrPoorFit, quality, MaxFitRMSCm)
	}
	return result, nil
}

// heldOutErrorCm fits a homography to every point NOT owned by tag index
// skip, then returns the RMS error (cm) of skip's own points under that fit.
// Returns 0 when the reduced fit is not computable.
func heldOutErrorCm(src, dst []Point2D, owner []int, skip int) float64 {
	subSrc := make([]Point2D, 0, len(src))
	subDst := make([]Point2D, 0, len(src))
	outSrc := make([]Point2D, 0, 4)
	outDst := make([]Point2D, 0, 4)
	for i := range src {
		if owner[i] == skip {
			outSrc = append(outSrc, src[i])
			outDst = append(outDst, dst[i])
		} else {
			subSrc = append(subSrc, src[i])
			subDst = append(subDst, dst[i])
		}
	}
	sub := NewHomography()
	if err := sub.ComputeFromPoints(subSrc, subDst); err != nil {
		return 0
	}
	rms, _, _ := sub.ReprojectionError(outSrc, outDst)
	return rms
}
