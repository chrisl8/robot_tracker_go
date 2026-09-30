# Arduino firmware

Two sketches make up the radio link between this program and the robot:

    Mac --USB serial 9600--> gamepad (Nano) --Bluetooth HC-05--> robot (Vorpal hexapod)

| Directory | Runs on | Provenance |
|---|---|---|
| `Vorpal-Hexapod-Gamepad/` | Gamepad Arduino Nano | Upstream gamepad sketch, **modified** here (serial command bridge, SD-card code removed). |
| `Vorpal-Hexapod-Robot/` | Hexapod's Arduino | Upstream `#RV3r1c`, **modified**: adds the `H` heartbeat command (see Divergences). |

Upstream: <https://github.com/vorpalrobotics/VorpalHexapod> (vendored at commit `c4103c0`,
2021-02-25). License: CC BY-NC-SA 4.0, attribution to Vorpal Robotics, LLC; keep the license
headers in the sketches.

**Not yet verified:** that the robot's flashed firmware is this exact version. The robot prints
its `Version` string over its USB serial at boot and over Bluetooth after `BlueTooth.begin`;
compare against `#RV3r1c` before assuming these sources match the hardware.

Any change to the robot sketch should be recorded here as a divergence from `c4103c0`.

## Divergences from upstream `c4103c0`

**Robot (`#RV3r1c-Chris10-0.01`)**
- New `H` command (`sendHeartbeat()`, placed after `byte mode`). Reply:
  `V1`, length 4, uptime seconds (2 bytes), servos-detached flag, mode char, checksum.
  It does not read the ultrasonic sensor and does not reset `startedStanding`, so servos
  still power down after 5 s standing while it is polled. The upstream `S` command is unchanged.
- Version string gains a `-Chris10-0.01` suffix (the `#RV3r1c` prefix is kept).
- **Radio baud is 9600, not upstream's 38400** (`BLUETOOTH_BAUD`, used in both `BlueTooth.begin()`
  calls). The gamepad sketch has used 9600 since it was first added to this repo, so the HC-05
  modules here are configured for it. The robot firmware that was originally flashed must have
  had the same change; that source was never saved, and it was overwritten while bringing this
  up. Symptom of a mismatch: the robot prints a flood of `F:n BTER:...` and `BADCHR:` lines and
  its `LOS` counter climbs forever, while the gamepad shows no `#R=` replies.

**Gamepad (`#GV3r1c-Chris10-0.03`)**
- Serial command bridge from the Mac and SD-card code removed (earlier work).
- Once a second one transmit frame carries an extra `H`; the reply is validated and printed to
  the Mac as `#R=<uptime>,<detached>,<mode>` (or `#RBAD:<reason>`). Robot bytes are read even
  while the Mac is driving. The Go controller (`internal/controller/link.go`) turns these lines
  into the robot link state shown in the UI.

**Flash the robot before the gamepad.** An unmodified robot treats `H` as a bad command and
beeps an error every second.

## Building and flashing (arduino-cli)

    arduino-cli compile --fqbn arduino:avr:nano:cpu=atmega328old Arduino/Vorpal-Hexapod-Robot
    arduino-cli upload -p <port> --fqbn arduino:avr:nano:cpu=atmega328old Arduino/Vorpal-Hexapod-Robot

- Both boards here need the **old bootloader** setting (`cpu=atmega328old`); the default fails
  with "not in sync".
- The robot sketch needs `Adafruit PWM Servo Driver Library` **2.0.0** (`arduino-cli lib install
  "Adafruit PWM Servo Driver Library@2.0.0"`). With 3.x the unmodified upstream sketch is already
  too big for the Nano (30794 > 30720 bytes). With 2.0.0: upstream 29748, modified 29930 (97%).
  Very little room is left; check the size after any robot change.
- The robot resets its own Bluetooth after 15 s without a valid packet. A long gamepad reflash
  takes the gamepad offline for that long; after flashing, if the robot stays silent, power-cycle it.
- The Mac's serial port resets the gamepad Nano on open; replies resume within about a second of
  the service connecting (checked live).

## Notes

- **Read a board's flash before overwriting it.** The original robot firmware was lost because it
  was flashed over without a backup. Dump it first, for example
  `avrdude -C <arduino15>/packages/arduino/tools/avrdude/*/etc/avrdude.conf -p m328p -c arduino -P <port> -b 57600 -U flash:r:backup.hex:i`
  (and `eeprom:r:` too), and keep the file.

- The robot has no battery monitor. `A6`/`A7` are grip-arm current sensors and `A3` is unused.
- The robot reboots whenever its serial port is opened (auto-reset works on this wiring), which
  briefly drops the link. Opening it while testing the radio will confuse the result.
- Capture tool: `scripts/tools/serial_capture.py <port> <file>` (stop the service first).
