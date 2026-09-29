# Third-Pass Code Review — robot_tracker_go

## Context

A third full review, after the second (`code-review-2026-09-28-second-pass.md`) had found
that the first missed a lot. This one found real problems again, mostly one family the
earlier passes did not ask about: **what happens when an input goes quiet or stale**
(a tag, the camera, a calibration file, a browser). The second pass covered a component
*disappearing*; this pass covered it *lying* by going stale.

Everything below was read, and where marked *reproduced* was demonstrated with a throwaway
test on the old code before fixing. Each fix has a regression test; the ones for the
safety items were seen to fail when the fix was removed (mutation) or on the old code.
`go vet`, `staticcheck`, `errcheck`, `golangci-lint`, `nilaway`, `govulncheck` clean; full
Go suite with `-race`, Vue and Playwright green.

## Safety: the robot moves when it shouldn't

| Finding | Fix |
|---|---|
| **A lost tag stayed "confirmed".** `TrackStateLost` was never assigned, so a robot whose tag vanished stayed confirmed for `track_buffer` frames (~2 s at 15 fps) and autonomy drove it from a frozen position and cached heading. | Tracker reports `Lost` after `LostAfterMissedFrames` (3) frames without a detection (re-confirms instantly on re-acquisition). Autonomy also **stops any robot that has a goal but no confirmed track this frame**. |
| **A hung camera looked healthy.** `GetFrame()` returned the cached last frame with no sequence number; if `Read` blocked, the loop reprocessed the same image at full rate, kept `lastFrameNanos` fresh, and the stall watchdog never fired. Slow cameras also had duplicate frames processed, skewing the frame-counted steering. | `Frame.Seq`; the camera loop skips a repeated frame, so a hang produces no frames and the watchdog halts the robot. |
| **Clearing the e-stop could send a stale Forward** (*reproduced*: `"sff"`). `Enqueue` set the active command on a halted queue and `Start()` did not clear it. | Queue has a `halted` state that drops movement; `Start()` clears any active command. |
| **A held D-pad button survived window blur / tab hide / leaving Pilot mode**, and the 250 ms refresh timer then defeated the 2 s deadman. Touch had no handlers at all. | D-pad releases (with Stop) on blur and hidden; mode/e-stop change forgets the held button; touch events added. |
| **Shutdown stopped the robot last**, after the frame wait, the blocking background save and the camera stop. | `Stop()` stops the queue first. |
| **`controller.enabled: false` was ignored** by the queue's reconnect loop, and `port: auto` fell back to *any* `/dev/cu.*` (e.g. `Bluetooth-Incoming-Port`), reporting a connected Arduino and writing drive commands into it. | `SetAutoReconnect(enabled)`; auto-detect no longer falls back to unrelated ports (`--list-ports` still lists everything). |
| `Connect()` marked the port connected before the board's 2 s reset finished, so a Stop sent then was lost. | Connected only after the startup delay; a `connecting` flag prevents a double open. |
| `PathExecutor` kept its burst/wait phase across runs, so a new run could start with stale turns or drive-while-waiting. | `Reset()`, called on any frame with no steering. |

## Correctness

- **Goal set before the robot is tracked was planned from the world origin** (*reproduced*):
  `GetPathsWithGoals` (called every 10 frames) planned from `(0,0)` and stored it, and
  `AddRobot` kept it. Now it only plans from a position the robot was actually seen at.
- **A missing/invalid homography became "calibrated"**: `LoadCalibration` substituted an
  identity transform (pixels = 1/100 m). Now it is an `ErrInvalidCalibration` (missing,
  incomplete, non-finite, singular) and leaves the previous state untouched. The old
  `HasTransformation` check also rejected legitimate small-scale matrices; removed.
  An integer `world_scale` was silently ignored; fixed.
- **Obstacle world coordinates went stale after recalibration** (and after loading a file
  saved under an older one): they are now recomputed from the pixel boxes
  (`RecomputeObstacleWorld`) on calibration complete and at startup.
- **A slow WebSocket client stalled the frame loop** (which also runs autonomy): broadcasts
  wrote synchronously with a 10 s deadline. Now one writer goroutine and a bounded queue per
  client; a client that falls behind is dropped (test: 30 s stall before, 0.6 s after).
- **Obstacles were never drawn into the video when no tag was visible**: the drawer was
  handed raw BGR but decodes an encoded image, and the caller told the cases apart by
  `len(overlay) < w*h*3`. Existing tests asserted only `len(out) == len(in)`, i.e. they
  enshrined the bug. `DrawResults` now draws tags and obstacles onto one RGBA copy and
  returns `(jpeg, drawn)`; the demo path (which passed RGBA as BGR and discarded the result)
  just pushes its frame.

## Tech debt

- Config keys that did nothing were removed (`path_execution.max_speed/turn_speed/
  command_interval_ms`, `tracking.frame_rate/mot20`). `max_speed` also gated whether *any*
  other `path_execution` setting applied, so deleting it silently dropped the tuned values.
- Camera: dead file-capture path, `cap`, `GetFrameAsImage`, `GetRawJPEG`, size/fps getters,
  and the near-duplicate IP constructor removed; the interface is Start/Stop/GetFrame/GetName.
- `config.CameraConfig` is now an alias of `camera.CameraConfig` (was a field-for-field copy).
- WebSocket: client-to-server "command" relay (nothing in the UI consumed it) and the unused
  `bbox`/`track`/`path`/`command` overlay fields removed.
- Data races (documented, known): `cameraName`, `positionEstimator`, obstacles path and
  `isRunning` on `WebServer` are now guarded (mutex / atomic).
- Removed a struct-field round-trip test (`TestByteTrackConfig_Default`); rewrote the drawer
  tests to check pixels.

## Left alone

- `deadcode` still reports five functions, all used by tests (`SetCommandTTL`,
  `HasActiveCommand`, `GetLastCommand`, `GetHomography`, `DebugEnabled`).
- The Vue store still has a `case 'track'` (and matching type) for a message the server no
  longer defines; a test uses it.
- A stalled serial write can still block the E-stop (no write timeout in the serial library).
- The camera capture loop itself does not recover from a `Read` that never returns; the
  watchdog now notices and halts the robot, but restoring video needs a restart.
- Recalibrating while a path is being followed leaves the planner's waypoints in the old
  world frame until the next replan.
- Not verified on hardware: the lost-track stop and the frame de-duplication ran against the
  live camera at 14–15 fps with no errors, but no autonomous drive was run.
