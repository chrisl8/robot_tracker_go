//go:build gocv

package main

import (
	"fmt"
	"image"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/config"
	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/ui"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

const (
	// lostRobotTTL is how long a robot whose tag was just lost keeps being
	// masked at its last position (growing with time), so it is not mistaken
	// for an obstacle during a momentary detection gap.
	lostRobotTTL = 5 * time.Second
	// lostRobotSpeed is the assumed maximum robot speed (m/s) used to grow the
	// lost-robot mask.
	lostRobotSpeed = 0.25
	// defaultRobotDiameter is used for tags with no configured robot.
	defaultRobotDiameter = 0.30
	// tempObstacleHeartbeat is how often the UI is re-sent an unchanged set.
	tempObstacleHeartbeat = 2 * time.Second
)

type robotSighting struct {
	x, y float64 // last pixel position of the tag
	diam float64
	at   time.Time
}

// foregroundGlue connects the foreground detector to the rest of the system:
// it builds the masks (robots, static obstacles), runs the detector, filters
// the result over time, feeds the planner, and tells the UI. Everything except
// the command flags is touched only from the frame goroutine.
type foregroundGlue struct {
	settings config.ForegroundSettings
	det      *detection.ForegroundDetector
	filter   *detection.TemporalFilter

	enabled atomic.Bool
	apply   atomic.Bool

	cmdMu       sync.Mutex
	resetReq    bool
	absorbWorld []image.Point // pending absorb requests, natural pixels

	robots      map[int]robotSighting
	wasSuspend  bool
	published   []detection.TrackedBox
	lastSig     string
	lastSent    time.Time
	stateMu     sync.Mutex
	state       ui.ForegroundState
	lastApplied bool
	known       map[string]detection.WorldBox // ids logged as present
}

func newForegroundGlue(s config.ForegroundSettings) *foregroundGlue {
	params := detection.DefaultForegroundParams()
	params.Scale = s.Scale
	params.Threshold = s.Threshold
	params.DarkFactor = s.DarkFactor
	params.TauSec = s.TauSec
	params.WarmupSec = s.WarmupSec
	params.GuardFraction = s.GuardFraction
	params.AbsorbAfterSec = s.AbsorbAfterSec
	params.ShadowAlphaMin = s.ShadowAlphaMin
	params.ShadowAlphaMax = s.ShadowAlphaMax
	params.ShadowChromaMax = s.ShadowChromaMax

	tp := detection.DefaultTemporalParams()
	tp.Appear = time.Duration(s.AppearMs) * time.Millisecond
	tp.Vanish = time.Duration(s.VanishMs) * time.Millisecond
	tp.MinSize = s.MinSizeM
	tp.MaxTracks = s.MaxBlobs

	g := &foregroundGlue{
		settings: s,
		det:      detection.NewForegroundDetector(params),
		filter:   detection.NewTemporalFilter(tp),
		robots:   make(map[int]robotSighting),
		known:    make(map[string]detection.WorldBox),
	}
	g.enabled.Store(s.Enabled)
	g.apply.Store(s.ApplyToPlanner)
	return g
}

// requestReset asks the frame goroutine to forget the background and any
// published obstacles (safe to call from any goroutine).
func (g *foregroundGlue) requestReset() {
	g.det.Reset()
	g.cmdMu.Lock()
	g.resetReq = true
	g.cmdMu.Unlock()
}

func (g *foregroundGlue) requestAbsorb(x, y int) {
	g.det.Absorb(x, y)
	g.cmdMu.Lock()
	g.absorbWorld = append(g.absorbWorld, image.Pt(x, y))
	g.cmdMu.Unlock()
}

func (g *foregroundGlue) snapshotState() ui.ForegroundState {
	g.stateMu.Lock()
	defer g.stateMu.Unlock()
	st := g.state
	st.Enabled = g.enabled.Load()
	st.Applied = g.apply.Load()
	return st
}

// initForeground creates the detector and wires the UI controls.
func (rs *RobotSystem) initForeground() {
	settings := rs.cfg.EffectiveForeground()
	rs.fg = newForegroundGlue(settings)
	g := rs.fg
	utils.Logf("Foreground obstacle detection: enabled=%v steering_planner=%v", g.enabled.Load(), g.apply.Load())

	if settings.PersistBackground {
		if name := rs.cameraDisplayName(); name != "" {
			path := ui.GetForegroundStateFilename(name)
			interval := time.Duration(settings.PersistIntervalSec * float64(time.Second))
			g.det.EnablePersistence(path, interval)
			utils.Logf("Foreground background persistence: %s every %v", path, interval)
		}
	}

	rs.webServer.OnForegroundEnabled = func(on bool) {
		g.enabled.Store(on)
		if on {
			g.requestReset()
		}
		utils.Logf("Foreground obstacle detection %s", map[bool]string{true: "enabled", false: "disabled"}[on])
	}
	rs.webServer.OnForegroundApply = func(on bool) {
		g.apply.Store(on)
		utils.Logf("Foreground obstacles steer the planner: %v", on)
	}
	rs.webServer.OnForegroundReset = func() {
		g.requestReset()
		utils.Logf("Foreground background reset requested")
	}
	rs.webServer.OnForegroundAbsorb = func(x, y int) {
		g.requestAbsorb(x, y)
		utils.Logf("Foreground absorb requested at pixel (%d,%d)", x, y)
	}
	rs.webServer.ForegroundDebugJPEG = func() []byte {
		g.det.RequestDebug()
		return g.det.DebugJPEG()
	}
	rs.webServer.ForegroundState = g.snapshotState
}

// drainCommands applies pending reset/absorb requests to the temporal filter.
func (g *foregroundGlue) drainCommands(est *position.PositionEstimator) {
	g.cmdMu.Lock()
	reset := g.resetReq
	g.resetReq = false
	absorbs := g.absorbWorld
	g.absorbWorld = nil
	g.cmdMu.Unlock()

	if reset {
		g.filter.Reset()
		g.published = nil
	}
	if est == nil || !est.IsCalibrated() {
		return
	}
	for _, p := range absorbs {
		w := est.PixelToWorld(p.X, p.Y)
		g.filter.Absorb(w.X, w.Y)
	}
}

// processForeground runs one frame through the foreground pipeline and, when
// enabled, publishes the resulting temporary obstacles to the planner and UI.
func (rs *RobotSystem) processForeground(frame []byte, width, height int, now time.Time, result *detection.DetectionResult) {
	g := rs.fg
	if g == nil {
		return
	}
	est := rs.positionEst
	g.drainCommands(est)

	calibrated := est != nil && est.IsCalibrated()
	if !g.enabled.Load() || !calibrated {
		rs.publishTempObstacles(now, nil, false, false, g.enabled.Load(), 0)
		return
	}

	suspended := rs.webServer != nil && rs.webServer.CalibrationViewActive()
	if suspended {
		// Tags are lying on the floor during calibration: stop detecting and
		// relearn afterwards.
		g.wasSuspend = true
		g.published = nil
		g.filter.Reset()
		rs.publishTempObstacles(now, nil, false, false, true, 0)
		return
	}
	if g.wasSuspend {
		g.wasSuspend = false
		g.det.Reset()
		g.filter.Reset()
	}

	toWorld := func(px, py float64) (float64, float64) {
		p := est.PixelToWorldFloat(px, py)
		return p.X, p.Y
	}
	masks := rs.buildForegroundMasks(now, result, toWorld)
	res := g.det.Process(frame, width, height, now, masks)

	switch {
	case res.Warming:
		g.published = nil
	case res.Guarded:
		// Lighting event: hold the last obstacles rather than flooding the planner.
	default:
		dets := make([]detection.Detection, 0, len(res.Blobs))
		for _, b := range res.Blobs {
			dets = append(dets, detection.Detection{
				Box:  detection.BlobToWorld(b.AABB, toWorld),
				Quad: detection.BlobQuadToWorld(b.Corners, toWorld),
			})
		}
		g.published = g.filter.Update(now, dets)
	}
	rs.publishTempObstacles(now, g.published, res.Warming, res.Guarded, true, res.ShadowSuppressed)
}

// buildForegroundMasks returns the discs and boxes the detector must ignore:
// every robot (at its detected tag, or its last position for a few seconds
// after the tag is lost) and the user's static obstacles.
func (rs *RobotSystem) buildForegroundMasks(now time.Time, result *detection.DetectionResult, toWorld detection.PixelToWorldFunc) detection.ForegroundMasks {
	g := rs.fg
	var masks detection.ForegroundMasks

	seen := make(map[int]bool)
	for _, tag := range result.Tags {
		if position.IsTargetTagID(tag.TagID) {
			continue
		}
		diam := defaultRobotDiameter
		if rc := rs.cfg.GetRobotByTagID(tag.TagID); rc != nil && rc.Diameter > 0 {
			diam = rc.Diameter
		}
		g.robots[tag.TagID] = robotSighting{x: tag.CenterX, y: tag.CenterY, diam: diam, at: now}
		seen[tag.TagID] = true
		masks.Robots = append(masks.Robots, detection.RobotDisc(toWorld, tag.CenterX, tag.CenterY, diam, g.settings.RobotMarginM))
	}
	for id, s := range g.robots {
		if seen[id] {
			continue
		}
		age := now.Sub(s.at)
		if age > lostRobotTTL {
			delete(g.robots, id)
			continue
		}
		margin := g.settings.RobotMarginM + math.Min(lostRobotSpeed*age.Seconds(), 0.5)
		masks.Robots = append(masks.Robots, detection.RobotDisc(toWorld, s.x, s.y, s.diam, margin))
	}

	if rs.webServer != nil {
		pad := g.settings.StaticMarginPx
		for _, obs := range rs.webServer.GetObstacles() {
			masks.Statics = append(masks.Statics, image.Rect(
				obs.PixelsTopLeft[0]-pad, obs.PixelsTopLeft[1]-pad,
				obs.PixelsBottomRight[0]+pad, obs.PixelsBottomRight[1]+pad))
		}
	}
	return masks
}

// publishTempObstacles gives the planner (when steering is on) and the UI the
// current temporary obstacles. The planner call is made every frame because it
// is cheap when nothing changed; the UI message is sent on change and as a
// heartbeat.
func (rs *RobotSystem) publishTempObstacles(now time.Time, tracked []detection.TrackedBox, warming, guarded, enabled bool, shadowSuppressed int) {
	g := rs.fg
	est := rs.positionEst

	obstacles := make([]planning.Obstacle, 0, len(tracked))
	msgs := make([]ui.TempObstacleResponse, 0, len(tracked))
	for _, t := range tracked {
		box := t.Box.Expand(g.settings.PadM)
		quad := t.Quad.ExpandRect(g.settings.PadM)
		name := fmt.Sprintf("temp_%d", t.ID)
		obstacles = append(obstacles, planning.Obstacle{
			Name:             name,
			WorldTopLeft:     [2]float64{box.MinX, box.MinY},
			WorldBottomRight: [2]float64{box.MaxX, box.MaxY},
			// The real, possibly-rotated footprint (a degenerate rectangle
			// when the detector could not fit an oriented box), padded the
			// same as the cached AABB envelope above.
			Quad: planning.Quad(quad),
		})
		var tl, br [2]int
		var pixelQuad [][2]int
		if est != nil {
			tl, br = worldBoxToPixels(est, box)
			pixelQuad = worldQuadToPixels(est, quad)
		}
		worldQuad := make([][2]float64, 4)
		copy(worldQuad, quad[:])
		msgs = append(msgs, ui.TempObstacleResponse{
			ID: name, PixelTopLeft: tl, PixelBottomRight: br,
			WorldTopLeft:     [2]float64{box.MinX, box.MinY},
			WorldBottomRight: [2]float64{box.MaxX, box.MaxY},
			PixelQuad:        pixelQuad,
			WorldQuad:        worldQuad,
		})
	}

	g.logChanges(tracked)

	applied := g.apply.Load() && enabled
	if applied {
		rs.planner.SetDynamicObstacles(obstacles)
	} else if g.lastApplied {
		rs.planner.SetDynamicObstacles(nil)
	}
	g.lastApplied = applied

	sig := fmt.Sprintf("%v|%v|%v|%v", applied, warming, guarded, enabled)
	for _, m := range msgs {
		sig += fmt.Sprintf("|%s:%.2f,%.2f,%.2f,%.2f", m.ID, m.WorldTopLeft[0], m.WorldTopLeft[1], m.WorldBottomRight[0], m.WorldBottomRight[1])
	}

	g.stateMu.Lock()
	g.state = ui.ForegroundState{Warming: warming, Guarded: guarded, Count: len(msgs), ShadowSuppressed: shadowSuppressed}
	g.stateMu.Unlock()

	if rs.webServer != nil && (sig != g.lastSig || now.Sub(g.lastSent) >= tempObstacleHeartbeat) {
		g.lastSig, g.lastSent = sig, now
		rs.webServer.BroadcastTempObstacles(ui.TempObstaclesMessage{
			Obstacles: msgs, Applied: applied, Warming: warming, Guarded: guarded, Enabled: enabled,
		})
	}
}

// worldBoxToPixels returns the pixel bounding box of a floor rectangle.
func worldBoxToPixels(est *position.PositionEstimator, b detection.WorldBox) ([2]int, [2]int) {
	corners := [4]position.Point2D{{X: b.MinX, Y: b.MinY}, {X: b.MaxX, Y: b.MinY}, {X: b.MaxX, Y: b.MaxY}, {X: b.MinX, Y: b.MaxY}}
	minX, minY, maxX, maxY := math.MaxInt32, math.MaxInt32, math.MinInt32, math.MinInt32
	for _, c := range corners {
		px, py := est.WorldToPixel(c)
		minX, maxX = min(minX, px), max(maxX, px)
		minY, maxY = min(minY, py), max(maxY, py)
	}
	return [2]int{minX, minY}, [2]int{maxX, maxY}
}

// worldQuadToPixels projects a quad's four corners (in order) to pixels.
func worldQuadToPixels(est *position.PositionEstimator, q detection.Quad) [][2]int {
	out := make([][2]int, 4)
	for i, c := range q {
		x, y := est.WorldToPixel(position.Point2D{X: c[0], Y: c[1]})
		out[i] = [2]int{x, y}
	}
	return out
}

// logChanges records temporary obstacles appearing and disappearing, so a run
// can be judged from the log afterwards.
func (g *foregroundGlue) logChanges(tracked []detection.TrackedBox) {
	current := make(map[string]detection.WorldBox, len(tracked))
	for _, t := range tracked {
		id := fmt.Sprintf("temp_%d", t.ID)
		current[id] = t.Box
		if _, seen := g.known[id]; !seen {
			cx, cy := t.Box.Center()
			utils.Logf("Temporary obstacle %s appeared at (%.2f, %.2f) size %.2f x %.2f m (steering=%v)",
				id, cx, cy, t.Box.Width(), t.Box.Height(), g.apply.Load())
		}
	}
	for id, box := range g.known {
		if _, still := current[id]; !still {
			cx, cy := box.Center()
			utils.Logf("Temporary obstacle %s disappeared from (%.2f, %.2f)", id, cx, cy)
		}
	}
	g.known = current
}

// perfSummary is a short description for the PERF log line.
func (g *foregroundGlue) perfSummary() string {
	st := g.snapshotState()
	flags := ""
	if st.Warming {
		flags = " warming"
	}
	if st.Guarded {
		flags += " guarded"
	}
	shadow := ""
	if st.ShadowSuppressed > 0 {
		shadow = fmt.Sprintf(" shadow=%d", st.ShadowSuppressed)
	}
	return fmt.Sprintf(" fg=%d%s%s", st.Count, flags, shadow)
}
