//go:build gocv

package detection

import (
	"image"
	"image/color"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"gocv.io/x/gocv"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// ForegroundDetector finds objects that appeared in the fixed overhead view:
// anything that differs from an adaptive picture of the empty floor and is not
// a robot or a user-marked static obstacle. The pixel logic lives in
// foregroundModel; this wrapper does the OpenCV image work (resize, gray, blur,
// morphology, connected components).
//
// Process must be called from a single goroutine. Reset, Absorb, RequestDebug
// and DebugJPEG may be called from any goroutine.
type ForegroundDetector struct {
	params ForegroundParams
	model  *foregroundModel

	mu         sync.Mutex
	resetReq   bool
	absorbReqs []image.Point
	debugUntil time.Time
	debugJPEG  []byte

	lastNow    time.Time
	fullW      int
	fullH      int
	sw, sh     int
	robotMask  []uint8
	staticMask []uint8

	// Persistence: saving/restoring the learned background across restarts.
	// See EnablePersistence.
	persistPath     string
	persistInterval time.Duration
	lastPersist     time.Time
	saveInFlight    atomic.Bool
	persistMu       sync.Mutex     // serializes actual writes: see SaveNow
	persistWG       sync.WaitGroup // lets Close wait for an in-flight background save

	small, gray, blur, fgMat, opened, closed gocv.Mat
	labels, stats, centroids                 gocv.Mat
	kOpen, kClose                            gocv.Mat
	dbg                                      gocv.Mat
}

// NewForegroundDetector creates a detector. Zero-valued params are replaced by
// the defaults.
func NewForegroundDetector(p ForegroundParams) *ForegroundDetector {
	def := DefaultForegroundParams()
	if p.Scale <= 0 || p.Scale > 1 {
		p.Scale = def.Scale
	}
	if p.Threshold <= 0 {
		p.Threshold = def.Threshold
	}
	if p.DarkFactor <= 0 {
		p.DarkFactor = def.DarkFactor
	}
	if p.TauSec <= 0 {
		p.TauSec = def.TauSec
	}
	if p.WarmupSec <= 0 {
		p.WarmupSec = def.WarmupSec
	}
	if p.GuardFraction <= 0 {
		p.GuardFraction = def.GuardFraction
	}
	if p.MinBlobPx <= 0 {
		p.MinBlobPx = def.MinBlobPx
	}
	return &ForegroundDetector{
		params: p,
		model:  newForegroundModel(p),
		small:  gocv.NewMat(), gray: gocv.NewMat(), blur: gocv.NewMat(),
		fgMat: gocv.NewMat(), opened: gocv.NewMat(), closed: gocv.NewMat(),
		labels: gocv.NewMat(), stats: gocv.NewMat(), centroids: gocv.NewMat(),
		kOpen:  gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3)),
		kClose: gocv.GetStructuringElement(gocv.MorphRect, image.Pt(9, 9)),
		dbg:    gocv.NewMat(),
	}
}

// IsAvailable reports whether foreground detection is compiled in.
func (d *ForegroundDetector) IsAvailable() bool { return true }

// EnablePersistence turns on saving the learned background to path every
// interval while warm, and restoring it (when it matches the working
// resolution) the first time a frame of that resolution is processed, so a
// restart resumes instantly instead of re-learning. path == "" (the default)
// leaves persistence off, e.g. when there is no calibrated camera to name a
// file after. Call this once, before the first Process call.
func (d *ForegroundDetector) EnablePersistence(path string, interval time.Duration) {
	d.persistPath = path
	d.persistInterval = interval
}

// SaveNow immediately saves the background if persistence is enabled and the
// model is warm, blocking until the write completes. Intended for clean
// shutdown, where a fire-and-forget save (as maybePersist does during normal
// operation) could be killed by process exit before it finishes. It shares
// persistMu with maybePersist's background goroutine (SaveNow can be called
// from a different goroutine, at shutdown, while a periodic save is still in
// flight) so the two can never race on the same file.
func (d *ForegroundDetector) SaveNow() {
	if d.persistPath == "" {
		return
	}
	snap, ok := d.model.snapshot()
	if !ok {
		return
	}
	d.persistMu.Lock()
	defer d.persistMu.Unlock()
	if err := saveModelState(d.persistPath, snap); err != nil {
		utils.Debugf("foreground: could not save background to %s: %v", d.persistPath, err)
	}
}

// maybePersist saves the background in the background (pun intended) at most
// once per persistInterval, while the model is warm. snapshot() already
// copies the slices it returns, so handing them to a goroutine is safe; the
// saveInFlight guard just avoids scheduling a redundant goroutine while one
// is already pending — persistMu (also used by SaveNow) is what actually
// guarantees only one write to the file happens at a time.
func (d *ForegroundDetector) maybePersist(now time.Time) {
	if d.persistPath == "" || d.persistInterval <= 0 {
		return
	}
	if !d.lastPersist.IsZero() && now.Sub(d.lastPersist) < d.persistInterval {
		return
	}
	snap, ok := d.model.snapshot()
	if !ok {
		return
	}
	d.lastPersist = now
	if !d.saveInFlight.CompareAndSwap(false, true) {
		return
	}
	path := d.persistPath
	d.persistWG.Add(1)
	go func() {
		defer d.persistWG.Done()
		defer d.saveInFlight.Store(false)
		d.persistMu.Lock()
		defer d.persistMu.Unlock()
		if err := saveModelState(path, snap); err != nil {
			utils.Debugf("foreground: could not save background to %s: %v", path, err)
		}
	}()
}

// Reset discards the background and starts learning again (the operator says
// the floor is clear).
func (d *ForegroundDetector) Reset() {
	d.mu.Lock()
	d.resetReq = true
	d.mu.Unlock()
}

// Absorb declares the object at full-resolution pixel (x, y) part of the floor.
func (d *ForegroundDetector) Absorb(x, y int) {
	d.mu.Lock()
	d.absorbReqs = append(d.absorbReqs, image.Pt(x, y))
	d.mu.Unlock()
}

// RequestDebug asks for the debug view to be rendered for the next couple of
// seconds; cheap when nobody is watching.
func (d *ForegroundDetector) RequestDebug() {
	d.mu.Lock()
	d.debugUntil = time.Now().Add(2 * time.Second)
	d.mu.Unlock()
}

// DebugJPEG returns the latest debug image (working-resolution frame with the
// foreground in red and masked areas in blue), or nil if none was rendered.
func (d *ForegroundDetector) DebugJPEG() []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.debugJPEG == nil {
		return nil
	}
	return append([]byte(nil), d.debugJPEG...)
}

// Close releases the OpenCV resources. It waits for any in-flight background
// save (from maybePersist) to finish first, so a save never outlives the
// detector — call SaveNow before Close if you also want a final save.
func (d *ForegroundDetector) Close() {
	d.persistWG.Wait()
	for _, m := range []*gocv.Mat{&d.small, &d.gray, &d.blur, &d.fgMat, &d.opened, &d.closed,
		&d.labels, &d.stats, &d.centroids, &d.kOpen, &d.kClose, &d.dbg} {
		_ = m.Close()
	}
}

// Process analyses one BGR frame (width x height) taken at now.
func (d *ForegroundDetector) Process(frame []byte, width, height int, now time.Time, masks ForegroundMasks) ForegroundResult {
	if masks.Suspended || len(frame) != width*height*3 || width <= 0 || height <= 0 {
		d.lastNow = time.Time{}
		return ForegroundResult{}
	}

	d.applyCommands()
	d.ensureSize(width, height)

	src, err := gocv.NewMatFromBytes(height, width, gocv.MatTypeCV8UC3, frame)
	if err != nil || src.Empty() {
		return ForegroundResult{}
	}
	defer func() { _ = src.Close() }()

	if err := gocv.Resize(src, &d.small, image.Pt(d.sw, d.sh), 0, 0, gocv.InterpolationArea); err != nil {
		return ForegroundResult{}
	}
	if err := gocv.CvtColor(d.small, &d.gray, gocv.ColorBGRToGray); err != nil {
		return ForegroundResult{}
	}
	if err := gocv.GaussianBlur(d.gray, &d.blur, image.Pt(5, 5), 0, 0, gocv.BorderDefault); err != nil {
		return ForegroundResult{}
	}
	pixels, err := d.blur.DataPtrUint8()
	if err != nil || len(pixels) != d.sw*d.sh {
		return ForegroundResult{}
	}

	d.rasterizeMasks(masks)

	dt := 0.07
	if !d.lastNow.IsZero() {
		dt = now.Sub(d.lastNow).Seconds()
	}
	d.lastNow = now

	// TODO(shadow-suppression phase 2): pass the resized frame's colour here
	// instead of nil, to enable isShadowColor.
	fg, st := d.model.step(pixels, d.sw, d.sh, dt, d.robotMask, d.staticMask, nil)
	if !st.Warming {
		d.maybePersist(now)
	}
	res := ForegroundResult{Warming: st.Warming, Guarded: st.Guarded, Fraction: st.Fraction, Gain: st.Gain}
	if fg == nil || st.Warming || st.Guarded {
		d.renderDebug(pixels, fg, now)
		return res
	}

	res.Blobs = d.extractBlobs(fg, width, height)
	d.renderDebug(pixels, fg, now)
	return res
}

func (d *ForegroundDetector) applyCommands() {
	d.mu.Lock()
	reset := d.resetReq
	d.resetReq = false
	absorbs := d.absorbReqs
	d.absorbReqs = nil
	d.mu.Unlock()

	if reset {
		d.model.reset()
		if d.persistPath != "" {
			// The operator asked to relearn the floor: a stale save from
			// before the reset must not silently undo that on the next
			// restart, so it goes too, not just the live state.
			deleteModelState(d.persistPath)
		}
	}
	for _, p := range absorbs {
		if d.fullW > 0 && d.fullH > 0 {
			d.model.absorbAt(int(float64(p.X)*float64(d.sw)/float64(d.fullW)),
				int(float64(p.Y)*float64(d.sh)/float64(d.fullH)))
		}
	}
}

func (d *ForegroundDetector) ensureSize(width, height int) {
	if d.fullW == width && d.fullH == height {
		return
	}
	d.fullW, d.fullH = width, height
	d.sw = int(math.Max(16, math.Round(float64(width)*d.params.Scale)))
	d.sh = int(math.Max(16, math.Round(float64(height)*d.params.Scale)))
	d.robotMask = make([]uint8, d.sw*d.sh)
	d.staticMask = make([]uint8, d.sw*d.sh)
	d.model = newForegroundModel(d.params)
	d.restorePersistedState()
	_ = d.fgMat.Close()
	d.fgMat = gocv.NewMatWithSize(d.sh, d.sw, gocv.MatTypeCV8U)
}

// restorePersistedState loads a previously-saved background into the model
// ensureSize just (re)created, when persistence is enabled and the save
// matches the working resolution just computed. A size mismatch (a different
// camera, a different scale config) is left alone to warm up normally,
// exactly as if nothing had been saved.
func (d *ForegroundDetector) restorePersistedState() {
	if d.persistPath == "" {
		return
	}
	s, ok := loadModelState(d.persistPath)
	if !ok {
		return
	}
	if s.W != d.sw || s.H != d.sh {
		utils.Debugf("foreground: ignoring background save at %s (%dx%d, working resolution is %dx%d)",
			d.persistPath, s.W, s.H, d.sw, d.sh)
		return
	}
	if d.model.restore(s) {
		utils.Logf("foreground: restored background from %s, skipping warm-up", d.persistPath)
	}
}

// rasterizeMasks draws this frame's robot discs and static boxes into the
// working-resolution masks.
func (d *ForegroundDetector) rasterizeMasks(m ForegroundMasks) {
	for i := range d.robotMask {
		d.robotMask[i] = 0
		d.staticMask[i] = 0
	}
	sx := float64(d.sw) / float64(d.fullW)
	sy := float64(d.sh) / float64(d.fullH)

	for _, disc := range m.Robots {
		if disc.R <= 0 {
			continue
		}
		cx, cy, r := disc.X*sx, disc.Y*sy, disc.R*sx
		x0, x1 := clampInt(int(cx-r), 0, d.sw-1), clampInt(int(cx+r)+1, 0, d.sw-1)
		y0, y1 := clampInt(int(cy-r), 0, d.sh-1), clampInt(int(cy+r)+1, 0, d.sh-1)
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				dx, dy := float64(x)-cx, float64(y)-cy
				if dx*dx+dy*dy <= r*r {
					d.robotMask[y*d.sw+x] = 1
				}
			}
		}
	}
	for _, rect := range m.Statics {
		x0, x1 := clampInt(int(float64(rect.Min.X)*sx), 0, d.sw), clampInt(int(float64(rect.Max.X)*sx)+1, 0, d.sw)
		y0, y1 := clampInt(int(float64(rect.Min.Y)*sy), 0, d.sh), clampInt(int(float64(rect.Max.Y)*sy)+1, 0, d.sh)
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				d.staticMask[y*d.sw+x] = 1
			}
		}
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// extractBlobs cleans the mask with morphology, and for each connected region
// above the minimum size returns both its bounding box and a tight oriented
// box (gocv.MinAreaRect over the blob's own member pixels), in full-resolution
// pixels. The oriented fit is scoped to each blob's own small bounding
// rectangle from the label mat (bounded by object size, not frame size, and
// blob count is already capped), not a full-frame scan.
func (d *ForegroundDetector) extractBlobs(fg []uint8, width, height int) []DetectedBlob {
	buf, err := d.fgMat.DataPtrUint8()
	if err != nil || len(buf) != len(fg) {
		return nil
	}
	copy(buf, fg)

	if err := gocv.MorphologyEx(d.fgMat, &d.opened, gocv.MorphOpen, d.kOpen); err != nil {
		return nil
	}
	if err := gocv.MorphologyEx(d.opened, &d.closed, gocv.MorphClose, d.kClose); err != nil {
		return nil
	}

	n := gocv.ConnectedComponentsWithStats(d.closed, &d.labels, &d.stats, &d.centroids)
	invX := float64(width) / float64(d.sw)
	invY := float64(height) / float64(d.sh)
	scale := func(p image.Point) image.Point {
		return image.Pt(int(float64(p.X)*invX), int(float64(p.Y)*invY))
	}

	blobs := make([]DetectedBlob, 0, n)
	for i := 1; i < n; i++ {
		area := int(d.stats.GetIntAt(i, int(gocv.CC_STAT_AREA)))
		if area < d.params.MinBlobPx {
			continue
		}
		left := int(d.stats.GetIntAt(i, int(gocv.CC_STAT_LEFT)))
		top := int(d.stats.GetIntAt(i, int(gocv.CC_STAT_TOP)))
		w := int(d.stats.GetIntAt(i, int(gocv.CC_STAT_WIDTH)))
		h := int(d.stats.GetIntAt(i, int(gocv.CC_STAT_HEIGHT)))
		aabb := image.Rect(
			int(float64(left)*invX), int(float64(top)*invY),
			int(float64(left+w)*invX), int(float64(top+h)*invY))

		corners := AABBCorners(aabb)
		if obb, ok := d.minAreaRectForLabel(int32(i), left, top, w, h); ok {
			for j, p := range obb {
				corners[j] = scale(p)
			}
		}
		blobs = append(blobs, DetectedBlob{AABB: aabb, Corners: corners})
	}
	return blobs
}

// minAreaRectForLabel fits an oriented bounding box to the working-resolution
// pixels of connected-component label id within its own bounding rectangle
// (left, top, w, h), in working-resolution (pre-scale) coordinates. It reads
// only that small region of d.labels, not the whole frame.
func (d *ForegroundDetector) minAreaRectForLabel(id int32, left, top, w, h int) ([4]image.Point, bool) {
	const maxPoints = 4096 // a generous cap; MinAreaRect only needs the shape, not every pixel
	pts := make([]image.Point, 0, minInt(w*h, maxPoints))
	stride := 1
	if w*h > maxPoints {
		stride = w*h/maxPoints + 1
	}
	n := 0
	for y := top; y < top+h; y++ {
		for x := left; x < left+w; x++ {
			if d.labels.GetIntAt(y, x) != id {
				continue
			}
			n++
			if stride == 1 || n%stride == 0 {
				pts = append(pts, image.Pt(x, y))
			}
		}
	}
	if len(pts) < 3 {
		return [4]image.Point{}, false
	}

	pv := gocv.NewPointVectorFromPoints(pts)
	defer pv.Close()
	rect := gocv.MinAreaRect(pv)
	if len(rect.Points) != 4 {
		return [4]image.Point{}, false
	}
	return [4]image.Point(rect.Points), true
}

// renderDebug draws the debug image when someone asked for it recently.
func (d *ForegroundDetector) renderDebug(gray []uint8, fg []uint8, now time.Time) {
	d.mu.Lock()
	wanted := time.Now().Before(d.debugUntil)
	d.mu.Unlock()
	if !wanted || fg == nil {
		return
	}

	if err := gocv.CvtColor(d.blur, &d.dbg, gocv.ColorGrayToBGR); err != nil {
		return
	}
	out, err := d.dbg.DataPtrUint8()
	if err != nil || len(out) != d.sw*d.sh*3 {
		return
	}
	for i := range fg {
		switch {
		case fg[i] != 0:
			out[i*3], out[i*3+1], out[i*3+2] = 40, 40, 255
		case d.robotMask[i] != 0:
			out[i*3], out[i*3+1], out[i*3+2] = 255, 120, 0
		case d.staticMask[i] != 0:
			out[i*3], out[i*3+1], out[i*3+2] = 0, 200, 255
		}
	}
	label := "learning background"
	if !d.model.warm {
		_ = gocv.PutText(&d.dbg, label, image.Pt(8, 20), gocv.FontHersheySimplex, 0.5, color.RGBA{R: 255, G: 255, B: 0, A: 255}, 1)
	}
	buf, err := gocv.IMEncodeWithParams(gocv.JPEGFileExt, d.dbg, []int{gocv.IMWriteJpegQuality, 70})
	if err != nil {
		return
	}
	defer buf.Close()
	jpeg := append([]byte(nil), buf.GetBytes()...)

	d.mu.Lock()
	d.debugJPEG = jpeg
	d.mu.Unlock()
	_ = now
}
