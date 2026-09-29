# Second-Pass Code Review — robot_tracker_go

## Context

A second full review, done after every finding of the first
(`code-review-2026-09-27.md`, 38 items) had been marked fixed. The question asked was
whether the first pass (A) glossed over things, (B) didn't really fix things, or (C) was
too big to be thorough. Answer: mostly **A** (the physical-safety paths and algorithm
correctness were thin), some **B** (a few fixes were cosmetic or incomplete), little **C**.

Three read-only passes covered (1) cmd/controller/config, (2) detection/tracking/
planning/position/camera, (3) the web server and Vue UI. Each finding below was then
reproduced or re-read before fixing, and each fix has a regression test that was checked
to fail on the old behavior where that was possible (by stashing the fix, or by
mutating the fix).

All of it is committed on `main` (commits `1bfaedf` … `eec7ff8`, 20 commits).

---

## Safety: the robot keeps driving

The first review fixed the stop-on-key-release bug but never asked what happens when the
*operator or a component disappears*. The queue re-sent the last command forever.

| Finding | Fix | Commit |
|---|---|---|
| Alt-tab / hidden tab while holding a key: `keyup` never arrives | UI resets keys and sends Stop on window blur and tab hide | `1bfaedf` |
| Browser or network dies mid-drive: nothing stops the robot | **Deadman**: `CommandQueue` drops the active command and sends Stop if not re-enqueued within 2 s; the UI re-sends a held key every 250 ms; autonomous mode enqueues every frame | `e3f610e` |
| Camera stalls: watchdog only told the UI; autonomy runs from the frame loop so nothing stopped the robot | Watchdog calls `HaltMotion()` once per stall | `ef1aac8` |
| Shutdown never sent Stop; a leftover test block enqueued F/L/R/S on the live robot at exit | Removed the block; `Stop()` sends a final Stop | `1bfaedf` |
| E-stop could be followed by a buffered Forward (random `select`) | `halt()` marks the queue stopped first; `sendCommand` refuses non-Stop after that; `sendMu` orders an in-flight write against the final Stop | `1bfaedf`, `10b6f66` |
| Serial write error left the port open and control dead until restart | Failed write closes the port; the queue reconnects every 2 s (also covers an Arduino unplugged at boot), then sends Stop and drops the stale command | `10b6f66` |
| `Stop()` raced the frame loop (`SaveNow`/`Close` on live Mats; plain-bool `cameraRunning`) | `frameMu` + `stopped`; atomic flag; camera `Stop()` idempotent and waits (bounded) for the read loop | `ef1aac8` |
| Failed Stop / E-Stop clicks were silent | Error toasts | `1bfaedf` |
| Multi-robot control was illusory (one queue/executor/serial line; a path-less robot cleared another robot's command) | **Single controlled robot**, by decision: setting a goal releases other robots' goals; queue cleared only when no robot has a path | `e3f610e` |

## Correctness bugs

- **Kalman**: covariance predict was `F·P` instead of `F·P·Fᵀ`, so the velocity state never
  left 0 and fast robots lost their track match. (`1bfaedf`)
- **ByteTrack**: a tag ID is now authoritative for association (a tagged detection goes to
  the track with that tag; IoU never pairs different tags); the low-confidence pass only
  touches tracks not already updated this frame and honors `MatchThresh`; deterministic
  IDs and order; `Reset()` clears the stale timestamp. `Track.Age` counted twice per
  matched frame. (`5fce7a5`, `927d9b6`)
- **Planner**: a second `SimplifyPath` after validation undid the validation (paths came
  within 7.7 cm of obstacles against a 15 cm margin); `AdvancePastWaypoints` skipped
  detour corners straight into walls (now only skips when the line to the next waypoint is
  clear). (`5fce7a5`)
- **A\***: floor vs. truncation for negative start/goal cells; waypoints on cell corners
  instead of centers; diagonal corner-cutting between blocked cells; `start == goal` was a
  failure. (`927d9b6`)
- **Position**: autonomy re-projected the bbox and ignored `center_offset`; now uses
  `track.WorldPos`. (`5fce7a5`)
- **Panics**: `drawLine` divided by zero on a zero-length segment (two copies);
  `Planner.RemoveObstacle` on empty; camera `Stop()` twice; nil planner in the no-config
  demo fallback. (`1bfaedf`, `ef1aac8`, `5fce7a5`)
- **Data race missed by review 1**: `DetectionPipeline.SetObstacles` (HTTP goroutine)
  vs `DrawResults` (frame loop) shared a slice with no lock. (`5f4443a`)
- **Demo/real divergence**: `ProcessDemoFrame` never told the position estimator the
  frame size, silently disabling the API's range checks in demo mode. (`c06908a`)

## Web layer

- **Security**: wildcard CORS + always-true WebSocket origin + JSON binding that ignored
  Content-Type meant any web page open on the LAN could POST drive commands. Now:
  cross-origin state-changing requests get 403, bodies must be `application/json` and
  ≤ 1 MiB, WebSocket origin is checked, server read/idle timeouts. (`3b8ca6b`)
- **Validation**: unknown commands rejected; destinations require `robot_id`, `x`, `y`
  (a missing field silently meant robot 0 / pixel (0,0)), a configured robot, and a point
  inside the frame; a refused destination is neither stored nor broadcast (the UI used to
  show goals the planner never got); obstacle boxes are range-checked; PUT/DELETE of an
  unknown obstacle is 404 and doesn't dirty state; update is copy-on-write; calibration tag
  count capped; unique obstacle names. (`70f0ad7`, `c06908a`, `3b8ca6b`)
- **Stale state**: mode/e-stop changes are broadcast to every tab; state is re-read on
  reconnect; failed mode changes and goal clears show the reason; cancelling the
  calibration wizard no longer makes a working calibration look lost; the calibration
  broadcast carries `resolutionMismatch`. (`70f0ad7`)
- **UI**: heading `0` and tag `0` were hidden by falsy checks; toast IDs collided within a
  millisecond; `useWebSocket.disconnect()` let an unmounted app reconnect forever;
  obstacle drawing clamps to the frame. (`70f0ad7`)
- Obstacle delete showed a false failure toast every time. (`1bfaedf`)

## Tech debt

- **Dead code sweep**: `deadcode -tags=gocv ./...` findings went from **118 to 4** (the four
  are deliberate test seams). About 4,000 lines net removed, including the whole
  robot-velocity chain (computed, never read), `CollisionDetector` (collapsed into one
  `clearance()` function), unused USB/IP/video cameras, `SerialProtocol` (a duplicate of the
  real wire encoding whose tests covered the duplicate; now one `encodeCommand()` used by
  production), and a non-test file whose fuzz targets never ran. (`5e2b1ba`, `5f4443a`)
- **Config**: dead keys and struct fields removed, so the file no longer implies behavior
  that doesn't exist (`position.smoothing: true` did nothing); fixed the wrong
  `heartbeat_timeout` comment. (`0099756`)
- **Frontend**: duplicate coordinate helpers, unused types/exports, tracked
  `package.json.backup`, REST bodies typed against the API contract, `knip` no longer
  hides `types/api.ts` and `types/obstacle.ts`. (`0099756`)
- **Debug logging** was unconditional and per-frame; now off unless `--debug` or
  `ROBOT_TRACKER_DEBUG=1`. (`eec7ff8`)

## Tests that didn't test

- `canvasRegression.test.ts` "render" tests called `useCanvas` outside a component; it only
  gets its 2D context in `onMounted`, so `render()` returned on its first line and
  `not.toThrow()` proved nothing. The drawing-box test also never enabled drawing mode.
  Replaced with a mounted harness that asserts what is drawn (each assertion checked by
  mutating the renderer). (`153be8c`)
- Log-only `internal/integration_test.go`, tautological `canvasScaling` and
  `types/__tests__`, struct-field round-trip tests: removed or replaced.
- First HTTP handler tests in the repo, first tests for `internal/utils`, and Playwright
  destination tests that no longer depend on run order or on a leftover git-ignored
  `calibration_demo.yaml`.
- Several of *my own* new tests turned out vacuous on first check (the tag-cap test passed
  without the cap; the heading-0 test matched text elsewhere in the panel). Mutation and
  old-code runs caught them. The lesson for this repo: a green check is not evidence until
  the test has been seen to fail.

## Structure

`cmd/main.go` 1,967 → 187 lines and `internal/ui/webserver.go` 1,688 → 258, split by
concern with a parser-based tool that moves declarations and prunes imports. Verified the
set of declarations is identical before and after (60 and 107). `main()` is decomposed
into `resolveDemoMode` / `runCameraLoop` / `enterDemoMode` / `runDemoLoop`. (`c219d1c`)
File layout is described in `CLAUDE.md`.

## Dependencies (all languages)

- **npm**: everything at latest; `npm audit` **22 → 0** vulnerabilities. Major bumps: Vite 8,
  Vitest 5, ESLint 10, Pinia 4, Knip 6, `@vueuse/core` 15. Removed six unused packages and
  the deprecated `lucide-vue-next` (now `@lucide/vue`). Held: **TypeScript at ~6.0.3**,
  because `typescript-eslint` declares `typescript <6.1.0`.
- **Go**: `gin` 1.12, `serial` 1.8 and ~45 indirect updates; the `go` directive rose from
  1.24.0 to 1.26.0 (an updated dependency requires it). `govulncheck`: 0 affecting the code.
  Analysis tools rebuilt at latest. `ui/go.mod` is a stub that keeps Go tooling out of
  `ui/node_modules` (the newer `flatted` ships Go files).
- **Held: OpenCV 4.13** (Homebrew's `opencv` is now 5.0.0). gocv v0.43.0 targets 4.x; OpenCV 5
  support is an open community PR (hybridgroup/gocv #1371, tracking #1370) that upstream
  says needs OpenCV 5.1 on macOS. See "Standing constraints".
- Removed a stray root `package.json`/lockfile; left `diagnostics/` (no Go source) alone.

## Standing constraints and known limits

1. **OpenCV chain is Homebrew-pinned** (`opencv` + its 101 dependencies). `libopencv_dnn`
   loads 78 abseil libraries by versioned name, so upgrading `abseil` (or `opencv`) breaks the
   binary at launch. This also blocks `brew upgrade node` (Homebrew refuses, by design).
   Unpin (`brew list --pinned | xargs brew unpin`) only when gocv supports OpenCV 5.
2. **A stalled serial write can still block the E-stop.** The Go serial library has no write
   timeout. The rest of the system no longer freezes behind it, but a truly hung USB write
   needs an unplug.
3. **One controlled robot** by design; the wire protocol has no robot address. Multi-robot
   needs a controller (or an addressed protocol) per robot.
4. The demo registers robots 1–3 as configured robots; a robot must be configured to be
   given a goal.
5. Node comes from Homebrew here (not fnm) and is only a build/test tool: the UI is embedded
   into the Go binary and nothing needs Node at runtime.

## Not verified on hardware

The safety changes (deadman, stall halt, shutdown Stop, serial reconnect) are covered by unit,
race and demo tests, but the serial path only ever ran against a fake port. Confirm on the
robot: hold a key then alt-tab; close the tab mid-drive; unplug the camera during autonomous
driving; Ctrl+C mid-drive; unplug and replug the Arduino (it must reconnect and *not* resume
its old command).

## Remaining minor debt

- `ClassName`/`ClassID` legacy fields, and a duplicate `Quad` type in `detection` and
  `planning`.
- Unused `detection.AprilTagConfig` fields (`quadSigma`, `decimate`, ...).
- `tracking.ByteTrackConfig.MOT20` and `FrameRate` are set but never read.
