# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

**Always use wrapper scripts** — they automatically set up required OpenCV environment variables (`OPENCV_DIR`, `LD_LIBRARY_PATH`, `PKG_CONFIG_PATH`). Do NOT run `go build` directly.

```bash
./scripts/build.sh           # Build Vue 3 UI + Go backend
./scripts/run.sh --demo      # Run demo mode (no camera required)
./scripts/run.sh             # Run with camera
./scripts/test.sh --verbose  # Run all tests (Go + Vue)
```

## Testing

```bash
# Run all Go tests
./scripts/test.sh --verbose

# Run a specific Go package
go test -v ./internal/planning/

# Run a single Go test function
go test -v -run TestFunctionName ./internal/package/

# Vue tests
cd ui && npm run test:run      # CI mode
cd ui && npm run test:coverage # With coverage
```

## Architecture

This is a multi-robot tracking and control system. Data flows through a pipeline:

**Camera → Detection → Tracking → Planning → Arduino Controller**

The `RobotSystem` struct in `cmd/main.go` owns and orchestrates all subsystems, grouped into 11 named sub-structs by concern (capture, detection, tracking, planning, obstacles, position, io, web, control, heading, stats — declared in `cmd/robot_system_types.go`; see the doc comment on `RobotSystem`). `Initialize()` wires each group up via its own `init<Group>()` method, in a fixed order. `ProcessFrame()` (real camera) and `ProcessDemoFrame()` (no camera) share their per-frame math (`updateFPS`, `updateTrackWorldPosition`, `broadcastFrameStats`) and both register confirmed tracks with the planner, compute headings, and run autonomous control — demo mode is intentionally kept behaviorally identical to the real-camera path here, so don't reintroduce a divergence between the two without also updating both.

### Key Packages

- **`internal/detection/`** — AprilTag (robot ID) detection, fused into `FusedDetection`s; obstacles come from the background-subtraction foreground detector, not this package
- **`internal/tracking/`** — ByteTrack multi-object tracker with Kalman filter; outputs `Track` objects with world positions
- **`internal/position/`** — Homography calibration maps pixel↔world coordinates; saved to `config/calibration_<camera>.yaml`
- **`internal/planning/`** — A\* global path planning; `Coordinator` is a shared per-robot position/velocity/goal state store used by `Planner` (not a second command/collision-avoidance system — that dead code was removed, see `docs/archived/code-review-2026-09-27.md`); `Planner` owns A* + waypoints only. Steering is bearing-based (`cmd/main.go` → `controller.BearingToCommand`); there is no local/velocity-obstacle avoidance layer (removed, see the same review doc)
- **`internal/controller/`** — Arduino serial at 9600 baud; single ASCII commands (lowercase f/b/l/r/s + `\r\n`, no robot address); `CommandQueue` + `PathExecutor`. `CommandQueue` re-sends the active command as a heartbeat but drops it (and sends Stop) if nobody re-`Enqueue`s it within `DefaultCommandTTL` (2 s) — a deadman for a dropped browser/network/frozen camera. The UI re-sends a held key every 250 ms; the autonomous loop enqueues every frame. A failed serial write closes the port and marks it disconnected; the queue's run loop then retries `Connect()` every `DefaultReconnectInterval` (2 s) — this also covers an Arduino that wasn't plugged in at startup — and on reconnect sends Stop and drops any stale active command rather than resuming it. `Stop()`/`EmergencyStop()` send a final Stop and refuse further movement writes (`sendMu` orders this against an in-flight write).
- **Single controlled robot (deliberate):** there is one serial line, one `CommandQueue`, one `PathExecutor`, and the UI has one destination. Only one robot may have a goal at a time (`OnDestinationSet` releases the others); other tracked robots are only observed. Real multi-robot control needs a controller (or an addressed protocol) per robot — build that when a second robot exists.
- **`internal/ui/`** — Gin HTTP server; MJPEG stream at `/stream`; WebSocket at `/ws` for real-time overlay; REST API for calibration/obstacles/goals
- **`ui/src/`** — Vue 3 + TypeScript frontend; Pinia stores (`robotStore`, `obstacleStore`, `uiStore`); canvas overlay renders tracks/paths; `composables/useCanvas.ts` is just the rAF/lifecycle orchestrator, with rendering, animation state, and mouse handling split into `composables/canvas/` and coordinate math in `utils/canvasTransform.ts`

### Configuration

`config/tracking_config.yaml` — cameras, robot definitions (tag IDs, sizes), detection thresholds, planning parameters, serial settings.

## Go Code Style

- Import groups: stdlib → third-party → internal, separated by blank lines
- Errors: `fmt.Errorf("context: %w", err)`; return early; define `ErrXxx` sentinel errors
- Use table-driven tests
