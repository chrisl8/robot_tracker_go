package detection

import "image"

// ForegroundParams tunes the background-subtraction obstacle detector. They are
// built from the config file's foreground section (see cmd) so this package
// does not depend on the config package.
type ForegroundParams struct {
	// Scale is the working resolution relative to the camera frame (e.g. 0.5).
	Scale float64
	// Threshold is the gray-level difference (0-255) that counts as a change
	// when the pixel got brighter; darkening (shadows) needs DarkFactor times it.
	Threshold  float64
	DarkFactor float64
	// TauSec is the background's learning time constant in seconds.
	TauSec float64
	// WarmupSec is how long the background is averaged before detecting.
	WarmupSec float64
	// GuardFraction is the foreground fraction of the frame above which the
	// frame is treated as a lighting event rather than obstacles.
	GuardFraction float64
	// GuardMaxSec is how long the guard may hold before the background is
	// relearned from scratch (assuming a lasting lighting change).
	GuardMaxSec float64
	// MinBlobPx is the smallest blob area, in working-resolution pixels.
	MinBlobPx int
	// BorderPx ignores this many working-resolution pixels around the edge.
	BorderPx int
	// AbsorbAfterSec folds an unchanged foreground pixel into the background
	// after this long; 0 means never (objects stay obstacles until removed).
	AbsorbAfterSec float64
	// Shadow suppression: a darkened pixel whose colour is still (within
	// these bounds) just the background colour scaled down is a cast shadow,
	// not an object, and is not flagged as foreground. ShadowAlphaMin/Max
	// bound the plausible scale factor (0-1, how much darker); ShadowChromaMax
	// bounds the leftover colour error (as a fraction of the background
	// colour's magnitude) after removing that scale. See isShadowColor. Any
	// of these left at zero (the zero value) disables the gate, since 0 is
	// not a usable bound for either.
	ShadowAlphaMin  float64
	ShadowAlphaMax  float64
	ShadowChromaMax float64
}

// DefaultForegroundParams returns the tuned defaults.
func DefaultForegroundParams() ForegroundParams {
	return ForegroundParams{
		Scale:           0.5,
		Threshold:       22,
		DarkFactor:      1.4,
		TauSec:          60,
		WarmupSec:       3,
		GuardFraction:   0.25,
		GuardMaxSec:     10,
		MinBlobPx:       60,
		BorderPx:        4,
		AbsorbAfterSec:  0,
		ShadowAlphaMin:  0.15,
		ShadowAlphaMax:  0.98,
		ShadowChromaMax: 0.20,
	}
}

// Disc is a circle in full-resolution frame pixels.
type Disc struct{ X, Y, R float64 }

// ForegroundMasks are the areas the detector must ignore this frame, in
// full-resolution frame pixels.
type ForegroundMasks struct {
	// Robots are discs around robots (tag detected or recently lost).
	Robots []Disc
	// Statics are user-marked static obstacles, already padded.
	Statics []image.Rectangle
	// Suspended stops detection and background updates (e.g. while the
	// calibration wizard has tags lying on the floor).
	Suspended bool
}

// DetectedBlob is one candidate obstacle, in full-resolution frame pixels.
// Corners is its tight oriented bounding box (via gocv.MinAreaRect on the
// blob's own member pixels), in detector order (not guaranteed clockwise or
// counter-clockwise — callers that need a specific winding, such as building
// a planning.Quad, must normalize it). When an oriented fit was not possible
// (e.g. too few pixels), Corners falls back to AABB's own four corners, so it
// is always populated and always a valid (possibly degenerate) quadrilateral.
type DetectedBlob struct {
	AABB    image.Rectangle
	Corners [4]image.Point
}

// AABBCorners returns r's four corners in the same order MinAreaRect uses
// (starting at the bottom-left, clockwise), the fallback DetectedBlob.Corners
// value when an oriented fit is unavailable.
func AABBCorners(r image.Rectangle) [4]image.Point {
	return [4]image.Point{
		{X: r.Min.X, Y: r.Max.Y},
		{X: r.Min.X, Y: r.Min.Y},
		{X: r.Max.X, Y: r.Min.Y},
		{X: r.Max.X, Y: r.Max.Y},
	}
}

// ForegroundResult is one frame's detection outcome.
type ForegroundResult struct {
	// Blobs are candidate obstacles in full-resolution frame pixels.
	Blobs []DetectedBlob
	// Warming is true while the background is still being learned.
	Warming bool
	// Guarded is true when the frame looked like a lighting event (too much
	// changed at once); Blobs is empty and callers should hold their last set.
	Guarded bool
	// Fraction is the share of the frame flagged as foreground (before masks).
	Fraction float64
	// Gain is the exposure normalisation applied to the frame.
	Gain float64
	// ShadowSuppressed is how many pixels this frame were reclassified from
	// would-be foreground to background by the colour-based shadow test
	// (isShadowColor) — diagnostic only, see foregroundGlue.perfSummary.
	ShadowSuppressed int
}
