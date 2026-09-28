package detection

import "testing"

// TestForegroundParams_WithDefaults covers the defaulting logic
// NewForegroundDetector and config.EffectiveForeground both now route
// through, instead of each keeping their own copy (see the code-review
// tech-debt fix for "duplicated default-filling for ForegroundParams").
func TestForegroundParams_WithDefaults(t *testing.T) {
	t.Run("zero-valued params get the tuned defaults", func(t *testing.T) {
		got := ForegroundParams{}.WithDefaults()
		want := DefaultForegroundParams()
		// AbsorbAfterSec, BorderPx, GuardMaxSec, and the three shadow
		// fields are never touched by WithDefaults, so a zero-valued input
		// keeps them at zero (BorderPx/GuardMaxSec/shadow) or DefaultForegroundParams'
		// own value (AbsorbAfterSec is 0 in both, so this doesn't matter here).
		want.BorderPx = 0
		want.GuardMaxSec = 0
		want.ShadowAlphaMin = 0
		want.ShadowAlphaMax = 0
		want.ShadowChromaMax = 0
		if got != want {
			t.Errorf("WithDefaults() = %+v, want %+v", got, want)
		}
	})

	t.Run("shadow fields left at zero are not coerced to defaults", func(t *testing.T) {
		p := DefaultForegroundParams()
		p.ShadowAlphaMin = 0
		p.ShadowAlphaMax = 0
		p.ShadowChromaMax = 0
		got := p.WithDefaults()
		if got.ShadowAlphaMin != 0 || got.ShadowAlphaMax != 0 || got.ShadowChromaMax != 0 {
			t.Errorf("WithDefaults() coerced an explicit zero shadow bound back to a default: %+v", got)
		}
	})

	t.Run("out-of-range Scale is replaced by the default", func(t *testing.T) {
		def := DefaultForegroundParams()
		for _, scale := range []float64{-1, 0, 1.5} {
			p := def
			p.Scale = scale
			got := p.WithDefaults()
			if got.Scale != def.Scale {
				t.Errorf("Scale=%v: WithDefaults().Scale = %v, want %v", scale, got.Scale, def.Scale)
			}
		}
	})

	t.Run("already-valid values pass through unchanged", func(t *testing.T) {
		p := ForegroundParams{
			Scale: 0.75, Threshold: 30, DarkFactor: 2, TauSec: 90,
			WarmupSec: 5, GuardFraction: 0.4, MinBlobPx: 120,
		}
		got := p.WithDefaults()
		if got != p {
			t.Errorf("WithDefaults() = %+v, want unchanged %+v", got, p)
		}
	})
}
