package detection

import (
	"image"
	"math"
	"reflect"
	"testing"
)

const floatTol = 1e-9

func approx(a, b float64) bool { return math.Abs(a-b) <= floatTol }

func wbox(x0, y0, x1, y1 float64) WorldBox { return WorldBox{MinX: x0, MinY: y0, MaxX: x1, MaxY: y1} }

func TestWorldBox_Basics(t *testing.T) {
	b := wbox(1, 2, 3, 6)
	if b.Width() != 2 || b.Height() != 4 || b.Area() != 8 {
		t.Errorf("size = %vx%v area %v, want 2x4 area 8", b.Width(), b.Height(), b.Area())
	}
	if x, y := b.Center(); x != 2 || y != 4 {
		t.Errorf("center = (%v,%v), want (2,4)", x, y)
	}
	if got := b.Expand(0.5); got != wbox(0.5, 1.5, 3.5, 6.5) {
		t.Errorf("Expand = %+v", got)
	}
	if got := b.Union(wbox(0, 3, 2, 9)); got != wbox(0, 2, 3, 9) {
		t.Errorf("Union = %+v", got)
	}
	if (WorldBox{MinX: 2, MaxX: 1, MinY: 0, MaxY: 1}).Area() != 0 {
		t.Error("an inverted box should have zero area")
	}

	tests := []struct {
		name string
		o    WorldBox
		iou  float64
		hit  bool
	}{
		{"identical", b, 1, true},
		{"half overlap", wbox(2, 2, 4, 6), 4.0 / 12.0, true},
		{"touching edge", wbox(3, 2, 5, 6), 0, true},
		{"apart", wbox(10, 10, 11, 11), 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := b.IoU(tt.o); !approx(got, tt.iou) {
				t.Errorf("IoU = %v, want %v", got, tt.iou)
			}
			if got := b.Intersects(tt.o); got != tt.hit {
				t.Errorf("Intersects = %v, want %v", got, tt.hit)
			}
		})
	}

	if !b.Contains(1, 2) || !b.Contains(3, 6) || b.Contains(3.01, 4) {
		t.Error("Contains should include edges and exclude outside points")
	}
}

// rotatedScaled maps pixels to metres with a 30 degree rotation and 1 cm/px
// scale, so a pixel rectangle becomes a rotated quadrilateral on the floor.
func rotatedScaled(px, py float64) (float64, float64) {
	c, s := math.Cos(math.Pi/6), math.Sin(math.Pi/6)
	return 0.01 * (c*px - s*py), 0.01 * (s*px + c*py)
}

func TestBlobToWorld_UsesAllFourCorners(t *testing.T) {
	r := image.Rect(0, 0, 100, 50)
	got := BlobToWorld(r, rotatedScaled)

	// Projecting only two opposite corners would give x in [0, 0.616].
	x0, y0 := rotatedScaled(0, 0)
	x1, y1 := rotatedScaled(100, 50)
	twoCorner := wbox(math.Min(x0, x1), math.Min(y0, y1), math.Max(x0, x1), math.Max(y0, y1))
	if got == twoCorner {
		t.Fatal("result equals the two-corner box; the other corners were ignored")
	}

	want := wbox(-0.25, 0, 0.8660254037844386, 0.9330127018922193)
	for name, pair := range map[string][2]float64{
		"MinX": {got.MinX, want.MinX}, "MinY": {got.MinY, want.MinY},
		"MaxX": {got.MaxX, want.MaxX}, "MaxY": {got.MaxY, want.MaxY},
	} {
		if math.Abs(pair[0]-pair[1]) > 1e-9 {
			t.Errorf("%s = %v, want %v", name, pair[0], pair[1])
		}
	}
}

func TestLocalPixelsPerMeter(t *testing.T) {
	uniform := func(px, py float64) (float64, float64) { return px * 0.01, py * 0.01 }
	perspective := func(px, py float64) (float64, float64) {
		// scale shrinks with y: farther rows have fewer metres per pixel... more px per metre
		k := 0.01 * (1 + py/1000)
		return px * k, py * k
	}
	degenerate := func(px, py float64) (float64, float64) { return 1, 1 }
	nan := func(px, py float64) (float64, float64) { return math.NaN(), math.NaN() }

	tests := []struct {
		name string
		fn   PixelToWorldFunc
		want float64 // 0 means expect exactly 0
		tol  float64
	}{
		{"uniform 1 cm per pixel", uniform, 100, 1e-9},
		{"degenerate mapping", degenerate, 0, 0},
		{"nan mapping", nan, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LocalPixelsPerMeter(tt.fn, 300, 200); math.Abs(got-tt.want) > tt.tol {
				t.Errorf("LocalPixelsPerMeter = %v, want %v", got, tt.want)
			}
		})
	}

	near, far := LocalPixelsPerMeter(perspective, 300, 0), LocalPixelsPerMeter(perspective, 300, 900)
	if !(near > far) {
		t.Errorf("scale should vary with position under perspective: y=0 gives %v, y=900 gives %v", near, far)
	}
}

func TestRobotDisc(t *testing.T) {
	uniform := func(px, py float64) (float64, float64) { return px * 0.01, py * 0.01 }
	d := RobotDisc(uniform, 400, 300, 0.30, 0.05)
	if d.X != 400 || d.Y != 300 || math.Abs(d.R-20) > 1e-9 {
		t.Errorf("disc = %+v, want centre (400,300) radius 20", d)
	}

	flat := func(px, py float64) (float64, float64) { return 0, 0 }
	if d := RobotDisc(flat, 400, 300, 0.30, 0.05); d.R != 0 || d.X != 400 || d.Y != 300 {
		t.Errorf("degenerate scale should give a zero-radius disc at the centre, got %+v", d)
	}
}

func TestMergeWorldBoxes(t *testing.T) {
	tests := []struct {
		name string
		in   []WorldBox
		gap  float64
		want []WorldBox
	}{
		{"empty", nil, 0.03, []WorldBox{}},
		{"single", []WorldBox{wbox(0, 0, 1, 1)}, 0.03, []WorldBox{wbox(0, 0, 1, 1)}},
		{"overlapping merge", []WorldBox{wbox(0, 0, 1, 1), wbox(0.5, 0.5, 2, 2)}, 0.03, []WorldBox{wbox(0, 0, 2, 2)}},
		{"within gap merge", []WorldBox{wbox(0, 0, 1, 1), wbox(1.05, 0, 2, 1)}, 0.06, []WorldBox{wbox(0, 0, 2, 1)}},
		{"beyond gap stay apart", []WorldBox{wbox(0, 0, 1, 1), wbox(1.05, 0, 2, 1)}, 0.04, []WorldBox{wbox(0, 0, 1, 1), wbox(1.05, 0, 2, 1)}},
		{"chain merges transitively", []WorldBox{wbox(2, 0, 3, 1), wbox(0, 0, 1, 1), wbox(1.02, 0, 2.02, 1)}, 0.03, []WorldBox{wbox(0, 0, 3, 1)}},
		{"sorted by minx then miny", []WorldBox{wbox(5, 5, 6, 6), wbox(1, 9, 2, 10), wbox(1, 1, 2, 2)}, 0.03, []WorldBox{wbox(1, 1, 2, 2), wbox(1, 9, 2, 10), wbox(5, 5, 6, 6)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := append([]WorldBox(nil), tt.in...)
			got, _ := MergeWorldBoxesWithGroups(in, tt.gap)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
			if !reflect.DeepEqual(in, tt.in) {
				t.Error("input slice must not be modified")
			}
		})
	}
}
