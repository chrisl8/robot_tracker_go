# Fifth-Pass Code Review — robot_tracker_go

## Context

A fifth full review, expecting little after the fourth (`code-review-2026-09-29-fourth-pass.md`).
It found four real problems worth fixing and one worth deciding on, three of them in the
robot's safety paths. The earlier passes' family ("state that outlives the thing it
described") shows up again in two of them; the other two are *ordering*: a command that
lands after the Stop that was meant to end it, and an operator keystroke that is not
what it looks like.

Read this pass, for the first time: the foreground detector (`foreground*.go`,
`temporal.go`, `cmd/foreground_glue.go`), the calibration fit (`calibration_target.go`,
`selfcal.go`, `homography.go`), and the canvas/store/WebSocket side of the Vue app. Re-read:
`cmd/` (autonomy, heading, control state, lifecycle, callbacks, watchdog, frame loop),
`internal/controller`, `planning`, `tracking`, `camera`, `config`, `ui` (all handlers).
No defects in the foreground detector, the calibration fit, or the canvas code.

`go vet`, `staticcheck` and the full Go suite with `-race` were green before and after.
`eslint`, `prettier`, `vue-tsc`, `knip --strict` and all 260 Vitest tests are green.
Playwright was not run and nothing was tried on the robot. Fixes 2–4 each have a regression
test that was seen to fail on the old code (details below); nothing is committed.

## Findings fixed

| Finding | Fix |
|---|---|
| **Modifier keys drove the robot in Pilot mode.** `App.vue`'s key handler ignored Cmd/Ctrl/Alt, so Cmd+W drove forward, Cmd+S backward, Cmd+D right. macOS never delivers the `keyup` of a letter pressed with Cmd held, so the key stayed "held" and `ControlPanel`'s 250 ms refresh kept the command alive indefinitely, defeating the 2 s deadman until the window lost focus. (The macOS `keyup` behaviour is documented browser behaviour, *not reproduced here*; the wrong-direction driving follows directly from the code.) | Handlers moved to `utils/driveKeys.ts`: a keydown with a modifier is not a drive key and releases whatever was held; keyup always releases. Test seen failing before the guard was added (4 of 7). |
| **A vanished path left the robot running for ~0.4 s.** When a robot's path disappeared (an obstacle blocked the goal, a proximity replan failed), autonomy called `ClearActiveCommand()`, which only stops the *heartbeat* re-sending the command: the robot keeps its last command until the next heartbeat writes Stop. Measured 377 ms, against 6 ms for an explicit Stop. | `CommandQueue.StopIfActive()`: Stop now if a movement command is held, no-op when idle (so calling it every frame does not spam the serial line, and a halted queue stays halted). Autonomy uses it. Queue-level tests; the first was seen to fail with a no-op stub. The call in `cmd/autonomy.go` itself is not observable from the `cmd` rig (its controller is unconnected), so it is covered only by the queue tests. |
| **Switching to Hold/Pilot could be followed by a drive command** (*reproduced*, 3 of 3 runs). `executeAutonomousControl` checked the mode once at the top; a frame already past that check enqueued its command after `SetControlMode`'s Stop, and the deadman then let it run up to 2 s. E-stop was immune (the queue's `halted` flag). | The frame holds the control-state read lock for its whole run, so a mode switch waits for it. `EmergencyStop` now halts the queue *before* taking that lock so it never waits on a frame. Stress test (400 Auto→Hold switches against a running frame loop) fails on the old code, passes under `-race`; a second test seen failing on the old e-stop ordering. |
| **The UI kept drawing a driving robot after it stopped.** `robotCommands` (the per-robot motion state behind the thrust animation) was written only by autonomy and never cleared, so after Hold, an e-stop or a cleared goal the robot kept showing "forward". | Rebuilt every autonomy frame; cleared whenever autonomy is not driving. `BroadcastTracks` now runs after autonomy in both frame paths, so it sends this frame's state, not last frame's. Test covers all three routes and fails on the old code. |

## Found, not changed

- **DNS rebinding passes the origin check** (*reproduced*). `originAllowed` accepts an Origin equal to the Host, so a page on `evil.example` rebinding to the robot's LAN IP sends both as `evil.example:9086` and is allowed; the server listens on `:9086` with no auth. A Host allowlist (LAN IP, `.local`, tailnet name) is the fix; it needs a decision on which names to allow.
- Planning has no arena bounds: the grid is 100 × 100 m, so a detour can route outside the camera's view, and a 6 m wall already exceeds the 10 000-iteration A\* cap. A goal that cannot be planned is retried every frame by `AddRobot` (5.4 ms each in a scratch run with an enclosed goal).
- Startup fails open: any config error makes the service run in demo mode.
- An unknown `april_tags.family` silently becomes `tag36h11`.
- `modelPersister.clear()` does not wait for an in-flight background save, so a save started just before a reset could rewrite the pre-reset background (tiny window).
- Carried over: a stalled serial write still holds the controller mutex; `normalizeAngle` loops on a non-finite heading; `config.Load` does no range validation.

## Tech debt noticed

- `executeAutonomousControl` has cyclomatic complexity 44 (`matchTracks` and `AStar.Plan` 34, `foregroundModel.step` 33).
- Dead code: `CalibrationData`, `PositionResult`, `RobotPosition`, `Track.ToDict`, `DetectionTypeFused`, seven `AprilTagConfig` fields nothing sets or reads (`NThreads`, `DecodingSharpening`, `MinTagSize`, `MaxHammingDistance`, `RefineEdges`, `QuadSigma`, `TagSize`), the intrinsics parsing in `LoadCalibration` (write-only), `Homography.PixelsPerMeter` (stored, never read), `obstaclesSubsystem.StaticObstacles`.
- Obstacles are written through typed structs (`ui/obstacles.go`) but read back with a hand-rolled `map[string]interface{}` loader (`cmd/web_callbacks.go`); two comments refer to a `PositionEstimator.LoadObstacles` that no longer exists.
- `FitTarget` reads `perTag[worst]` after sorting `perTag`; it works only because the centre tag has the lowest ID.
- Possible perf, not measured: `minAreaRectForLabel` makes a cgo `GetIntAt` call per pixel; the camera copies every captured frame (`ToBytes`) at capture rate while processing is capped lower.

## Verified fine (looked for, not found)

Foreground background model (warm-up, gain, shadow gate, guard/rewarm, absorb), temporal
filter association and quantisation, persistence file handling, calibration LM fit and
homography normalisation, tracker association and lost-state reporting, Kalman update,
camera reopen/stop paths, WebSocket writer/backpressure, request guard body/type limits,
config defaults, A\* grid/diagonal handling, queue heartbeat/deadman/reconnect ordering.
