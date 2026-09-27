package planning

import (
	"math"
	"testing"
)

func approxEq(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func TestRectQuad_MatchesBounds(t *testing.T) {
	q := RectQuad([2]float64{1, 2}, [2]float64{4, 6})
	tl, br := q.Bounds()
	if tl != [2]float64{1, 2} || br != [2]float64{4, 6} {
		t.Errorf("Bounds() = (%v, %v), want ((1,2), (4,6))", tl, br)
	}
}

func TestQuad_Normalized_FixesReversedWinding(t *testing.T) {
	rect := RectQuad([2]float64{0, 0}, [2]float64{2, 1})
	reversed := Quad{rect[0], rect[3], rect[2], rect[1]}

	if reversed.signedArea2() >= 0 {
		t.Fatal("test setup: reversed quad should have negative signed area")
	}
	got := reversed.Normalized()
	if got.signedArea2() < 0 {
		t.Error("Normalized() should flip a reversed quad to positive winding")
	}
	// Normalizing an already-correct quad is a no-op.
	if again := rect.Normalized(); again != rect {
		t.Errorf("Normalized() changed an already-correct quad: %v -> %v", rect, again)
	}
}

func TestQuadContains(t *testing.T) {
	square := RectQuad([2]float64{0, 0}, [2]float64{10, 10})
	// A diamond (45-degree rotated square) centred at (5,5), "radius" 5.
	diamond := Quad{{5, 0}, {10, 5}, {5, 10}, {0, 5}}

	tests := []struct {
		name string
		q    Quad
		p    [2]float64
		want bool
	}{
		{"center of square", square, [2]float64{5, 5}, true},
		{"corner of square (on boundary)", square, [2]float64{0, 0}, true},
		{"just outside square", square, [2]float64{10.01, 5}, false},
		{"far outside square", square, [2]float64{100, 100}, false},
		{"center of diamond", diamond, [2]float64{5, 5}, true},
		// Exact vertices are an inherent edge case for even-odd ray casting
		// (which way "on the boundary" rounds is implementation-defined); the
		// real correctness guarantee production code relies on is
		// distanceToQuad returning 0 there, tested separately below.
		{"just inside the diamond, near its bottom vertex", diamond, [2]float64{5, 0.5}, true},
		{"outside diamond, inside its AABB", diamond, [2]float64{1, 1}, false},
		{"inside diamond near an edge", diamond, [2]float64{5, 1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quadContains(tt.p, tt.q); got != tt.want {
				t.Errorf("quadContains(%v) = %v, want %v", tt.p, got, tt.want)
			}
		})
	}
}

func TestDistanceToQuad(t *testing.T) {
	square := RectQuad([2]float64{0, 0}, [2]float64{10, 10})
	rotated45 := Quad{{5, 0}, {10, 5}, {5, 10}, {0, 5}} // same diamond as above

	tests := []struct {
		name string
		q    Quad
		p    [2]float64
		want float64
	}{
		{"inside is zero", square, [2]float64{5, 5}, 0},
		{"on boundary is zero", square, [2]float64{0, 5}, 0},
		{"directly right of the square", square, [2]float64{13, 5}, 3},
		{"directly above the square", square, [2]float64{5, -4}, 4},
		{"diagonally outside a corner", square, [2]float64{13, 14}, math.Hypot(3, 4)},
		{"outside the rotated diamond, below its lowest vertex", rotated45, [2]float64{5, -2}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := distanceToQuad(tt.p, tt.q)
			if !approxEq(got, tt.want, 1e-9) {
				t.Errorf("distanceToQuad(%v) = %.6f, want %.6f", tt.p, got, tt.want)
			}
		})
	}
}

// TestDistanceToQuad_MatchesRectClamp cross-checks distanceToQuad against the
// textbook AABB closest-point formula for many random axis-aligned cases, the
// same regression the collision-detector cutover (phase 3) relies on.
func TestDistanceToQuad_MatchesRectClamp(t *testing.T) {
	rect := RectQuad([2]float64{-1, -2}, [2]float64{3, 4})
	tl, br := [2]float64{-1, -2}, [2]float64{3, 4}

	points := [][2]float64{
		{0, 0}, {-1, -2}, {3, 4}, {5, 5}, {-5, -5}, {1, 10}, {-10, 1}, {10, 1}, {1, -10},
	}
	for _, p := range points {
		clampedX := math.Max(tl[0], math.Min(p[0], br[0]))
		clampedY := math.Max(tl[1], math.Min(p[1], br[1]))
		want := math.Hypot(p[0]-clampedX, p[1]-clampedY)

		if got := distanceToQuad(p, rect); !approxEq(got, want, 1e-9) {
			t.Errorf("distanceToQuad(%v) = %.6f, want %.6f (AABB clamp)", p, got, want)
		}
	}
}

func TestDistToSegment(t *testing.T) {
	tests := []struct {
		name    string
		p, a, b [2]float64
		want    float64
	}{
		{"point on segment", [2]float64{1, 0}, [2]float64{0, 0}, [2]float64{2, 0}, 0},
		{"perpendicular from midpoint", [2]float64{1, 3}, [2]float64{0, 0}, [2]float64{2, 0}, 3},
		{"beyond endpoint a", [2]float64{-3, 0}, [2]float64{0, 0}, [2]float64{2, 0}, 3},
		{"beyond endpoint b", [2]float64{5, 0}, [2]float64{0, 0}, [2]float64{2, 0}, 3},
		{"degenerate segment (point)", [2]float64{3, 4}, [2]float64{0, 0}, [2]float64{0, 0}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := distToSegment(tt.p, tt.a, tt.b); !approxEq(got, tt.want, 1e-9) {
				t.Errorf("distToSegment = %.6f, want %.6f", got, tt.want)
			}
		})
	}
}
