# Plan: a look-ahead ("carrot") path follower

Status: **steps 1-4 done; the carrot is the default.** After two long runs round the fake box on the robot looked good, `carrot` became the default; `path_execution.follower: waypoint` in `config/tracking_config.yaml` restores the old behaviour. The UI draws the carrot as an orange dot.

## Why

Today the robot hops from waypoint to waypoint and counts one as reached within
`waypoint_threshold` (0.10 m). Two visible consequences, both seen on the hardware:

1. **Corner cutting.** At a bend the robot turns as soon as it is within 10 cm of the waypoint,
   so it cuts the corner by about `0.10 m × sin(turn angle)`, always towards the inside. Planned
   bend waypoints sit on the soft obstacle margin (0.27 m centre-to-obstacle), so a 90° bend can
   leave about 2 cm of daylight, and a squeeze (hard limit 0.22 m) would put the robot's centre
   inside its own radius.
2. **Drift on long straight legs.** The simplified path has few waypoints, so a 2 m leg has nothing
   pulling the robot back onto the line. 10 cm of sideways drift was seen on the top run.

Related and already understood: after a proximity replan the escape waypoint is also "reached"
10 cm early, so the robot is still close, and trips the proximity backstop again.

## The idea

Steer at a **carrot**: a point a fixed distance L ahead of the robot's progress along the planned
polyline, instead of at the next waypoint. Waypoints become the shape of a line, not places to
arrive at. The existing `PathExecutor` (turn / burst-wait / nudge / reverse rules) is kept as is:
it only needs a bearing, and anything that produces a bearing can feed it.

Pure pursuit also cuts corners (roughly in proportion to L), so the useful version is **corner-clamped**:

- on straight legs the carrot leads (cross-track correction);
- at a bend the carrot never goes past the bend until the robot is within an arrival radius of it,
  then the robot pivots there (cheap for this robot) and the carrot is released along the next leg;
- the arrival radius at a bend is how much slack the waypoint has above the hard obstacle limit
  (capped at 0.10 m, floored at about 0.04 m), so corners next to obstacles are not cut into them;
- the carrot is also kept within line of sight of the robot at the planner's segment margin.

## Principles (from the owner, and binding)

- **Camera is the odometry.** There is no on-board odometry and no plant model. The follower uses
  only where the robot is and which way it faces, now.
- **The robot is inconsistent and always will be**: battery level changes speed and the ability to
  turn, carpet versus hard floor changes slip, and motion is stick-and-lurch. So every parameter is
  in metres or degrees, never seconds or speed. Nothing is tuned to one robot's speeds.
- L is a multiple of the robot's diameter (about 1), so a different robot needs no retuning.
- The goal-arrival check, the proximity backstop, heading handling (no smoothing) and the deadman
  layers are not touched.

## What was built

- `internal/pursuit`: the carrot tracker. Pure geometry, no robot model. Progress along the path is the
  nearest point on it, searched only a short way back and a good way ahead of where it was (so a path
  that passes near itself cannot snap the robot to the wrong leg). The carrot is `Lookahead` (1 robot
  diameter) ahead of progress, but never past the next *bend* (turn over 15 degrees) until the robot is
  within `ArrivalRadius` (0.2 diameters, 0.06 m for the Vorpal) of it or has gone past it, and is pulled
  back along the path until a straight line to it keeps the planner's clearance (`Planner.CanDriveStraight`).
- `cmd/carrot.go` + a switch in `executeAutonomousControl`: the carrot's bearing goes into the same
  `PathExecutor` as before, so all the turn / burst-wait / nudge / reverse tuning is untouched. The goal
  check, the proximity backstop and the deadmen are untouched too. The follower also tells the planner
  which waypoint is next, so the path drawn in the UI starts at the robot's progress.
- `internal/followsim`: the randomized-plant simulator (see below), with the old follower as `Baseline`.

## Results in the simulator

200 random robots per scene, the same sequence of robots for both followers (`go test
./internal/followsim -run TestReport -v`; `FOLLOWSIM_RUNS=200`):

| scene | follower | reached | collided | closest gap, 5th pct / median | proximity replans per run |
|---|---|---|---|---|---|
| bar detour | waypoint | 91% | 4% | 0.000 / 0.038 m | 11.4 |
| | carrot | 90% | 0% | 0.035 / 0.064 m | 4.3 |
| bar detour, reversed | waypoint | 89% | 4% | 0.000 / 0.036 m | 12.5 |
| | carrot | 90% | 0% | 0.041 / 0.063 m | 4.8 |
| round a block | waypoint | 100% | 2% | 0.020 / 0.057 m | 4.0 |
| | carrot | 100% | 0% | 0.050 / 0.088 m | 1.6 |

Time to goal and jerkiness (command changes per metre) are the same. Cross-track error is better on two of
three scenes. With the camera's position noise doubled (3-6 cm) the carrot still collides in 0-1% of runs
against 11-18% for the waypoint follower.

What the simulator taught us along the way (each is covered by a test):

- Corner cutting is mostly the 0.10 m arrival radius, not noise: with a perfect plant (no noise, no
  slip) the waypoint follower still cuts to about 8 cm.
- The carrot waiting at bends is what does the work: without it the carrot is no better than the
  waypoint follower.
- A robot already inside the planner's hard margin must be allowed to head out. The strict
  "line must keep the margin" check failed from the robot's own position, which collapsed the carrot onto
  the robot and made it dither. `Planner.CanDriveStraight` allows it as long as it gets no closer. The
  strict check is still what the waypoint follower's shortcut uses: loosening it for that follower let it
  skip the waypoint that leads out, and collisions went from 4% to 29%.
- The arrival radius must be above the camera's position noise (a few cm), or a bend never counts as reached.
- An arrival radius that adapts to the room around the bend (the original "step 3") did no better than
  a fixed one, even with doubled noise, so it was dropped.

## Steps

1. **Randomized-plant simulator** (`internal/followsim`, test-only use). Not a model of this robot:
   a model of a bad one. Per run it draws a different plant: wide-range speed and turn rate, a weak
   battery that hurts turning more than walking, stick-slip regimes (stalled, slipping, racing) for
   walking and for turning, a sideways veer, left/right asymmetry, command latency, camera position
   noise, and heading that lags and is noisy and spikes during rotation (the measured noise in
   `executor.go`). The current follower is the baseline. Metrics: success, time, closest approach
   to obstacles, collisions, cross-track error, replans, command switches, leaving the frame. **Done.**
2. **Corner-clamped carrot follower**, behind a config switch, off by default, using the existing
   `PathExecutor` unchanged. **Done** (`path_execution.follower: carrot`).
3. ~~Slack-based arrival radius~~ **Dropped**: no measurable benefit over a fixed radius (see above).
4. **Try it on the robot**, then decide on the default. **Done**: two long runs round the fake box looked good; carrot is the default. Keep watching.
   What to look for in the log (`~/Library/Logs/robot-tracker.log`): fewer `PROXIMITY ... replanning`
   lines, and no stalls where the robot dithers near a bend. If it does dither, the arrival radius
   (`ArrivalRadius` in `pursuit.DefaultParams`) is the first suspect: the real camera's position noise
   may be larger than the simulated 0.5-3 cm.

## Things the simulator cannot tell us

- Whether the assumed ranges (speed, turn rate, noise) cover the real robot's bad days. They are
  guesses, deliberately wide; the real run is the check.
- Tag-loss dropouts are not simulated.
- The baseline and carrot followers in `internal/followsim` are copies of the per-frame logic in
  `cmd/autonomy.go` and `cmd/carrot.go` (package `main`, behind the OpenCV build tag, so not importable).
  If one changes, change the other.
