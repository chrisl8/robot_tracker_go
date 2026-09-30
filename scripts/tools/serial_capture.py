#!/usr/bin/env python3
"""Passive serial logger for the firmware spike (no dependencies).

Reads whatever the gamepad Nano sends to the Mac and writes timestamped
hex + ASCII lines. Sends nothing. Type a label and press Enter to insert a
marker (e.g. "robot off"); Ctrl-C to stop.

Opening the port resets the Nano (DTR), so its boot banner is captured too.
The robot-tracker service must be stopped first: it owns the port.

    python3 scripts/tools/serial_capture.py /dev/cu.usbserial-XXXX capture.log
"""
import os, select, sys, termios, time


def open_raw(path, speed=termios.B9600):
    fd = os.open(path, os.O_RDWR | os.O_NOCTTY | os.O_NONBLOCK)
    attrs = termios.tcgetattr(fd)
    attrs[0] = 0  # iflag
    attrs[1] = 0  # oflag
    attrs[2] = termios.CS8 | termios.CREAD | termios.CLOCAL  # cflag
    attrs[3] = 0  # lflag
    attrs[4] = attrs[5] = speed
    attrs[6][termios.VMIN] = 0
    attrs[6][termios.VTIME] = 0
    termios.tcsetattr(fd, termios.TCSANOW, attrs)
    return fd


def printable(b):
    return "".join(chr(x) if 32 <= x < 127 else "." for x in b)


def main():
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    fd = open_raw(sys.argv[1])
    t0 = time.time()
    total = 0
    with open(sys.argv[2], "a", buffering=1) as log:
        def emit(line):
            log.write(line + "\n")
            print(line)
        emit(f"# start {time.strftime('%Y-%m-%d %H:%M:%S')} port={sys.argv[1]} 9600 8N1")
        try:
            watch = [fd, sys.stdin]
            while True:
                r, _, _ = select.select(watch, [], [], 1.0)
                now = time.time() - t0
                if sys.stdin in r:
                    line = sys.stdin.readline()
                    if line == "":  # stdin closed: keep logging, stop watching it
                        watch.remove(sys.stdin)
                    else:
                        emit(f"{now:9.3f} MARK {line.strip()}")
                if fd in r:
                    data = os.read(fd, 4096)
                    total += len(data)
                    emit(f"{now:9.3f} {len(data):4d}B {data.hex(' ')} | {printable(data)}")
        except KeyboardInterrupt:
            emit(f"# end total={total} bytes in {time.time() - t0:.1f}s")


if __name__ == "__main__":
    main()
