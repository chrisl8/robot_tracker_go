package followsim

import (
	"fmt"
	"math"
	"sort"
)

// Summary is a follower's record over many runs. Medians and tails matter more
// than means here: a follower that is usually fine and sometimes hits things
// is not fine.
type Summary struct {
	Runs                                                 int
	ReachedPct, CollidedPct, LeftFramePct, PlanFailedPct float64
	SecondsP50, SecondsP90                               float64 // reached runs only
	ClearanceP05, ClearanceP50                           float64 // true gap to obstacles
	CrossTrackP50, CrossTrackP90                         float64
	ReplansMean, SwitchesPerMetre                        float64
}

// Summarize aggregates results.
func Summarize(rs []Result) Summary {
	s := Summary{Runs: len(rs)}
	if len(rs) == 0 {
		return s
	}
	var secs, clear, cross []float64
	var replans, switches, travelled float64
	each := 100 / float64(len(rs))
	for _, r := range rs {
		if r.Reached {
			s.ReachedPct += each
			secs = append(secs, r.Seconds)
		}
		if r.Collided {
			s.CollidedPct += each
		}
		if r.LeftFrame {
			s.LeftFramePct += each
		}
		if r.PlanFailed {
			s.PlanFailedPct += each
			continue
		}
		clear = append(clear, r.MinClearance)
		cross = append(cross, r.MaxCrossTrack)
		replans += float64(r.Replans)
		switches += float64(r.CmdSwitches)
		travelled += r.Travelled
	}
	s.SecondsP50, s.SecondsP90 = percentile(secs, 50), percentile(secs, 90)
	s.ClearanceP05, s.ClearanceP50 = percentile(clear, 5), percentile(clear, 50)
	s.CrossTrackP50, s.CrossTrackP90 = percentile(cross, 50), percentile(cross, 90)
	if n := float64(len(clear)); n > 0 {
		s.ReplansMean = replans / n
	}
	if travelled > 0 {
		s.SwitchesPerMetre = switches / travelled
	}
	return s
}

// percentile is the p-th percentile (nearest rank) of xs, NaN if empty.
func percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	i := int(math.Ceil(p/100*float64(len(s)))) - 1
	return s[max(0, min(len(s)-1, i))]
}

// String formats the summary as one line of percentages and percentiles for the report.
func (s Summary) String() string {
	return fmt.Sprintf("runs=%d reached=%.0f%% collided=%.0f%% leftFrame=%.0f%% planFail=%.0f%% | time p50=%.0fs p90=%.0fs | clearance p05=%+.3fm p50=%+.3fm | crossTrack p50=%.3fm p90=%.3fm | replans=%.2f switches/m=%.1f",
		s.Runs, s.ReachedPct, s.CollidedPct, s.LeftFramePct, s.PlanFailedPct,
		s.SecondsP50, s.SecondsP90, s.ClearanceP05, s.ClearanceP50, s.CrossTrackP50, s.CrossTrackP90, s.ReplansMean, s.SwitchesPerMetre)
}
