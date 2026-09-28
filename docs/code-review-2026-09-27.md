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

9. **[FIXED] `MatchThresh` config is effectively dead below 0.5 IoU.**
   `internal/tracking/hungarian.go` computes cost against the configured
   `MatchThresh` (default 0.3), but `bytetrack.go:157,202` re-gates the result with
   a hardcoded `< 0.5`. Any IoU in `[0.3, 0.5)` — exactly the range the config was
   supposed to accept — is rejected anyway, causing more track ID churn than
   intended regardless of what a user configures.
   **Fix:** `ComputeIoUCost` already encodes the "below `matchThresh`" case as a
   sentinel cost (`NoMatchCost = 1.0`); `bytetrack.go`'s two match functions now
   check `costMatrix[i][j] < NoMatchCost` instead of a hardcoded `< 0.5`, so a
   configured `MatchThresh` anywhere in `(0, 1)` is honored. Added
   `TestByteTrack_Update_MatchThreshBelowHalfIsHonored`, which fails against the
   old hardcoded check and passes with the fix.

10. **[FIXED] `computeCollisionAvoidance` ignores goal direction, causes oscillation.**
    `internal/planning/local.go:201-214` — `desiredVel`, `combinedRadius`, and
    `timeHorizon` parameters are all unused; it unconditionally reverses at 80% max
    speed directly away from the obstacle regardless of penetration depth or goal,
    which will cause hunting/oscillation near any obstacle boundary.
    **Fix:** the function (renamed `computeCombinedCollisionAvoidance` by the
    fix for finding #4) now takes `desiredVel` and blends in its component
    tangential to the escape direction, weighted by how shallow the
    penetration is — deep penetration still leans on pure retreat, but
    shallow penetration lets the robot slide sideways toward its goal
    instead of backing straight out and being driven right back in once
    `desiredVel` resumes control. Added
    `TestLocalPlanner_CollisionAvoidance_BlendsGoalDirection`.

11. **[FIXED] WebSocket connections leak on silent network death.**
    `internal/ui/webserver.go:227-352` — no `SetReadLimit`, `SetReadDeadline`,
    `SetPongHandler`, or ping ticker. A client that vanishes without a clean TCP
    close (sleep, NAT timeout) leaves its goroutine and `s.clients` entry alive
    indefinitely; `BroadcastOverlay` keeps trying to write to the dead connection
    every frame.
    **Fix:** `handleWebSocket` now sets `SetReadLimit`, a `SetReadDeadline`
    (`wsPongWait`), and a `SetPongHandler` that renews it; a new `wsPinger`
    goroutine per connection sends a ping every `wsPingPeriod` (sharing the
    connection's write mutex with `BroadcastOverlay` so writes never
    interleave) and closes the connection if a ping write fails or the read
    deadline lapses with no pong, which unblocks `wsReader`'s `ReadMessage`
    and lets its existing cleanup remove the `s.clients` entry.
    `BroadcastOverlay` also now sets a `wsWriteWait` deadline on each send and
    closes any connection whose write fails, instead of silently ignoring the
    error. Added `TestWebServer_SilentlyDeadClientIsCleanedUp`, which connects
    a client that never reads (so it can never answer a ping) and asserts the
    server reclaims it once the shortened liveness timers elapse.

12. **[FIXED] `WebServer.Stop()` doesn't actually stop the HTTP server.**
    `internal/ui/webserver.go` — `Stop()` closes an unused `stopChan` and sets a
    flag, but the `*http.Server` from `Start()` is never stored on the struct or
    told to shut down (no `Shutdown()`/`Close()`). Graceful shutdown from
    `cmd/main.go` does not stop the HTTP/WS/MJPEG listener.
    **Fix:** `Start()` now builds the `*http.Server` once and stores it on the
    struct (guarded by a small mutex, since `Start`/`Stop` can be called from
    different goroutines); `Stop()` calls `srv.Shutdown(ctx)` with a 5s timeout
    instead of closing the unused `stopChan` (removed — nothing ever selected
    on it). Added `TestWebServer_StopShutsDownListener`, which starts a real
    listener, confirms it answers `/api/status`, calls `Stop()`, and asserts
    the port stops accepting connections.

13. **[FIXED] A\* open-set has no decrease-key; violates heap invariant.**
    `internal/planning/astar.go:174-221` — when a cheaper path to an already-queued
    node is found, `gScore`/`cameFrom` are updated but the stale heap entry is
    never replaced, and `inOpenSet` does an O(n) linear scan per neighbor. Final
    paths are still correct, but the search degrades toward Dijkstra-with-dead-
    entries and can get very slow on sparse/large maps (default grid is
    2000×2000 cells — see #16).
    **Fix:** switched to the standard lazy-deletion pattern for `container/heap`,
    which has no decrease-key: instead of checking `inOpenSet` and leaving a
    stale, worse-`G` copy of a cell sitting in the heap, every improved path now
    just pushes a fresh entry, and a popped node is skipped if its `G` no longer
    matches the best known `gScore` for that cell (i.e. it was superseded after
    being queued). This removes the O(n) `inOpenSet` scan entirely (deleted) and
    keeps the heap's priority order honest. Added
    `TestAStar_Plan_FindsOptimalCost`, which checks the returned path's total
    cost against the true 8-connected-grid optimum
    (`min(dx,dy)*sqrt(2) + abs(dx-dy)`).

14. **[FIXED] Divide-by-zero/NaN when two robots coincide.**
    `internal/planning/coordinator.go:190-199` — `adjustForConflict` divides by
    `dist` with only a `< 0.5` guard; if `dist == 0` (plausible after a tracking
    re-ID swap) this produces NaN that corrupts that robot's command. Lower
    priority since this code path is currently dead in production (see #17).
    **Fix:** added a `coincidentEpsilon` (1e-6) guard — when `dist` falls below
    it, the direction vector is not normalized (which would divide by ~0) but
    instead falls back to a deterministic escape direction that differs by
    robot ID, so the two coincident robots diverge instead of both nudging the
    same way. Added `TestCoordinator_AdjustForConflict_CoincidentRobotsNoNaN`
    and `TestCoordinator_AdjustForConflict_CoincidentRobotsDivergeDeterministically`.

15. **[FIXED] `main()` shutdown path can double-`Stop()` and panic.**
    `cmd/main.go:1806-1817` — both a `defer rs.Stop()` and the SIGINT handler call
    `rs.Stop()`. `os.Exit(0)` in the signal handler happens to prevent the deferred
    call today, but `Stop()` itself isn't fully idempotent (unconditional
    `close()` calls), so this is fragile to future refactors.
    **Fix:** added a `stopOnce sync.Once` field to `RobotSystem` and wrapped
    `Stop()`'s entire body in `rs.stopOnce.Do(...)`, matching the existing
    `watchdogStopOnce` pattern already used inside it. This makes "call `Stop()`
    more than once, even concurrently" an actual invariant rather than a
    coincidence of `os.Exit(0)` timing, so a future refactor (e.g. removing that
    `os.Exit`, or a shutdown path that returns normally while a SIGINT also
    arrives) can no longer double-close a channel or double-invoke shutdown on
    a subsystem. Added `TestRobotSystem_Stop_IsIdempotent`, which calls `Stop()`
    twice sequentially and then 20 times concurrently and fails (panics) without
    the fix.

16. **[FIXED] `AStarConfig.GridWidth`/`GridHeight` are metres, not cell counts, despite the name.**
    `internal/planning/astar.go` — `gridWidth := int(GridWidth / Resolution)`. With
    documented defaults (100, 0.05) this silently produces a 2000×2000-cell grid.
    A user "fixing" this by setting `GridWidth: 2000` thinking it means cells would
    produce a 40,000-cell-wide grid (1.6B cells).
    **Fix:** renamed the fields to `GridWidthMeters`/`GridHeightMeters` (plain
    `float64`, not `int`) so the name matches what the code has always done with
    them, and added a doc comment on `AStarConfig` spelling out the units and the
    2000×2000-cell default. No behavior change — these fields are never wired to
    `config/tracking_config.yaml` today (every production call site passes `nil`
    and gets the struct's own defaults), so this is a naming/clarity fix that
    forecloses the misconfiguration trap described above rather than a runtime
    bug fix. Updated `astar_test.go`/`planner_test.go` for the new field names.

17. **[FIXED] AprilTag family config has an inverted-looking condition.**
    `internal/detection/apriltag.go:77-79` — `if config.Family != "" { config.Family
    = "tag36h11" }` forces the stored family to a constant whenever one *is*
    configured, backwards from the apparent intent (default only when unset).
    Doesn't break tag decoding (a separate switch handles the real dictionary
    selection) but corrupts a diagnostic/UI field.
    **Fix:** flipped the condition to `if config.Family == ""`, so the stored
    `family` field (used only for diagnostics/UI, per the switch above which
    already handles the real dictionary selection independently) now defaults
    to `tag36h11` only when unset and otherwise preserves whatever family was
    configured. Added `TestNewAprilTagDetector_FamilyDefault`, which fails
    against the old inverted check and passes with the fix.

18. **[FIXED] Shadow-suppression "disable via zero" doesn't work.**
    `internal/detection/foreground_types.go:36-39` documents that leaving
    `ShadowAlphaMin`/`ShadowChromaMax` at `0` disables the gate — but both
    `config.go`'s `EffectiveForeground` and `foreground.go`'s
    `NewForegroundDetector` treat `0` as "unset" and coerce it back to the
    default. A user tuning the newest, least-tested feature against the real robot
    shadow cannot actually turn it off the documented way.
    **Fix:** `ForegroundConfig`'s three shadow fields are now `*float64` so
    `EffectiveForeground` can tell "unset in YAML" (nil, defaults) apart from
    "explicitly configured to 0" (kept as 0); `NewForegroundDetector` no longer
    coerces a zero shadow bound back to the tuned default; and
    `foregroundModel.step` now gates the `isShadowColor` call behind an
    explicit `shadowGateOn := shadowAlphaMin > 0 && shadowAlphaMax > 0 &&
    shadowChromaMax > 0` check, rather than relying on each bound's
    incidental (and, for `ShadowAlphaMin`, wrong-direction) effect on the
    scale/chroma math. Added `TestEffectiveForeground`'s new "explicit zero
    disables shadow suppression instead of defaulting" case and
    `TestForegroundModel_ShadowGateZeroDisablesIt`, both of which fail against
    the old behavior and pass with the fix.

19. **[FIXED] `DetectionPipeline.tagDetector` nil-call landmine.**
    `internal/detection/pipeline.go:11-16` sets `tagDetector = nil` on constructor
    error but `Detect()` calls it unconditionally with no nil check. Currently
    unreachable (constructors never actually return an error today) but will panic
    the instant someone adds a real failure path — the equivalent YOLO path already
    handles this correctly two lines below.
    **Fix:** `Detect()` and `DrawResults()` now nil-check `tagDetector` before
    calling it, degrading to "no tags detected"/"skip drawing tags" instead of a
    nil-pointer panic, matching the pattern already used for `obstacleDrawer`.
    Added a doc comment on `NewDetectionPipeline` explaining why the nil check
    exists despite the constructor never currently failing. Added
    `TestDetectionPipeline_Detect_NilTagDetectorDoesNotPanic` and
    `TestDetectionPipeline_DrawResults_NilTagDetectorDoesNotPanic`, which
    construct a pipeline with a nil `tagDetector` directly and fail (panic)
    against the old code.

---

## 🟡 Tech debt

- **[FIXED] `cmd/main.go` is a ~1950-line god file** — `RobotSystem` has ~35 fields covering
  camera, detection, tracking, planning, position, Arduino I/O, command queue, path
  execution, web server, calibration, heading smoothing, obstacles, and perf
  tracking. `Initialize()` (285 lines) wires 10+ subsystems via 9 inline callback
  closures. `ProcessFrame` and `ProcessDemoFrame` are ~80% duplicated (FPS
  smoothing, RGBA conversion, per-track world-position math, `BroadcastTracks`) —
  and demo mode silently never calls `AddRobot`/`computeTrackHeading`/
  `executeAutonomousControl`, so demo robots never get paths/headings, a divergence
  that isn't obvious from reading either function alone.
  **Investigation:** the struct actually has 47 fields (not ~35) and `Initialize()`
  registers 10 callbacks (not 9); the "~80% duplicated" claim was overstated —
  real, unextracted duplication was closer to 30% of each function (FPS smoothing,
  per-track pixel-radius/world-position math, and the stats/status/tags broadcast
  tail; "RGBA conversion" wasn't duplicated at all, only `ProcessFrame` does it).
  The demo-mode divergence, however, was confirmed exactly as described.
  **Fix:** split `RobotSystem`'s fields into 11 named sub-structs by concern
  (`cmd/robot_system_types.go`); split `Initialize()` into per-group `init<Group>()`
  methods called in the same order as before; extracted the three genuinely
  duplicated blocks into shared helpers (`updateFPS`, `updateTrackWorldPosition`,
  `broadcastFrameStats`) used by both `ProcessFrame` and `ProcessDemoFrame`; and
  fixed the demo-mode divergence by having `ProcessDemoFrame` also call
  `AddRobot`/`computeTrackHeading`/`executeAutonomousControl`, with a new guard so
  autonomous control refuses to run if demo mode is active with a real, connected
  Arduino. Added `cmd/demo_autonomy_test.go` (regression test that fails against
  the old code, plus a guard test). No behavior change other than the demo-mode
  fix itself; `./scripts/build.sh` and `./scripts/test.sh` pass throughout.
- **[FIXED] Dead second collision-avoidance/coordination system.** `Coordinator.
  ComputeCommands`/`ResolveConflicts`/`willCollide`/`adjustForConflict`/
  `AssignGoals` (`internal/planning/coordinator.go`) are never called from
  `cmd/main.go` — the real pipeline uses `PlanPath` + `ComputeVelocityWithDynamicObstacles`
  directly. This ~100-line parallel scheme (different hardcoded thresholds, O(n²)
  conflict resolution) is exercised only by unit tests and is a trap: it could be
  "fixed" without ever affecting production, or re-wired in without realizing it
  conflicts with the live VO logic (and reintroduces bug #14).
  **Investigation:** confirmed, and the dead chain runs one level deeper than
  described — `Planner.ComputeAllCommands`/`ResolveConflicts` (the only callers
  of the `Coordinator` methods above) were themselves never called from
  `cmd/main.go` either; `AssignGoals` had zero callers even in tests. The
  finding's own premise needed a correction, though: the live pipeline does
  **not** use `ComputeVelocityWithDynamicObstacles` — that function (and its
  sibling `ComputeVelocity`) also has zero callers anywhere. Since the
  bearing-based-steering rewrite (`f6edb54`), `cmd/main.go` drives robots via
  `PlanPath` → waypoints → `AdvancePastWaypoints`/`GetNextWaypoint` → bearing
  math → `controller.BearingToCommand` — none of the velocity-obstacle
  machinery is in the production path. Fixing that separate pair of dead
  `ComputeVelocity*` entry points is out of scope here; it's already tracked
  as its own tech-debt bullet below ("Three near-duplicate compute velocity
  entry points"). Also confirmed `Coordinator` itself is **not** entirely
  dead: its `robots`/`goals`/`obstacles` state and the plain accessor methods
  (`AddRobot`, `SetGoal`, `GetRobotState`, `UpdateRobots`, etc.) are `Planner`'s
  only storage for per-robot state and are exercised on every real request
  through `cmd/main.go`.
  **Fix:** removed only the dead priority-ordered conflict-resolution logic —
  `Coordinator.ComputeCommands`, `getRobotsByPriority`, `ResolveConflicts`,
  `willCollide`, `adjustForConflict`, `AssignGoals`, the `coincidentEpsilon`
  constant, and the `priorities`/`localPlanner`/`collisionDetector` fields that
  existed only to support it (`collisionDetector` was never even read once
  assigned). `Coordinator` is kept, now with a doc comment describing its
  narrower, accurate role as a shared per-robot state store. `NewCoordinator`
  no longer takes constructor args it didn't need for that role.
  `Planner.ComputeAllCommands`/`ResolveConflicts` (the dead wrappers) were
  removed too. Deleted `coordinator_test.go`, which tested only the
  now-removed `adjustForConflict` (the bug #14 NaN-guard regression tests) and
  had zero coverage of anything still in the codebase. Two integration tests
  in `internal/integration_test.go` that called the removed
  `Planner.ComputeAllCommands` were rewritten to exercise the actual live
  pipeline (`GetNextWaypoint` + bearing math + `BearingToCommand`) instead.
  `./scripts/build.sh` and `./scripts/test.sh --verbose` pass.
- **[FIXED] Three near-duplicate "compute velocity" entry points** in
  `internal/planning/local.go` (`ComputeVelocity`, `ComputeVelocityWithObstacles`,
  `ComputeVelocityToWaypoint`) — two of the three appear unused in production.
  **Investigation:** understated — repo-wide grep found **zero** callers
  (production or test) for all three, not two: `ComputeVelocityToWaypoint`
  wasn't even wrapped by `Planner`, and the two `Planner`-level wrappers
  around the other pair (`Planner.ComputeVelocity`,
  `Planner.ComputeVelocityWithDynamicObstacles`) were themselves dead too —
  `cmd/main.go`'s only obstacle-feeding call is
  `Planner.SetDynamicObstacles([]Obstacle)`, a different, still-live type
  used by A* planning, never these. `Coordinator.getOtherRobots` existed
  only to serve those two wrappers. This is the concrete instance of the
  "two parallel local-avoidance pipelines exist (one live, one dead)"
  over-complexity bullet below — this VO-entry-point layer is the dead one.
  **Fix:** removed all three `LocalPlanner` entry points, the orphaned
  `computeDesiredVelocity` helper they alone called, both dead `Planner`
  wrappers, and the now-unused `Coordinator.getOtherRobots`. Kept
  `applyVelocityObstacles` and everything it calls (`computeVO`,
  `velocityInVO`, `computeBestAvoidanceVelocity`,
  `generateCandidateVelocities`, `evaluateVelocity`,
  `computeCombinedCollisionAvoidance`) — this is real, tested logic
  (exercised directly by `internal/planning/local_test.go`'s regression
  tests for findings #4 and #10), just not currently wired to any entry
  point; added a doc comment on `applyVelocityObstacles` explaining that and
  pointing at the live bearing-based steering path instead. No behavior
  change; `./scripts/build.sh` and `./scripts/test.sh --verbose` pass.
- **[FIXED] Duplicated default-filling for `ForegroundParams`** across
  `config.go:EffectiveForeground` and `foreground.go:NewForegroundDetector` — the
  comment literally admits "defaults must match" between two independently
  maintained lists, and this exact seam is where bug #18 happened.
  **Investigation:** confirmed — both places hardcoded their own copy of the
  defaulting logic for the same six fields (`Scale`, `Threshold`,
  `DarkFactor`, `TauSec`, `WarmupSec`, `GuardFraction`); the two lists agreed
  today, but nothing enforced that beyond the comment. Every real call site
  (`cmd/foreground_glue.go`'s `newForegroundGlue`, fed from
  `EffectiveForeground()`) and every existing test already ran fully-defaulted
  params into `NewForegroundDetector`, so its own defaulting block was
  currently dead weight with no direct test of its own.
  **Fix:** extracted the six-field defaulting block into
  `detection.ForegroundParams.WithDefaults()` (`foreground_types.go`, next to
  `DefaultForegroundParams`, in the always-compiled file so it works in every
  build configuration including non-`gocv`). `NewForegroundDetector` now
  calls `p.WithDefaults()` instead of its own inline checks.
  `config.EffectiveForeground` now builds a `detection.ForegroundParams` from
  the raw config fields, calls `.WithDefaults()` on it, and reads the six
  values back out, instead of hardcoding a second copy of the same literals;
  the config-only fields with no `detection.ForegroundParams` counterpart
  (`MinSizeM`, `AppearMs`, `VanishMs`, `RobotMarginM`, `StaticMarginPx`,
  `MaxBlobs`, `PadM`, `PersistIntervalSec`) keep their own `orF`/`orI`
  defaulting untouched. The three shadow-suppression bounds
  (`ShadowAlphaMin`/`Max`, `ShadowChromaMax`) keep their existing nil-vs-zero
  `orFPtr` resolution in `config.go` exactly as-is — `WithDefaults` never
  touches them, by design, preserving the bug #18 fix (explicit zero still
  disables the shadow gate) by construction rather than by convention. There
  is now exactly one place that decides which fields get defaulted and how.
  No behavior change. Added `TestForegroundParams_WithDefaults`
  (`internal/detection/foreground_types_test.go`), closing the gap where this
  logic had no direct unit test; `TestEffectiveForeground` (including the bug
  #18 regression subtest) passes unchanged. `./scripts/build.sh` and
  `./scripts/test.sh --verbose` pass.
- **[FIXED] Hand-rolled YAML string building** (fragile, unescaped edge cases) in at
  least two places: `internal/ui/webserver.go:1308-1327` (obstacles) and
  `internal/position/estimator.go:422-441` (`SaveObstacles`) — while calibration
  save in the same area correctly uses `yaml.Marshal`.
  **Investigation:** confirmed — both functions built the obstacle YAML file
  by hand with `fmt.Sprintf`/string concatenation, one keyed on
  `planning.Obstacle` (`[2]float64` world corners), the other on
  `position.Obstacle` (`Point2D` world corners), each independently
  responsible for producing the exact key/nesting shape (`version`,
  `obstacles[].name`, `.pixels.top_left`/`.bottom_right`,
  `.world.top_left`/`.bottom_right`) that `PositionEstimator.LoadObstacles`
  expects when it parses the file generically via
  `yaml.Unmarshal(&map[string]interface{})`. `%q` happened to escape the name
  field correctly today, but nothing enforced that beyond the two hand-kept
  copies staying in sync — the same kind of seam that caused bug #18.
  **Fix:** both functions now build a small `yaml:"..."`-tagged
  `obstacleFile`/`obstacleFileEntry` struct pair (one copy per package, since
  the two callers use different domain `Obstacle` types) and call
  `yaml.Marshal`, matching the pattern `SaveCalibration` already used in the
  same file. On-disk keys/nesting are unchanged, so `LoadObstacles` needed no
  changes; world coordinates now marshal at full float64 precision instead of
  the old fixed 4-decimal format (cosmetic only). Added
  `TestSaveObstaclesToFile_WritesParsableYAML`
  (`internal/ui/obstacles_file_test.go`) and
  `TestSaveObstacles_RoundTripsThroughLoadObstacles`
  (`internal/position/estimator_test.go`), closing the gap where neither
  save path had a direct test. `./scripts/build.sh` and
  `./scripts/test.sh --verbose` pass.
- **[FIXED] Config file paths that don't match reality**: `config/tracking_config.yaml`
  references `config/calibration_default.yaml` / `calibration_camera0.yaml`; actual
  files are `calibration_Camera_0.yaml` / `calibration_demo.yaml`. Either dead
  config or a latent bug.
  **Investigation:** confirmed as dead config, not a latent bug. `Config`
  (`internal/config/config.go`) has no `calibration` field at all, so the
  top-level `calibration:` block in `tracking_config.yaml` was never parsed by
  anything — `yaml.Unmarshal` silently drops unknown keys. The real
  calibration path is computed at runtime from the camera's display name via
  `ui.GetCalibrationFilename` (`config/calibration_<sanitized-camera-name>.yaml`),
  which is why the actual files are named `calibration_Camera_0.yaml` /
  `calibration_demo.yaml`. The only place the wrong hardcoded names existed in
  Go was `cmd/main.go`'s `Initialize()`, as a fallback used only when no
  camera name is available at all (no camera opened and no camera config) —
  harmless today because `NewPositionEstimator` checks `FileExists` before
  loading and just starts uncalibrated if the file is missing.
  **Fix:** deleted the dead, wrong `calibration:` block from
  `tracking_config.yaml` instead of correcting its paths, since nothing reads
  it. Changed the `cmd/main.go` fallback to build its path via
  `ui.GetCalibrationFilename("default")` instead of a separately hardcoded
  string literal, so it can't drift from the real naming convention again, and
  added a comment explaining why that path deliberately doesn't exist on disk.
  No behavior change (the fallback path string is identical); moved the log
  line so it fires for both branches instead of only the camera-name one.
  `./scripts/build.sh` and `./scripts/test.sh --verbose` pass.
- **[FIXED] `ControllerConfig.HeartbeatTimeout` is a silent no-op** — declared in config,
  never read anywhere; the real timeout is a hardcoded constant
  (`protocol.go:37`).
  **Investigation:** confirmed — `config/tracking_config.yaml` actually sets
  `heartbeat_timeout: 0.5`, documented as "seconds before sending STOP if no
  command received," so this wasn't just a dead struct field but a setting
  presented to whoever edits that YAML as real and tunable while silently
  doing nothing; `queue.go`'s heartbeat ticker used the hardcoded
  `HeartbeatTimeoutMs` constant instead. Same shape as the sibling
  `CommandInterval` field, which sits right next to it and *is* correctly
  wired through `cmd/main.go` into `NewCommandQueue` — `HeartbeatTimeout` was
  clearly meant to follow that pattern and was never finished.
  **Fix:** `NewCommandQueue` now takes a third `heartbeatTimeoutMs` parameter
  (falling back to `HeartbeatTimeoutMs` when `<= 0`, mirroring the existing
  `intervalMs`/`CommandIntervalMs` fallback), and `cmd/main.go`'s
  `initArduinoAndQueue` reads `rs.cfg.Controller.HeartbeatTimeout` (seconds →
  ms) and passes it through, exactly like `CommandInterval`. Added doc
  comments on `ControllerConfig.CommandInterval`/`HeartbeatTimeout` spelling
  out the units and fallback. No behavior change with the shipped
  `tracking_config.yaml` (0.5s matches the old hardcoded default), but the
  setting is no longer ignored if changed. Added
  `TestNewCommandQueue_HeartbeatTimeoutDefault` and
  `TestNewCommandQueue_HeartbeatTimeoutOverride`, the latter failing against
  the old hardcoded-constant code and passing with the fix.
  `./scripts/build.sh` and `./scripts/test.sh --verbose` pass.
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

