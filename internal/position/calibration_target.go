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
	// this size, so it is assumed rather than entered by the user. It is the
	// ONLY length the calibration is built on; tag positions are solved from it.
	TargetTagSize = 0.15

	// TargetIDMin..TargetIDMax are the tag IDs reserved for the calibration
	// target. Robots must not use them.
	TargetIDMin    = 100
	TargetIDMax    = 104
	TargetCenterID = 100

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
	ErrMissingTags    = errors.New("calibration target tags missing")
	ErrUnknownTargetT = errors.New("not a calibration target tag")
)

// TargetTag describes one printed calibration tag. GuideX/GuideY are where the
// wizard draws its placement box, as fractions (0-1) of the video frame. They
// only help the user spread the tags around the view; the solver does not
// assume the tags were placed exactly there.
type TargetTag struct {
	ID     int     `json:"id"`
	Label  string  `json:"label"`
	Role   string  `json:"role"`
	GuideX float64 `json:"guideX"`
	GuideY float64 `json:"guideY"`
}

var targetTags = []TargetTag{
	{ID: 100, Label: "Center", Role: RoleCenter, GuideX: 0.5, GuideY: 0.5},
	{ID: 101, Label: "Corner 1 (top-left)", Role: RoleCorner, GuideX: 0.22, GuideY: 0.24},
	{ID: 102, Label: "Corner 2 (top-right)", Role: RoleCorner, GuideX: 0.78, GuideY: 0.24},
	{ID: 103, Label: "Corner 3 (bottom-right)", Role: RoleCorner, GuideX: 0.78, GuideY: 0.76},
	{ID: 104, Label: "Corner 4 (bottom-left)", Role: RoleCorner, GuideX: 0.22, GuideY: 0.76},
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

// TargetCapture is one detected target tag: its ID and pixel corners in
// detector order (top-left, top-right, bottom-right, bottom-left).
type TargetCapture struct {
	ID      int
	Corners [4]Point2D
}

// TagFit is the fit residual for the four corners of one target tag: how far
// they land, in cm, from a perfect 15 cm square once mapped to the floor.
type TagFit struct {
	ID    int     `json:"id"`
	Label string  `json:"label"`
	RMSCm float64 `json:"rmsCm"`
	MaxCm float64 `json:"maxCm"`
}

// DistanceCheck is a floor distance between two tags implied by the fit. It is
// an OPTIONAL sanity check the user can verify with a tape measure; it is never
// used as input.
type DistanceCheck struct {
	FromID    int     `json:"fromId"`
	ToID      int     `json:"toId"`
	FromLabel string  `json:"fromLabel"`
	ToLabel   string  `json:"toLabel"`
	Meters    float64 `json:"meters"`
}

// FitResult is the outcome of fitting the target.
type FitResult struct {
	Homography *Homography
	RMSCm      float64
	MaxCm      float64
	// QualityCm drives Rating and the accept/reject decision: the larger of
	// the overall RMS and the worst single tag's RMS, so one bad tag (curled,
	// printed at the wrong size) cannot hide in the average.
	QualityCm float64
	Rating    string
	// WorstTagID is the tag whose corners fit worst: the one most likely
	// curled, printed at the wrong size, or badly detected.
	WorstTagID int
	PerTag     []TagFit
	Checks     []DistanceCheck
}

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

// checkPairs are the tag pairs reported as floor distances.
var checkPairs = [][2]int{{101, 102}, {102, 103}, {101, 103}}

// FitTarget fits the pixel-to-floor homography to the detected target tags.
// The tags may lie wherever the user dropped them: their floor positions are
// solved from their known size (see solveLayout). It needs the Center tag and
// at least three corner tags. When the fit is worse than MaxFitRMSCm it still
// returns the result (so callers can show what was wrong) with ErrPoorFit.
func FitTarget(captures []TargetCapture) (*FitResult, error) {
	seen := make(map[int]bool, len(captures))
	tags := make([]TargetTag, 0, len(captures))
	byID := make(map[int]TargetCapture, len(captures))
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
		byID[c.ID] = c
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

	// Center first: it defines the world frame. The rest in ID order.
	sort.Slice(tags, func(a, b int) bool {
		if (tags[a].ID == TargetCenterID) != (tags[b].ID == TargetCenterID) {
			return tags[a].ID == TargetCenterID
		}
		return tags[a].ID < tags[b].ID
	})
	obs := make([][4]Point2D, len(tags))
	for i, tag := range tags {
		obs[i] = byID[tag.ID].Corners
	}

	sol, err := solveLayout(obs)
	if err != nil {
		return nil, err
	}

	pw, ok := inverse3(sol.worldToPixel)
	if !ok {
		return nil, fmt.Errorf("%w: singular homography", errLayoutSolve)
	}
	if math.Abs(pw[2][2]) > 1e-12 {
		s := pw[2][2]
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				pw[i][j] /= s
			}
		}
	}
	h := NewHomography()
	h.H = pw
	h.ComputeInverse()
	src := make([]Point2D, 0, len(tags)*4)
	for _, o := range obs {
		src = append(src, o[:]...)
	}
	h.PixelsPerMeter = h.meanPixelsPerMeter(src)
	h.Valid = true

	local := localTagCorners()
	dst := make([]Point2D, 0, len(tags)*4)
	for _, pose := range sol.poses {
		c := poseCorners(pose, local)
		dst = append(dst, c[:]...)
	}
	rms, maxCm, perPoint := h.ReprojectionError(src, dst)
	if len(perPoint) != len(src) {
		return nil, fmt.Errorf("%w: could not measure the fit error", errLayoutSolve)
	}

	perTag := make([]TagFit, len(tags))
	worst := 0
	for i, tag := range tags {
		sumSq, tagMax := 0.0, 0.0
		for _, e := range perPoint[i*4 : i*4+4] {
			sumSq += e * e
			tagMax = math.Max(tagMax, e)
		}
		perTag[i] = TagFit{ID: tag.ID, Label: tag.Label, RMSCm: math.Sqrt(sumSq / 4), MaxCm: tagMax}
		if perTag[i].RMSCm > perTag[worst].RMSCm {
			worst = i
		}
	}
	worstID := perTag[worst].ID

	center := make(map[int]Point2D, len(tags))
	labels := make(map[int]string, len(tags))
	for i, tag := range tags {
		center[tag.ID] = Point2D{X: sol.poses[i].X, Y: sol.poses[i].Y}
		labels[tag.ID] = tag.Label
	}
	checks := make([]DistanceCheck, 0, len(checkPairs))
	for _, pair := range checkPairs {
		a, okA := center[pair[0]]
		b, okB := center[pair[1]]
		if okA && okB {
			checks = append(checks, DistanceCheck{
				FromID: pair[0], ToID: pair[1],
				FromLabel: labels[pair[0]], ToLabel: labels[pair[1]],
				Meters: math.Hypot(a.X-b.X, a.Y-b.Y),
			})
		}
	}

	sort.Slice(perTag, func(a, b int) bool { return perTag[a].ID < perTag[b].ID })

	quality := math.Max(rms, perTag[worst].RMSCm)
	result := &FitResult{
		Homography: h,
		RMSCm:      rms,
		MaxCm:      maxCm,
		QualityCm:  quality,
		Rating:     RateFit(quality),
		WorstTagID: worstID,
		PerTag:     perTag,
		Checks:     checks,
	}
	if quality > MaxFitRMSCm {
		return result, fmt.Errorf("%w: %.1f cm (limit %.1f cm)", ErrPoorFit, quality, MaxFitRMSCm)
	}
	return result, nil
}
