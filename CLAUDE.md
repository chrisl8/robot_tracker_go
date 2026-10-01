# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build, run and deploy

**Always use wrapper scripts** — they automatically set up required OpenCV environment variables (`OPENCV_DIR`, `LD_LIBRARY_PATH`, `PKG_CONFIG_PATH`). Do NOT run `go build` directly. Every Go file that touches OpenCV carries the `//go:build gocv` tag, and the scripts pass `-tags=gocv`; a build or `go vet` without it silently skips most of `cmd/` and `internal/ui/`.

```bash
./scripts/build.sh           # Build Vue 3 UI + Go backend, sign the binary, restart the service
./scripts/run.sh --demo      # Run demo mode (no camera required)
./scripts/run.sh             # Run with camera
./scripts/test.sh --verbose  # Run all tests and analysis (Vue + Go)
./scripts/service.sh status|start|stop|restart|log   # macOS LaunchAgent (wrapper for scripts/macos/service.sh)
```

**Building and deploying is encouraged here.** This is a hobby project: build, restart and leave the freshly built version running both while making changes and when finishing them, without asking first. `build.sh` restarts the `com.chrisl8.robot-tracker` LaunchAgent itself; `test.sh` stops it for the Playwright run and starts it again. After a deploy, confirm it came up (`pgrep -fl "robot_tracker "`, then `~/Library/Logs/robot-tracker.log`). The web UI is at <http://localhost:9086>.

Two things the service owns that matter when working on hardware:
- It holds the gamepad's serial port. **Stop the service before flashing or capturing on that port, and start it again afterwards.**
- A local build is signed with the Apple Development identity so macOS keeps the camera permission; SSH sessions cannot do that (see `scripts/macos/build.sh`).

## Testing

`./scripts/test.sh` runs, in order, and **aborts at the first failure**:
1. Vue: `npm outdated` (informational), ESLint, Prettier check, `vue-tsc` type check, `knip --strict` dead-code check, Vitest, then Playwright (`ui/tests/app.spec.ts`, which starts its own `--demo` on port 9086, hence the service stop).
2. Go: `golangci-lint` (`.golangci.yml`), `go vet`, `staticcheck`, `errcheck`, `gocyclo -over 60`, `gosec`, `govulncheck`, `deadcode` (informational), `nilaway`, then `go test -race` with coverage.

```bash
go test -v ./internal/planning/                       # one Go package
go test -v -run TestFunctionName ./internal/package/  # one Go test (add -tags=gocv for cmd/ and internal/ui)
cd ui && npm run test:run                             # Vue unit tests, CI mode
cd ui && npm run test:coverage                        # with coverage
```

Conventions:
- Table-driven Go tests. A new test must be shown capable of failing: break the code (a mutation) and confirm the test catches it.
- `internal/controller` tests use `recordingPort` (`queue_stop_test.go`), a fake serial port with queued `incoming` bytes and `failReads`/`failWrites` switches; `stubSerialOpen` makes `Connect()` hand out fake ports. `cmd/` tests build a rig with `newDemoAutonomyRig`.
- `nilaway` is strict: a test helper that returns a possibly-nil slice has failed it before. Return `make(...)`, not `var out []T`.
- The Arduino sketches have no automated tests; verify firmware changes on the hardware (see Firmware).
- Do not read or commit `scratch-pad.txt`: it is the developer's private notes (gitignored).

## Architecture

This is a multi-robot tracking and control system. Data flows through a pipeline:

**Camera → Detection → Tracking → Planning → Arduino Controller → (Bluetooth) → Robot**

The `RobotSystem` struct in `cmd/main.go` owns and orchestrates all subsystems, grouped into 11 named sub-structs by concern (capture, detection, tracking, planning, obstacles, position, io, web, control, heading, stats — declared in `cmd/robot_system_types.go`; see the doc comment on `RobotSystem`). `Initialize()` wires each group up via its own `init<Group>()` method, in a fixed order. `ProcessFrame()` (real camera) and `ProcessDemoFrame()` (no camera) share their per-frame math (`updateFPS`, `updateTrackWorldPosition`, `broadcastFrameStats`) and both register confirmed tracks with the planner, compute headings, and run autonomous control — demo mode is intentionally kept behaviorally identical to the real-camera path here, so don't reintroduce a divergence between the two without also updating both.

**File layout** (same-package files, so navigation is by concern rather than by one giant file): in `cmd/`, `main.go` holds the `RobotSystem` struct, constructor, flags and `main()`; `lifecycle.go` (Initialize/init*/Stop), `frame_loop.go` (camera and demo loops), `frame_processing.go` (`ProcessFrame`, `broadcastFrameStats`, robot-link handling), `autonomy.go`, `heading.go`, `control_state.go` (modes and e-stop), `web_callbacks.go`, `watchdog.go` (camera-stall watchdog and PERF log), `demo_frames.go`, `demo_mode.go`, `foreground_glue.go`. In `internal/ui/`, `webserver.go` holds the `WebServer` struct, callbacks, routes and Start/Stop; handlers and broadcasts live in `ws_hub.go`, `messages.go`, `status.go`, `broadcast.go`, `obstacles.go`, `calibration.go`, `destination.go`, `control.go`, `foreground.go`, `stream.go`, `filenames.go`; request validation is in `validation.go` and cross-site protection in `security.go`.

### Key Packages

- **`internal/detection/`** — AprilTag (robot ID) detection, fused into `FusedDetection`s; obstacles come from the background-subtraction foreground detector (`foreground*.go`, background model persisted to `config/foreground_bg_<camera>.bin`), not from AprilTags. `*_stub.go` files build without OpenCV.
- **`internal/camera/`** — `GetFrame()` returns the latest frame *without waiting*, so it can return the same one twice; consumers must skip a repeated non-zero `Frame.Seq` (the camera loop does). Processing repeats would hide a hung camera from the stall watchdog.
- **`internal/tracking/`** — ByteTrack multi-object tracker with Kalman filter; outputs `Track` objects with world positions. A confirmed track reports `TrackStateLost` after `LostAfterMissedFrames` frames without a detection (the tracker keeps it for `track_buffer` frames to re-acquire); autonomy only drives robots whose track is confirmed and stops a goal-holding robot that isn't.
- **`internal/position/`** — Homography calibration maps pixel↔world coordinates; saved to `config/calibration_<camera>.yaml` (timestamped `.bak-*` copies are kept beside it). `selfcal.go`/`calibration_target.go` implement the guided five-tag calibration.
- **`internal/planning/`** — A\* global path planning; `Coordinator` is a shared per-robot position/velocity/goal state store used by `Planner` (not a second command/collision-avoidance system — that dead code was removed, see `docs/archived/code-review-2026-09-27.md`); `Planner` owns A* + waypoints only. Paths are confined to the camera's view: `updatePlanningArea` (`cmd/frame_processing.go`, every frame) projects the frame corners to the floor via `PositionEstimator.VisibleWorldCorners` and gives the planner an `Area` (`area.go`); A* treats cells closer than the robot margin (radius + 0.12 m) to an edge as blocked and refuses a goal outside it, because a robot whose tag leaves the frame is lost. With no calibration the planner is unrestricted. Steering is bearing-based (`cmd/main.go` → `controller.BearingToCommand`); there is no local/velocity-obstacle avoidance layer (removed, see the same review doc)
- **`internal/controller/`** — Arduino serial at 9600 baud (`auto` port only considers USB-serial-looking names; `Connect()` reports connected only after the board's 2 s reset; `controller.enabled: false` turns off the queue's reconnect loop via `SetAutoReconnect`); single ASCII commands (lowercase f/b/l/r/s + `\r\n`, no robot address); `CommandQueue` + `PathExecutor`. `CommandQueue` re-sends the active command as a heartbeat but drops it (and sends Stop) if nobody re-`Enqueue`s it within `DefaultCommandTTL` (2 s) — a deadman for a dropped browser/network/frozen camera. The UI re-sends a held key every 250 ms; the autonomous loop enqueues every frame. A failed serial write closes the port and marks it disconnected; the queue's run loop then retries `Connect()` every `DefaultReconnectInterval` (2 s) — this also covers an Arduino that wasn't plugged in at startup — and on reconnect sends Stop and drops any stale active command rather than resuming it. `Stop()`/`EmergencyStop()` send a final Stop and refuse further movement writes (`sendMu` orders this against an in-flight write). **Reading:** `Connect()` starts `readLoop` (`link.go`) on the port; a read error drops the connection just like a failed write, so an unplug while idle is noticed (a stale reader never tears down a newer connection: it checks `c.serial == sp`).
- **Single controlled robot (deliberate):** there is one serial line, one `CommandQueue`, one `PathExecutor`, and the UI has one destination. Only one robot may have a goal at a time (`OnDestinationSet` releases the others); other tracked robots are only observed. Real multi-robot control needs a controller (or an addressed protocol) per robot — build that when a second robot exists.
- **`internal/ui/`** — Gin HTTP server; MJPEG stream at `/stream`; WebSocket at `/ws` for real-time overlay; REST API (`/api/command`, `/api/destination`, `/api/status`, `/api/mode`, `/api/emergency-stop`, `/api/clear-emergency-stop`, `/api/control-state`, `/api/obstacles*`, `/api/calibration/*`, `/api/foreground/*`). The server has no authentication and drives a physical robot, so `security.go` rejects cross-site browser requests (Origin check) and caps request bodies; `validation.go` bounds every request. The built UI is embedded via `go:embed` from `internal/ui/static` (Vite's `outDir`).
- **`ui/src/`** — Vue 3 + TypeScript frontend; Pinia stores (`robotStore`, `obstacleStore`, `tempObstacleStore`, `uiStore`, `fpsHealthStore`); canvas overlay renders tracks/paths; `composables/useCanvas.ts` is just the rAF/lifecycle orchestrator, with rendering, animation state, and mouse handling split into `composables/canvas/` and coordinate math in `utils/canvasTransform.ts`. `utils/driveKeys.ts` holds the keyboard drive-key logic. `npm run dev` serves on 5173 and proxies `/api` and `/ws` to the Go server on 9086.

### Control modes, safety layers and goal release

- Control mode is `hold` (default), `manual` or `autonomous` (`cmd/control_state.go`); leaving manual or autonomous clears the active command and stops the robot. Emergency stop halts everything until cleared.
- Several independent deadmen stand between a dropped link and a walking robot: the UI re-sends held keys every 250 ms; `CommandQueue` drops a command nobody re-sends within 2 s; the gamepad firmware reverts to its own buttons after `SERIAL_TIMEOUT_MS` (500 ms) of no Mac commands; the robot firmware stands still after 1 s without a valid packet. The camera watchdog (`watchdog.go`) broadcasts status and halts the robot once per stall when frames stop, because autonomy runs from the frame loop.
- **Goals are released, with a Stop, in two cases:** recalibration (the goal was in the old world frame) and the robot going silent (see below). `releaseAllGoals(reason)` in `cmd/web_callbacks.go` does it. A switched-off robot is still visible to the camera, so without the release autonomy would keep commanding it and it would walk toward the old goal when powered back on.

### Robot link (is the robot itself answering?)

`IsConnected()` only says the USB link to the gamepad Arduino is up. Whether the *robot* is powered and in radio range comes from a heartbeat:
- The gamepad firmware appends an `H` command to one transmit frame per second. The robot answers over Bluetooth; the gamepad validates the checksum and prints `#R=<uptime s>,<servos detached 0/1>,<mode char as number>` (or `#RBAD:<reason>`) to the Mac. Other robot bytes are passed through unchanged.
- `internal/controller/link.go` parses those lines. `RobotLink` is `unknown` (not connected, or under `RobotSilentAfter` = 3 s since connecting without a reply), `alive`, or `silent` (no reply for 3 s). `RobotInfo(now)` adds `Servos` (`asleep`/`awake`), `Mode` and `Reboots`. It takes only `linkMu`, never the serial `mu`, so a stalled write cannot freeze the status loop (lock order: `mu`, then `linkMu`).
- **Reboot detection:** the robot's 16-bit uptime going backwards counts as a restart (a brown-out or crash, usually a weak battery; there is no battery monitor). The counter wrapping at ~18 h (`uptimeWrapFloor`/`uptimeWrapCeil`) is not a reboot, the first heartbeat after (re)connecting is only a baseline, and the count is cumulative per process.
- Wiring: `cmd/frame_processing.go` `updateRobotLink` → `applyRobotLink` (logs each change, releases goals on the transition to `silent`) → `WebServer.SetRobotInfo` → status message fields `robotLink`, `robotServos`, `robotMode`, `robotReboots` (also in `/api/status`). The UI shows a "Robot" row plus Servos/Restarts rows (`CommStatus.vue`) and, in `robotStore`, a once-per-outage error toast, a recovery toast, and a warning toast when `robotReboots` grows (baselined so opening the page does not announce old restarts). The UI does not blame the robot when the Arduino itself is disconnected.

## Firmware (`Arduino/`)

Read `Arduino/README.md` for the full record; the essentials:

```
Mac --USB serial 9600--> gamepad (Arduino Nano) --Bluetooth HC-05, 9600--> robot (Vorpal hexapod, Nano + PCA9685 servo driver)
```

- `Vorpal-Hexapod-Gamepad/` is upstream's gamepad sketch, modified: a serial bridge (`f/b/l/r/s` from the Mac become gamepad dpad commands), SD-card code removed, the `H` poll and reply validation. It sends a control frame every 100 ms.
- `Vorpal-Hexapod-Robot/` is upstream `#RV3r1c` (commit `c4103c0` of github.com/vorpalrobotics/VorpalHexapod) plus: the `H` heartbeat command (`sendHeartbeat()`; reply `V1`, length 4, uptime seconds, servos-detached flag, mode char, checksum — it does not read the ultrasonic sensor and does not reset the sleep timer, so servos still power down after 5 s idle, unlike upstream's `S`), and `BLUETOOTH_BAUD 9600`. Licence CC BY-NC-SA 4.0 (Vorpal Robotics, LLC); keep the headers.
- **The radios run at 9600, not upstream's 38400.** A robot sketch built with 38400 receives only garbage: its USB serial floods with `F:n BTER:` and `BADCHR:` lines and `LOS` climbs forever, while the gamepad shows no `#R=`. The originally flashed robot firmware had this change and was lost because it was overwritten without a backup.
- **Before overwriting any board, dump its flash and EEPROM** (`avrdude … -U flash:r:backup.hex:i`, see the README for the command). The robot's servo trims (EEPROM addresses 1–12, marker `'V'` at 0) are written down in `Arduino/robot_trims.txt`; the gamepad's EEPROM is in `Arduino/gamepad_eeprom.txt` (only the `HC05_pad` byte matters and it is unset, so padding is off).
- Build and flash with `arduino-cli` (installed via Homebrew): `arduino-cli compile|upload --fqbn arduino:avr:nano:cpu=atmega328old …`. Both boards need the **old bootloader** setting. The robot needs `Adafruit PWM Servo Driver Library` **2.0.0** (3.x makes even unmodified upstream too big) and is ~97% full of program space, so check the size after any robot change. Flash the **robot before the gamepad**: an unmodified robot treats `H` as a bad command.
- Ports (FTDI): gamepad `/dev/cu.usbserial-BG01OQ2N`, robot `/dev/cu.usbserial-BG01OQ2Z`. The names differ by one letter, so **identify a board by its boot banner (`#GV3r1c-…` or `#RV3r1c-…`), never by name alone**. The robot's USB adapter is only plugged in while flashing or debugging, and opening its port reboots it and briefly drops the radio link. `scripts/tools/serial_capture.py <port> <file>` is a dependency-free timestamped logger (stop the service first for the gamepad port).
- The robot has no speaker (its error beeps are unobservable) and no battery monitor (`A6`/`A7` are grip-arm current sensors, `A3` is unused). It resets its own Bluetooth after 15 s without a valid packet, so a long gamepad reflash can leave the link needing a robot power cycle.
- `.ino` files use **CRLF** line endings; preserve them when editing (edit as bytes, or convert and convert back), or the diff shows every line changed.

## Configuration

`config/tracking_config.yaml` — `cameras`, `april_tags`, `foreground`, `tracking`, `position`, `processing` (frame-rate cap), `controller` (serial port/baud, command interval, heartbeat timeout), `robots` (tag IDs, sizes, `heading_offset_degrees`), `obstacles` (file), `path_execution`. The `path_execution` values (`spin_threshold_deg`, `burst_frames`, `max_wait_frames`) were tuned empirically against AprilTag heading noise; the comments in the file say what broke at other values, so read them before changing anything, and do not add heading smoothing. Static obstacles live in `config/obstacles_<camera>.yaml` (per-camera name, from `GetObstaclesFilename`). The `foreground` section controls the detector that finds temporary obstacles; with `apply_to_planner: false` (the current setting) it runs in **shadow mode**: detections are shown in the UI but the planner does not steer around them, until it is turned on in the config or the UI toggle.

## Debug logging and logs

`utils.Debugf` (per-frame steering, path and track detail) is **off by default**: it is emitted for every frame, so leaving it on floods the log and slows the loop. Turn it on with `--debug` or `ROBOT_TRACKER_DEBUG=1` (the env var lets the LaunchAgent enable it without editing scripts). Normal `utils.Log`/`Logf` output is unaffected.

The service logs to `~/Library/Logs/robot-tracker.log` (`--log-file`, rotated to `.log.1`–`.log.3`, set up in `internal/utils/logger.go`). Useful lines: `PERF …` every few seconds (frame rate, timings, CPU, control mode), `Robot link: "a" -> "b"`, `Robot rebooted …`, `Arduino read failed …` / `Arduino reconnect …`. Other flags: `--demo`, `--config`, `--list-ports`, `--list-cameras`, `--test-camera N`, `--quiet`.

## Documentation map

`README.md` (user-facing setup), `Arduino/README.md` (firmware), `docs/archived/` (design and migration history, including the numbered code-review records, which say what was fixed and what is left open), `BUGS.md` (open bugs and debt; resolved ones move to `docs/archived/ARCHIVED_BUGS.md`), `PLAN.md` (implementation summary), `diagnostics/` (GoCV/OpenCV compatibility probes, not part of the app).

## Go Code Style

- Import groups: stdlib → third-party → internal, separated by blank lines
- Errors: `fmt.Errorf("context: %w", err)`; return early; define `ErrXxx` sentinel errors
- Use table-driven tests

## Vue / TypeScript style

Prettier and ESLint are enforced by `test.sh` (`npm run format` / `npm run lint` fix them), `knip --strict` fails on unused exports, and `vue-tsc` type-checks the tests too. Status and API shapes live in `ui/src/types/api.ts`; keep them in step with `internal/ui/messages.go` (new fields optional on the TypeScript side so an older backend still works).
