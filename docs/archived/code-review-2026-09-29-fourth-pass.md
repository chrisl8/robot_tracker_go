# Fourth-Pass Code Review — robot_tracker_go

## Context

A fourth full review, expecting little after the third (`code-review-2026-09-29-third-pass.md`).
It found three real problems, all in the same family as the earlier passes' best finds:
**state that outlives the thing it described** (a path for a goal that was replaced, a
device name for a port that was unplugged, waypoints in a floor frame that was recalibrated
away). Nothing in this pass touches the frame loop, the command queue or the steering logic;
those were re-read in full and no defects were found.

Read this pass: `cmd/` (autonomy, control state, heading, frame loop/processing, watchdog,
callbacks, lifecycle, foreground glue, demo), `internal/controller`, `planning`, `tracking`
(bytetrack, kalman, types), `position` (estimator, homography), `camera` (gocv), `config`,
`ui` (all handlers, ws hub, stream), `detection` (pipeline, apriltag) and the Vue robot
store, control panel and WebSocket composable. Not re-read: the foreground detector's
internals (`foreground*.go`, `temporal.go`), the calibration target fit, and the canvas
rendering code. Each fix has a regression test that was seen to fail with the fix removed.
`go vet` clean; full Go suite with `-race` green. Vue/Playwright were not run (no frontend
change).

## Findings fixed

| Finding | Fix |
|---|---|
| **A goal that could not be planned left the robot following the path to its *previous* goal** (*reproduced*). `SetGoal` only stored a path on success, so goal B (e.g. inside an obstacle's margin, which the UI's pixel-box check does not catch) kept goal A's path. In Auto the robot drove to A, then hovered there forever, because goal-reached is judged against B; the UI showed B as accepted. The same happened when an obstacle change made the existing goal unreachable (`replanAllPathsLocked` kept the pre-change path). | `dropPathLocked`: a failed plan removes the path but keeps the goal, so `GetPathsWithGoals` retries it. Test covers both routes. |
| **An Arduino replugged into a different USB port could never be reconnected.** `Connect()` overwrote the configured `"auto"` with the first detected device (`/dev/cu.usbmodem1101`), so the queue's reconnect loop retried that dead name forever (until restart). | The configured port and the opened device are separate fields; `auto` re-detects on every `Connect`. `listSerialPorts` seam for the test. |
| **Recalibrating with a goal set left the planner in the old floor frame** (previously listed under "left alone"). Waypoints, the goal and the planner's last robot position were in the replaced frame; `RecomputeObstacleWorld` then triggered a replan *from that stale position*. | `OnCalibrationComplete` releases every goal/path, clears the UI destination and stops the robot before anything replans; the operator sets the destination again. |

## Found, not changed (needs a decision)

- **A robot that starts within 8 cm of an obstacle can never leave in Auto.** Autonomy stops
  and `continue`s while `GetClearance < 0.08`, before it steers, so it never moves. Meanwhile
  A\* deliberately plans *out* from inside the margin (`r + 0.12` m, with the start cells
  cleared). It logs a PROXIMITY WARNING every frame (~15 lines/s) while stuck. Operator can
  drive it out in Pilot mode. Options: allow steering while the clearance is not decreasing,
  or lower the stop threshold below the planning margin only for the first seconds of a run.
- A stalled serial write still holds the controller mutex, which `IsConnected()` (called from
  the frame loop once a second) shares (carried over from the third pass).
- `normalizeAngle` / `BearingToCommand` normalise with a `for` loop; an infinite heading would
  spin it forever. Unreachable today (headings come from `Atan2`), only via a hand-edited
  `heading_offset_degrees: .inf`. `config.Load` does no range validation of robot fields
  (diameter, offsets) at all.
- Frontend: a `sendCommand` failure for anything but Stop is silent (the 250 ms refresh
  covers a transient one).
- `sanitizeCameraName` recompiles its regexp on each call (cold path).

## Verified fine (looked for, not found)

Held keys/D-pad on blur and hide (`App.vue` + `ControlPanel.vue`), control state re-sync on
WebSocket reconnect, WebSocket writer/backpressure, MJPEG fan-out, request guard/origin
checks, obstacle copy-on-write, A\* diagonal/corner and grid-bound handling, tracker tag
identity matching and the lost-state reporting, e-stop/halt ordering in the queue, shutdown
ordering.
