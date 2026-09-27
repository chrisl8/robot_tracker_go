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
}

// DefaultForegroundParams returns the tuned defaults.
func DefaultForegroundParams() ForegroundParams {
	return ForegroundParams{
		Scale:          0.5,
		Threshold:      22,
		DarkFactor:     1.4,
		TauSec:         60,
		WarmupSec:      3,
		GuardFraction:  0.25,
		GuardMaxSec:    10,
		MinBlobPx:      60,
		BorderPx:       4,
		AbsorbAfterSec: 0,
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

// ForegroundResult is one frame's detection outcome.
type ForegroundResult struct {
	// Blobs are candidate obstacles in full-resolution frame pixels.
	Blobs []image.Rectangle
	// Warming is true while the background is still being learned.
	Warming bool
	// Guarded is true when the frame looked like a lighting event (too much
	// changed at once); Blobs is empty and callers should hold their last set.
	Guarded bool
	// Fraction is the share of the frame flagged as foreground (before masks).
	Fraction float64
	// Gain is the exposure normalisation applied to the frame.
	Gain float64
}
