# Codebase Review Findings — robot_tracker_go

## Context

This is a review-only task (no code changes requested). The user asked for a thorough
audit of the codebase for bugs, tech debt, and over-complexity, since it's been built
up over many sessions by different agents and has become hard to trace. Three parallel
Explore agents covered: (1) detection & position, (2) tracking & planning, (3)
controller & UI + `cmd/main.go`. Findings below are deduplicated, cross-referenced, and
ranked by severity. This file is the deliverable — a report, not an implementation plan.

---

## 🔴 Critical — safety / correctness bugs likely active right now

1. **[FIXED] Releasing the last held movement key never stops the robot.**
   `ui/src/components/ControlPanel.vue:43-60` — the keyboard watcher is an
   `if/else if` chain over `w/s/a/d/x` with no final `else`. When all keys are
   released, nothing matches, so `sendCommand('S')` is never sent. Because
   `CommandQueue` (`internal/controller/queue.go:107-117`) keeps re-sending the last
   active command on every heartbeat, **the robot keeps driving after you let go of
   the key**. The mouse D-pad already does this correctly (`mouseup`/`mouseleave` →
   `S`), so it's a one-line regression, not a design gap — but it's a physical robot
   that won't stop. Fix: add `else { sendCommand('S') }`.

2. **[FIXED] Robot velocity fed into the planner is hardcoded to zero.**
   `cmd/main.go:1371-1372` — `UpdateRobotState(robotID, pos, [2]float64{0, 0})`.
   The entire velocity-obstacle collision-avoidance system (`internal/planning/local.go`)
   computes time-to-collision from this value. A fast-approaching robot looks
   stationary to every other robot's avoidance math — this undermines the premise of
   the local planner. No robot velocity is computed anywhere in `main.go`.

3. **[FIXED] Hungarian-algorithm unmatched rows silently default to "matched to track 0."**
   `internal/tracking/hungarian.go:103-108` — when there are more detections than
   tracks, some assignment rows are never written and Go's zero-value (`0`) is
   indistinguishable from a real match to track index 0. Downstream in
   `bytetrack.go`, this can silently glue an unrelated new detection onto track 0
   (ID-switch risk) instead of correctly creating a new track. Not covered by
   `hungarian_test.go` (only tests square matrices with all-real assignments).

4. **[FIXED] Multi-obstacle avoidance: last obstacle processed wins, not the most dangerous.**
   `internal/planning/local.go:61-98` (`applyVelocityObstacles`) — `avoidanceVel` is
   overwritten each loop iteration rather than combined/intersected. With 2+ nearby
   robots/obstacles, the final escape velocity can still be inside an earlier
   obstacle's collision cone. This is a real avoidance failure, not just an
   inefficiency.

5. **[FIXED] Data race on calibration reload while tracking is running.**
   `internal/position/estimator.go` — `LoadCalibration` mutates `homography`/
   `intrinsics` fields with no lock, while `PixelToWorld`/`WorldToPixel` read them
   every frame from the processing goroutine with no lock either (only
   `calibratedRes`/`frameRes` are mutex-guarded). Recalibrating live (a normal
   operator action, wired via `OnCalibrationComplete` from the HTTP handler
   goroutine) races against the frame-processing goroutine — can send a robot's
   computed position to a nonsensical place.

6. **[FIXED] `CommandQueue`/`ArduinoController` have unsynchronized shared state.**
   `internal/controller/queue.go` — `running` is read/written without `q.mu` from
   multiple goroutines (HTTP handler via `ClearEmergencyStop`, frame-processing
   goroutine, shutdown path). Concurrent `Stop()` calls can both pass the
   `if !q.running` check and double-`close(q.stopCh)` → panic. Separately,
   `internal/controller/arduino.go`'s `connected`/`serial` fields have **no**
   locking at all, so `Disconnect()` (setting `serial = nil`) can race a concurrent
   `SendCommand` write → nil-pointer panic or corrupted serial writes.

---

## 🟠 High — real bugs, lower likelihood or narrower blast radius

7. **[FIXED] `YOLODetectionsToDynamicObstacles` mixes pixel and metre units.**
   `internal/detection/dynamic_obstacle.go:35-52` — world position is correctly
   converted to metres, but the obstacle `radius` is left in raw pixels and fed
   into a metres-based struct. If YOLO obstacle detection is ever re-enabled
   (config still supports it, code path still live via `feedYOLOObstacles`), every
   detected object becomes an obstacle tens-to-hundreds of metres wide, effectively
   blocking the whole planning grid. (User note: We originally intended to REMOVE YOLO entirely after the new background-based obstacle detection was implemented, so it MAY BE that the fix here is to finish the YOLO removal instaed. Investigate this option as well.)
   **Fix:** removed YOLO detection entirely (detector, config, obstacle
   conversion, `--self-test`/`--demo-yolo` modes, and all references) rather than
   patching the unit conversion — the code was already dead (foreground detector
   fully supersedes it) and self-documented as scheduled for removal.

8. **[FIXED] `Homography.SetFromValues` stomps the calibrated pixels-per-meter scale.**
   `internal/position/homography.go:285-298` calls `EstimateScale()`, which is a
   hardcoded stub (`h.PixelsPerMeter = 100.0`). The real value only survives today
   because `estimator.LoadCalibration` happens to re-apply it afterward from YAML —
   a coincidental double-write, not an enforced invariant. Any other caller, or a
   reordering, silently gets `100.0`.
   **Fix:** `SetFromValues` no longer touches `PixelsPerMeter` at all — it's a
   plain matrix setter now, not a calibration step. `EstimateScale()` is an
   explicit last-resort fallback callers opt into (`ComputeFromAprilTag`'s
   degenerate-corners branch, and `estimator.LoadCalibration` when a file has no
   `world_scale`). This also fixes a previously-dead-but-real bug in
   `Homography.Load`: it read the correct saved scale off disk and then
   immediately clobbered it back to 100.0 via the `SetFromValues` call that
   followed. Added `TestHomography_SetFromValues_PreservesScale` and
   `TestHomography_SaveLoad_RoundTrip` to cover both cases.

9. **`MatchThresh` config is effectively dead below 0.5 IoU.**
   `internal/tracking/hungarian.go` computes cost against the configured
   `MatchThresh` (default 0.3), but `bytetrack.go:157,202` re-gates the result with
   a hardcoded `< 0.5`. Any IoU in `[0.3, 0.5)` — exactly the range the config was
   supposed to accept — is rejected anyway, causing more track ID churn than
   intended regardless of what a user configures.

10. **`computeCollisionAvoidance` ignores goal direction, causes oscillation.**
    `internal/planning/local.go:201-214` — `desiredVel`, `combinedRadius`, and
    `timeHorizon` parameters are all unused; it unconditionally reverses at 80% max
    speed directly away from the obstacle regardless of penetration depth or goal,
    which will cause hunting/oscillation near any obstacle boundary.

11. **WebSocket connections leak on silent network death.**
    `internal/ui/webserver.go:227-352` — no `SetReadLimit`, `SetReadDeadline`,
    `SetPongHandler`, or ping ticker. A client that vanishes without a clean TCP
    close (sleep, NAT timeout) leaves its goroutine and `s.clients` entry alive
    indefinitely; `BroadcastOverlay` keeps trying to write to the dead connection
    every frame.

12. **`WebServer.Stop()` doesn't actually stop the HTTP server.**
    `internal/ui/webserver.go` — `Stop()` closes an unused `stopChan` and sets a
    flag, but the `*http.Server` from `Start()` is never stored on the struct or
    told to shut down (no `Shutdown()`/`Close()`). Graceful shutdown from
    `cmd/main.go` does not stop the HTTP/WS/MJPEG listener.

13. **A\* open-set has no decrease-key; violates heap invariant.**
    `internal/planning/astar.go:174-221` — when a cheaper path to an already-queued
    node is found, `gScore`/`cameFrom` are updated but the stale heap entry is
    never replaced, and `inOpenSet` does an O(n) linear scan per neighbor. Final
    paths are still correct, but the search degrades toward Dijkstra-with-dead-
    entries and can get very slow on sparse/large maps (default grid is
    2000×2000 cells — see #16).

14. **Divide-by-zero/NaN when two robots coincide.**
    `internal/planning/coordinator.go:190-199` — `adjustForConflict` divides by
    `dist` with only a `< 0.5` guard; if `dist == 0` (plausible after a tracking
    re-ID swap) this produces NaN that corrupts that robot's command. Lower
    priority since this code path is currently dead in production (see #17).

15. **`main()` shutdown path can double-`Stop()` and panic.**
    `cmd/main.go:1806-1817` — both a `defer rs.Stop()` and the SIGINT handler call
    `rs.Stop()`. `os.Exit(0)` in the signal handler happens to prevent the deferred
    call today, but `Stop()` itself isn't fully idempotent (unconditional
    `close()` calls), so this is fragile to future refactors.

16. **`AStarConfig.GridWidth`/`GridHeight` are metres, not cell counts, despite the name.**
    `internal/planning/astar.go` — `gridWidth := int(GridWidth / Resolution)`. With
    documented defaults (100, 0.05) this silently produces a 2000×2000-cell grid.
    A user "fixing" this by setting `GridWidth: 2000` thinking it means cells would
    produce a 40,000-cell-wide grid (1.6B cells).

17. **AprilTag family config has an inverted-looking condition.**
    `internal/detection/apriltag.go:77-79` — `if config.Family != "" { config.Family
    = "tag36h11" }` forces the stored family to a constant whenever one *is*
    configured, backwards from the apparent intent (default only when unset).
    Doesn't break tag decoding (a separate switch handles the real dictionary
    selection) but corrupts a diagnostic/UI field.

18. **Shadow-suppression "disable via zero" doesn't work.**
    `internal/detection/foreground_types.go:36-39` documents that leaving
    `ShadowAlphaMin`/`ShadowChromaMax` at `0` disables the gate — but both
    `config.go`'s `EffectiveForeground` and `foreground.go`'s
    `NewForegroundDetector` treat `0` as "unset" and coerce it back to the
    default. A user tuning the newest, least-tested feature against the real robot
    shadow cannot actually turn it off the documented way.

19. **`DetectionPipeline.tagDetector` nil-call landmine.**
    `internal/detection/pipeline.go:11-16` sets `tagDetector = nil` on constructor
    error but `Detect()` calls it unconditionally with no nil check. Currently
    unreachable (constructors never actually return an error today) but will panic
    the instant someone adds a real failure path — the equivalent YOLO path already
    handles this correctly two lines below.

---

## 🟡 Tech debt

- **`cmd/main.go` is a ~1950-line god file** — `RobotSystem` has ~35 fields covering
  camera, detection, tracking, planning, position, Arduino I/O, command queue, path
  execution, web server, calibration, heading smoothing, obstacles, and perf
  tracking. `Initialize()` (285 lines) wires 10+ subsystems via 9 inline callback
  closures. `ProcessFrame` and `ProcessDemoFrame` are ~80% duplicated (FPS
  smoothing, RGBA conversion, per-track world-position math, `BroadcastTracks`) —
  and demo mode silently never calls `AddRobot`/`computeTrackHeading`/
  `executeAutonomousControl`, so demo robots never get paths/headings, a divergence
  that isn't obvious from reading either function alone.
- **Dead second collision-avoidance/coordination system.** `Coordinator.
  ComputeCommands`/`ResolveConflicts`/`willCollide`/`adjustForConflict`/
  `AssignGoals` (`internal/planning/coordinator.go`) are never called from
  `cmd/main.go` — the real pipeline uses `PlanPath` + `ComputeVelocityWithDynamicObstacles`
  directly. This ~100-line parallel scheme (different hardcoded thresholds, O(n²)
  conflict resolution) is exercised only by unit tests and is a trap: it could be
  "fixed" without ever affecting production, or re-wired in without realizing it
  conflicts with the live VO logic (and reintroduces bug #14).
- **Three near-duplicate "compute velocity" entry points** in
  `internal/planning/local.go` (`ComputeVelocity`, `ComputeVelocityWithObstacles`,
  `ComputeVelocityToWaypoint`) — two of the three appear unused in production.
- **Duplicated default-filling for `ForegroundParams`** across
  `config.go:EffectiveForeground` and `foreground.go:NewForegroundDetector` — the
  comment literally admits "defaults must match" between two independently
  maintained lists, and this exact seam is where bug #18 happened.
- **Hand-rolled YAML string building** (fragile, unescaped edge cases) in at least
  two places: `internal/ui/webserver.go:1308-1327` (obstacles) and
  `internal/position/estimator.go:422-441` (`SaveObstacles`) — while calibration
  save in the same area correctly uses `yaml.Marshal`.
- **Config file paths that don't match reality**: `config/tracking_config.yaml`
  references `config/calibration_default.yaml` / `calibration_camera0.yaml`; actual
  files are `calibration_Camera_0.yaml` / `calibration_demo.yaml`. Either dead
  config or a latent bug.
- **`ControllerConfig.HeartbeatTimeout` is a silent no-op** — declared in config,
  never read anywhere; the real timeout is a hardcoded constant
  (`protocol.go:37`).
- **`CommandQueue.Enqueue` doesn't rate-limit** — despite the type's name and a
  configured interval, only the heartbeat re-send path is throttled; a fast caller
  can burst up to 10 raw writes back-to-back to the 9600-baud serial line.
- **Duplicated coordinate-transform logic in the frontend**:
  `ui/src/composables/useCanvas.ts` has `canvasToNatural` and
  `canvasToNaturalShared` with identical bodies, used inconsistently by different
  call sites (`robotStore.confirmDestination` vs. the canvas click handler) — a
  latent source of transform drift.
  Similarly, two independent sets of mouse listeners (Vue template bindings in
  `VideoOverlay.vue` + imperative `addEventListener` in `useCanvas.ts`) attach to
  the same canvas for different purposes, with no single source of truth for
  "what does a click do."
- **Repeated lock/unlock-then-relock pattern** in `internal/ui/webserver.go`'s
  obstacle handlers (`handleObstacleAdd`/`Delete`/`Update`) — same 4-line dance
  copy-pasted three times.
- **Obstacle-file path resolution logic spread across three places**
  (`webserver.go`, `main.go:loadStaticObstacles`, `config.go`'s
  `ObstaclesConfig.GetPath()`), each with its own fallback chain.
- **Magic numbers scattered and inconsistent across planning files** — e.g.
  `dist < 0.5` appears with different meanings in `local.go` and `coordinator.go`;
  avoidance-strength/nudge constants (`0.1`, `0.3`, `0.8`) aren't named or shared
  despite expressing related concepts.
- **Integer-division truncation** in `internal/tracking/kalman.go:176-179`
  (`bboxToCenter`) — divides before casting to float, losing up to 0.5px per
  measurement, compounding across frames into real-world position error.
- **No `dt` handling in the Kalman filter** — constant-velocity model assumes
  `dt = 1` implicitly; variable frame timing (drops, latency) silently produces
  wrong velocity/covariance estimates with no documentation of the assumption.

---

## 🟢 Over-complexity

- **`internal/controller/executor.go`'s `BearingToCommand`** (~160 lines) is a
  hand-tuned state machine (burst/wait/turn/nudge/spin-detection) with several
  interacting mutable fields and derived magic-number thresholds
  (`continuousTurnThresh := exitForwardThresh * 1.5`, etc.) — hard to verify by
  inspection; a good candidate for an explicit state-machine type with unit tests
  per state.
- **`WebServer` struct centralizes 4 unrelated responsibilities**: HTTP router,
  broadcast hub, obstacle store (with its own persistence), and calibration-state
  store — plus 13 injected `On*` callback fields used as ad-hoc dependency
  injection, each silently no-op'ing if `Initialize()` forgot to wire it.
- **`ui/src/composables/useCanvas.ts` is a 1264-line composable** mixing five
  concerns: rendering, a particle animation system, coordinate transforms,
  click/hit-testing, and its own `requestAnimationFrame`/`ResizeObserver`
  lifecycle. Prime candidate for splitting into focused composables.
- **Unclear ownership between `Planner`, `Coordinator`, and `LocalPlanner`** — two
  parallel local-avoidance pipelines exist (one live, one dead per the tech-debt
  section above); a newcomer must read all three files plus `main.go` to figure
  out which path actually runs.
- **`foregroundModel.step`** (~130 lines, `internal/detection/foreground_model.go:179-308`)
  inlines six per-pixel concerns in one dense loop. Defensible as a hot path, but
  it's why the newest shadow-suppression logic had to be threaded into the middle
  of an already-dense function rather than composed — each future "phase" gets
  riskier to add safely.
- **`ForegroundDetector` has accreted OpenCV lifecycle + command queuing + debug
  rendering + blob extraction + background persistence** into one struct after 4+
  incremental phases (persistence scheduling alone is 5 pieces of loosely-related
  state: `SaveNow`/`maybePersist`/`persistWG`/`persistMu`/`saveInFlight`) — a good
  candidate to extract as its own small, independently-testable type.

---

