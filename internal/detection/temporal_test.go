package detection

import (
	"math"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func at(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }

func boxNear(a, b WorldBox, tol float64) bool {
	return math.Abs(a.MinX-b.MinX) <= tol && math.Abs(a.MinY-b.MinY) <= tol &&
		math.Abs(a.MaxX-b.MaxX) <= tol && math.Abs(a.MaxY-b.MaxY) <= tol
}

// dets wraps boxes as Detections with a degenerate rectangular Quad matching
// each box, for tests that only care about the AABB-tracking behaviour.
func dets(boxes ...WorldBox) []Detection {
	out := make([]Detection, len(boxes))
	for i, b := range boxes {
		out[i] = Detection{Box: b, Quad: rectQuad(b)}
	}
	return out
}

// instant publishes on first sight so tests can focus on other behaviour.
func instant() TemporalParams {
	p := DefaultTemporalParams()
	p.Appear = 0
	return p
}

var objA = wbox(1.00, 1.00, 1.20, 1.20)

func TestTemporalFilter_AppearsOnlyAfterAppear(t *testing.T) {
	f := NewTemporalFilter(DefaultTemporalParams())
	for _, ms := range []int{0, 100, 200, 300} {
		if got := f.Update(at(ms), dets(objA)); len(got) != 0 {
			t.Fatalf("published %v at %d ms, before Appear", got, ms)
		}
	}
	got := f.Update(at(400), dets(objA))
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("at Appear got %+v, want one box with ID 1", got)
	}
}

func TestTemporalFilter_FlickerStaysContinuousAndVanishesLate(t *testing.T) {
	f := NewTemporalFilter(DefaultTemporalParams())
	for ms := 0; ms <= 500; ms += 100 {
		f.Update(at(ms), dets(objA))
	}
	if got := f.Update(at(500), dets(objA)); len(got) != 1 {
		t.Fatalf("expected the object to be published, got %+v", got)
	}

	// Absent for 900 ms (< Vanish 1500 ms): still published, same ID.
	if got := f.Update(at(1400), nil); len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("a short flicker dropped the object: %+v", got)
	}
	if got := f.Update(at(1450), dets(objA)); len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("object should keep its ID after a flicker, got %+v", got)
	}

	// Absent until exactly Vanish after the last sighting: gone.
	if got := f.Update(at(2900), nil); len(got) != 1 {
		t.Fatalf("still within Vanish (1450 ms ago), object should remain: %+v", got)
	}
	if got := f.Update(at(2950), nil); len(got) != 0 {
		t.Fatalf("object should vanish after Vanish, got %+v", got)
	}

	// Coming back later is a new object with a new ID.
	f.Update(at(3000), dets(objA))
	got := f.Update(at(3400), dets(objA))
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("returning object should be new (ID 2), got %+v", got)
	}
}

func TestTemporalFilter_Hysteresis(t *testing.T) {
	tests := []struct {
		name    string
		moved   WorldBox
		changes bool
	}{
		{"1 cm jitter keeps the published box", wbox(1.01, 1.01, 1.21, 1.21), false},
		{"1.9 cm jitter keeps the published box", wbox(1.019, 1.0, 1.219, 1.2), false},
		{"5 cm move publishes a new box", wbox(1.05, 1.0, 1.25, 1.2), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewTemporalFilter(instant())
			first := f.Update(at(0), dets(objA))
			if len(first) != 1 {
				t.Fatalf("expected published box, got %+v", first)
			}
			next := f.Update(at(100), dets(tt.moved))
			if len(next) != 1 {
				t.Fatalf("expected one box, got %+v", next)
			}
			if changed := next[0].Box != first[0].Box; changed != tt.changes {
				t.Errorf("published box changed = %v, want %v (first %+v, next %+v)", changed, tt.changes, first[0].Box, next[0].Box)
			}
		})
	}
}

func TestTemporalFilter_QuantisesOutward(t *testing.T) {
	f := NewTemporalFilter(instant())
	got := f.Update(at(0), dets(wbox(0.101, 0.203, 0.309, 0.407)))
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	want := wbox(0.10, 0.20, 0.32, 0.42)
	if !boxNear(got[0].Box, want, 1e-9) {
		t.Errorf("quantised box = %+v, want %+v", got[0].Box, want)
	}

	// Values already on the grid are not pushed a cell outward by float error.
	f = NewTemporalFilter(instant())
	got = f.Update(at(0), dets(wbox(0.30, 0.30, 0.50, 0.50)))
	if !boxNear(got[0].Box, wbox(0.30, 0.30, 0.50, 0.50), 1e-9) {
		t.Errorf("on-grid box changed: %+v", got[0].Box)
	}

	// Quantum 0 leaves boxes alone.
	p := instant()
	p.Quantum = 0
	f = NewTemporalFilter(p)
	raw := wbox(0.1013, 0.2027, 0.3091, 0.4073)
	if got = f.Update(at(0), dets(raw)); got[0].Box != raw {
		t.Errorf("quantum 0 should not modify the box, got %+v", got[0].Box)
	}
}

func TestTemporalFilter_MaxTracksKeepsLargestSortedByID(t *testing.T) {
	p := instant()
	p.MaxTracks = 2
	f := NewTemporalFilter(p)
	small := wbox(0, 0, 0.10, 0.10)  // area 0.01
	medium := wbox(1, 0, 1.20, 0.20) // area 0.04
	large := wbox(2, 0, 2.40, 0.40)  // area 0.16
	got := f.Update(at(0), dets(small, medium, large))
	if len(got) != 2 {
		t.Fatalf("got %d boxes, want 2: %+v", len(got), got)
	}
	// IDs follow merge order (sorted by MinX): small=1, medium=2, large=3.
	if got[0].ID != 2 || got[1].ID != 3 {
		t.Errorf("kept IDs %d,%d, want the two largest (2,3) in ID order", got[0].ID, got[1].ID)
	}
}

func TestTemporalFilter_DropsTinyDetections(t *testing.T) {
	f := NewTemporalFilter(instant())
	tests := []struct {
		name string
		box  WorldBox
		want int
	}{
		{"too narrow", wbox(0, 0, 0.03, 0.30), 0},
		{"too short", wbox(0, 0, 0.30, 0.04), 0},
		{"just big enough", wbox(0, 0, 0.05, 0.05), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f.Reset()
			if got := f.Update(at(0), dets(tt.box)); len(got) != tt.want {
				t.Errorf("got %d boxes, want %d", len(got), tt.want)
			}
		})
	}
}

func TestTemporalFilter_MergesOverlappingDetections(t *testing.T) {
	f := NewTemporalFilter(instant())
	got := f.Update(at(0), dets(wbox(1, 1, 1.20, 1.20), wbox(1.15, 1.0, 1.40, 1.20), wbox(5, 5, 5.2, 5.2)))
	if len(got) != 2 {
		t.Fatalf("got %+v, want the two overlapping detections merged (2 boxes total)", got)
	}
	if got[0].Box.Width() < 0.39 {
		t.Errorf("merged box too narrow: %+v", got[0].Box)
	}
}

// TestTemporalFilter_QuadsStayPairedWithTheirOwnBoxWhenUnsorted is a
// regression test: MergeWorldBoxesWithGroups sorts its output by position, so
// candidates() must track each detection's quad through that reorder by
// identity (its group), not by assuming the Nth output box is the Nth input
// detection. Three separate, non-merging boxes are given out of sorted
// (MinX, MinY) order, each with its own distinctive (marker) quad, so a
// mispairing shows up as one track publishing a different track's quad.
func TestTemporalFilter_QuadsStayPairedWithTheirOwnBoxWhenUnsorted(t *testing.T) {
	f := NewTemporalFilter(instant())
	boxA := wbox(5, 5, 5.1, 5.1) // sorts last
	boxB := wbox(1, 1, 1.1, 1.1) // sorts first
	boxC := wbox(3, 3, 3.1, 3.1) // sorts middle
	quadA := Quad{{9, 9}, {9, 9}, {9, 9}, {9, 9}}
	quadB := Quad{{1, 1}, {1, 1}, {1, 1}, {1, 1}}
	quadC := Quad{{5, 5}, {5, 5}, {5, 5}, {5, 5}}

	got := f.Update(at(0), []Detection{
		{Box: boxA, Quad: quadA},
		{Box: boxB, Quad: quadB},
		{Box: boxC, Quad: quadC},
	})
	if len(got) != 3 {
		t.Fatalf("got %d tracks, want 3", len(got))
	}
	cases := []struct {
		box  WorldBox
		quad Quad
	}{{boxA, quadA}, {boxB, quadB}, {boxC, quadC}}
	for _, tb := range got {
		matched := false
		for _, c := range cases {
			if !boxNear(tb.Box, c.box, 1e-9) {
				continue
			}
			matched = true
			if tb.Quad != c.quad {
				t.Errorf("box %v published with quad %v, want its own quad %v (a different track's quad leaked in)", tb.Box, tb.Quad, c.quad)
			}
		}
		if !matched {
			t.Fatalf("published box %v does not match any input box", tb.Box)
		}
	}
}

func TestTemporalFilter_IDsStableAndMonotonicAcrossReset(t *testing.T) {
	f := NewTemporalFilter(instant())
	a := wbox(1, 1, 1.2, 1.2)
	b := wbox(3, 3, 3.2, 3.2)

	first := f.Update(at(0), dets(a, b))
	if len(first) != 2 || first[0].ID != 1 || first[1].ID != 2 {
		t.Fatalf("first frame IDs = %+v, want 1 and 2", first)
	}
	// Both drift slightly and swap order in the input: IDs must follow the objects.
	second := f.Update(at(100), dets(b.Expand(0.005), a.Expand(0.005)))
	if len(second) != 2 || second[0].ID != 1 || second[1].ID != 2 {
		t.Fatalf("IDs changed between frames: %+v", second)
	}

	f.Reset()
	if got := f.Update(at(200), nil); len(got) != 0 {
		t.Fatalf("Reset should clear tracks, got %+v", got)
	}
	after := f.Update(at(300), dets(a))
	if len(after) != 1 || after[0].ID != 3 {
		t.Errorf("ID after Reset = %+v, want 3 (never reused)", after)
	}
}

func TestTemporalFilter_AbsorbRemovesSuppressesThenExpires(t *testing.T) {
	f := NewTemporalFilter(instant())
	if got := f.Update(at(0), dets(objA)); len(got) != 1 {
		t.Fatalf("setup failed: %+v", got)
	}

	if f.Absorb(9, 9) {
		t.Error("Absorb outside any obstacle should report false")
	}
	if !f.Absorb(1.1, 1.1) {
		t.Fatal("Absorb inside the obstacle should report true")
	}
	if f.Absorb(1.1, 1.1) {
		t.Error("the obstacle was already absorbed")
	}

	// The expiry clock starts at the next Update (1 s) and lasts 5 s.
	for _, ms := range []int{1000, 2000, 5000, 5900} {
		if got := f.Update(at(ms), dets(objA)); len(got) != 0 {
			t.Fatalf("absorbed object reappeared at %d ms: %+v", ms, got)
		}
	}
	got := f.Update(at(6100), dets(objA))
	if len(got) != 1 || got[0].ID != 2 {
		t.Errorf("after suppression expired the object should be tracked again as ID 2, got %+v", got)
	}
}

func TestTemporalFilter_MatchesByCentreWhenNoOverlap(t *testing.T) {
	f := NewTemporalFilter(instant())
	first := f.Update(at(0), dets(wbox(1, 1, 1.10, 1.10)))
	// Jumps a full box width sideways: no overlap, but the centre is within 15 cm.
	second := f.Update(at(100), dets(wbox(1.10, 1.0, 1.20, 1.10)))
	if len(first) != 1 || len(second) != 1 || first[0].ID != second[0].ID {
		t.Errorf("expected the same track to follow the object: %+v then %+v", first, second)
	}

	// Far away is a different object.
	third := f.Update(at(200), dets(wbox(1.10, 1.0, 1.20, 1.10), wbox(4, 4, 4.1, 4.1)))
	if len(third) != 2 || third[0].ID == third[1].ID {
		t.Errorf("distant detection should be a separate track: %+v", third)
	}
}
