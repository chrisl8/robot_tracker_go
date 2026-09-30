# Arduino firmware

Two sketches make up the radio link between this program and the robot:

    Mac --USB serial 9600--> gamepad (Nano) --Bluetooth HC-05--> robot (Vorpal hexapod)

| Directory | Runs on | Provenance |
|---|---|---|
| `Vorpal-Hexapod-Gamepad/` | Gamepad Arduino Nano | Upstream gamepad sketch, **modified** here (serial command bridge, SD-card code removed). |
| `Vorpal-Hexapod-Robot/` | Hexapod's Arduino | Upstream sketch, **unmodified** copy of `Version = "#RV3r1c"`. |

Upstream: <https://github.com/vorpalrobotics/VorpalHexapod> (vendored at commit `c4103c0`,
2021-02-25). License: CC BY-NC-SA 4.0, attribution to Vorpal Robotics, LLC; keep the license
headers in the sketches.

**Not yet verified:** that the robot's flashed firmware is this exact version. The robot prints
its `Version` string over its USB serial at boot and over Bluetooth after `BlueTooth.begin`;
compare against `#RV3r1c` before assuming these sources match the hardware.

Any change to the robot sketch should be recorded here as a divergence from `c4103c0`.

## Known issues / must-fix before regular use

- **Sensor poll keeps the robot awake.** The gamepad (`#GV3r1c-Chris10-0.02`) sends an `S`
  command once a second so the Mac can tell whether the robot is alive. The robot's `S` handler
  (`sendSensorData()`) sets `startedStanding = millis()`, which restarts the `BATTERYSAVER`
  (5 s) stand timer, so the servos never auto-detach while the poll runs. That costs battery
  and heat. Before this is used regularly the **robot** firmware must change, either:
  1. stop `sendSensorData()` resetting `startedStanding` (so servos still power down while
     `S` keeps arriving; the reply still proves the robot is alive), or
  2. add a separate heartbeat command/reply that has no side effect on the sleep timer.
  Option 2 is cleaner if we also want to report battery voltage. Either way this is a
  divergence from upstream `c4103c0` and must be recorded here.
