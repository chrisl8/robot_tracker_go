# Robot Tracker

An overhead camera system that tracks and autonomously steers robots around a tabletop arena. A ceiling-mounted camera watches the playing field, computer vision identifies robots by their AprilTag markers and detects obstacles with YOLOv8, an A\* path planner charts a course around them, and serial commands drive each robot to its destination.

[Video demo](https://youtu.be/VX7ouvZ6DLI)

Built for [Vorpal the Hexapod](https://log.ekpyroticfrood.net/vorpal-the-hexapod/) but works with any robot that accepts single-character serial commands over an Arduino.

## Features

- **AprilTag identification** — each robot wears a unique tag; the system tracks multiple robots simultaneously
- **YOLOv8 obstacle detection** — real-time detection of objects on the playing field
- **A\* path planning** — global paths with velocity-obstacle local avoidance and multi-robot coordination
- **Vue 3 web UI** — live MJPEG video with canvas overlay showing tracks, paths, and obstacle boundaries; click anywhere to send a robot there
- **Guided web calibration** — print five tags, drop them roughly in the boxes the wizard draws on the video, and it solves the pixel-to-floor mapping (no measuring) and reports its accuracy in centimeters
- **Arduino serial control** — single ASCII commands (F/B/L/R/S) at 9600 baud
- **Demo mode** — run the full UI with simulated robots, no camera or hardware required

## Quick Start

**Prerequisites:** Go 1.24+, Node.js 18+, OpenCV 4.x

```bash
# Install OpenCV (Linux or macOS)
./scripts/install-dependencies.sh

# Build the Vue UI and Go backend
./scripts/build.sh

# Run in demo mode (no camera or Arduino needed)
./scripts/run.sh --demo
```

Open [http://localhost:9086](http://localhost:9086) to see the web UI.

## Setup

### OpenCV

OpenCV 4.x is required (via [GoCV](https://gocv.io/) bindings).

**Linux (Ubuntu/Debian):**
```bash
./scripts/install-dependencies.sh
```

**macOS (Homebrew):**
```bash
brew install opencv
```

The wrapper scripts (`build.sh`, `run.sh`, `test.sh`) automatically set the OpenCV environment variables (`OPENCV_DIR`, `LD_LIBRARY_PATH`, `PKG_CONFIG_PATH`). Always use the wrapper scripts rather than running `go build` directly.

### Camera

A USB webcam is strongly recommended over WiFi/IP cameras. WiFi cameras introduce 1--1.5 second frame delivery gaps due to WiFi jitter, causing steering pauses. USB cameras deliver frames at a consistent ~33 ms interval.

In `config/tracking_config.yaml`:
```yaml
cameras:
  - id: 0
    name: "usb_camera"
    width: 1280
    height: 720
    fps: 30
```

### Arduino / Serial

Connect the Arduino over USB. The tracker sends single ASCII characters at 9600 baud:

| Command  | Char | Description      |
| -------- | ---- | ---------------- |
| FORWARD  | F    | Move forward     |
| BACKWARD | B    | Move backward    |
| LEFT     | L    | Rotate CCW       |
| RIGHT    | R    | Rotate CW        |
| STOP     | S    | Stop immediately |

Format: `{char}\r\n` — for example `F\r\n`.

```bash
# Auto-detect serial port
./scripts/run.sh

# Specify a port
./scripts/run.sh --port /dev/ttyUSB0    # Linux
./scripts/run.sh --port /dev/cu.usbserial-0001  # macOS

# List available ports
./scripts/run.sh --list-ports
```

### Calibration

Calibration teaches the system how camera pixels map to positions on the floor. It uses a
printable target of five tags: one **Center** tag and four **Corner** tags (tag36h11 IDs 100-104,
reserved: robots must not use them).

1. Open the web UI and click the calibration badge at the bottom to start the wizard.
2. **Print** the tags from the link in the wizard (or open `/calibration-tags/print.html`).
   Print at 100% / "Actual size", not "fit to page", on matte paper. Each black square must measure
   **15 cm**; if it doesn't, reprint. That size is the only measurement the calibration relies on.
3. **Place** the tags on the floor where the robots will drive. The wizard clears the view and draws
   five boxes on the live video; drop each tag roughly in its box. No tape measure, no precision: the
   tags can be off-square and rotated any way (only the Center tag's UP arrow should point toward the
   top of the video, since it sets the floor's axes). The software works out where the tags really are
   from their known size.
4. Press Calibrate. The wizard reports the fit error in centimeters, and optionally shows a couple of
   distances between tags you can spot-check with a tape measure. If a tag fits badly (curled, or printed
   at the wrong size) it is named and nothing is saved.

**Pick the tags up afterwards.** They are only needed while calibrating; the result is saved to
`config/calibration_<camera>.yaml` and loaded on every start. Recalibrate only if the camera is moved or
its resolution changes (the calibration records its resolution, and the UI badge says "Recalibrate"
if the camera is now delivering a different one).

### Temporary obstacles

The tracker notices anything that appears in the arena and is not the empty floor, a robot,
or an obstacle you marked static, and can plan around it. It works by comparing each frame with
an adaptive picture of the empty floor (background subtraction): no model, any kind of object,
about 1.5 ms per frame.

- **Detected obstacles** are drawn as translucent shapes over the video, fitted tightly to each
  object's own footprint and rotation rather than an axis-aligned box — an elongated object lying
  at an angle (a screwdriver, a cable) no longer inflates to a large box around it. The planner and
  proximity-stop clearance check use that same tight shape, not a bounding box, so a robot can pass
  closer to a rotated object than a box-only fix would allow. The detector needs a few seconds of
  clear floor at start-up ("Learning background"). Robots are ignored using their tags.
- **Shadow mode first.** By default (`foreground.apply_to_planner: false`) obstacles are shown but the
  planner does not steer around them. Turn on **Steer around them** in the *Temporary obstacles* panel
  (or set it in `config/tracking_config.yaml`) once the detections look right.
- An object that stays put **remains an obstacle until you remove it** (nothing silently absorbs it).
  If something permanent was left in view, click it and choose **Absorb** (treat as floor), or press
  **Reset background** with the floor clear. **Show mask** displays what the detector sees.
- The learned background is **saved to disk** (`config/foreground_bg_<camera>.bin`) and restored on
  the next start, so a restart (during development, a service restart, a host reboot) resumes
  instantly instead of re-learning — and an object already sitting in view isn't briefly absorbed
  into a fresh "empty floor" baseline while it re-warms. A save that no longer matches the camera or
  working resolution is ignored, not misapplied. **Reset background** also discards the saved file,
  not just the live state, so it can't come back on the next restart. Controlled by
  `persist_background`/`persist_interval_sec` in `config/tracking_config.yaml`.
- Slow lighting drift is followed automatically. A sudden change that lights up a large part of the
  picture is treated as a lighting event: the last obstacles are held and the background is relearned.
- While the calibration wizard is open, detection pauses (the tags on the floor would look like
  objects) and the background relearns afterwards.
- Tuning knobs are in the `foreground:` section of `config/tracking_config.yaml` (threshold, minimum
  size, how long an object must persist to count, and so on).

### YOLO Model

A pre-trained YOLOv8n ONNX model is included at `assets/yolov8n.onnx`. No export step is needed.

### macOS: Running Remotely over SSH

macOS blocks camera access for processes started via SSH. Use the LaunchAgent service manager instead:

```bash
./scripts/service.sh install   # one-time setup
./scripts/service.sh start     # start/stop from any terminal, including SSH
./scripts/service.sh stop
./scripts/service.sh status
./scripts/service.sh log       # tail the log file
./scripts/service.sh uninstall
```

After a reboot, someone must log in to the Mac GUI before the LaunchAgent is available. Enable automatic login in **System Settings > Users & Groups** to make this hands-off.

## Hardware

### Cameras

| Option | Price | Resolution | FOV | Notes |
|--------|-------|-----------|-----|-------|
| **Logitech C920s Pro** | ~$50 | 1080p/30 fps | 78° | **Recommended.** Manual focus via UVC, excellent OpenCV compatibility. |
| Logitech Brio 100 | ~$25 | 1080p/30 fps | 58° | Budget pick. Fixed focus works well overhead. Narrower FOV limits arena size. |
| ELP USB (wide-angle) | ~$25--40 | 1080p/30 fps | 100--120° | Good for larger arenas. Avoid >150° — distortion degrades AprilTag detection. |

The AprilTag should be at least ~40 px across in the image for reliable detection. Avoid 4K cameras — the detection pipeline can't use the extra pixels at ~5 fps.

### Compute Platforms

| Platform | AprilTag | YOLOv8 nano | Track + Plan | Effective FPS |
|----------|----------|-------------|-------------|---------------|
| **Mac Mini M4** | ~40--70 ms | ~20--35 ms | ~15 ms | **~8--12 fps** |
| **x86 desktop (4+ cores)** | ~100--150 ms | ~50--70 ms | ~33 ms | **~5 fps** |
| Raspberry Pi 5 | ~150--250 ms | ~100--200 ms | ~50 ms | ~2--3 fps |

**Mac Mini M4** is the fastest option (roughly 2--3x faster than a typical x86 desktop). Install OpenCV via Homebrew. Serial ports are `/dev/cu.usbserial-*` or `/dev/tty.usbmodem-*`.

**Raspberry Pi 5** works but navigation is noticeably more sluggish. If using Pi 5:
- Disable YOLO (`conf_thres: 1.0`) if you don't need dynamic obstacle detection — saves ~100--200 ms per frame
- Increase `quad_decimate` from 2.0 to 3.0 for faster AprilTag detection at slight accuracy cost
- Use the 8 GB RAM model (4 GB is tight with YOLO loaded)

**Budget alternative:** Intel N100-based mini PCs (~$100--150) deliver near-desktop performance in a small form factor and run the standard Linux build without modification.

## Architecture

Data flows through a pipeline:

**Camera → Detection → Tracking → Planning → Controller**

The `RobotSystem` struct in `cmd/main.go` owns and orchestrates all subsystems.

### Project Structure

```
robot_tracker_go/
├── cmd/                        # Application entry point
├── internal/
│   ├── camera/                 # Camera abstraction
│   ├── config/                 # YAML configuration loading
│   ├── controller/             # Arduino serial communication
│   ├── detection/              # AprilTag + YOLOv8 detection
│   ├── planning/               # A* path planning + velocity obstacles
│   ├── position/               # Homography calibration (pixel ↔ world) and calibration target fit
│   ├── tracking/               # ByteTrack multi-object tracker + Kalman filter
│   ├── ui/                     # Gin HTTP server, MJPEG stream, WebSocket, REST API
│   └── utils/                  # Logging utilities
├── ui/                         # Vue 3 + TypeScript frontend (Pinia stores, canvas overlay)
├── Arduino/                    # Arduino gamepad firmware
├── assets/                     # YOLOv8 ONNX model
├── config/                     # Configuration YAML files
└── scripts/                    # Build, run, test, and install wrappers
```

## Configuration

Edit `config/tracking_config.yaml` to configure:

- Robot definitions (tag IDs, physical sizes, speeds)
- Detection parameters (confidence thresholds, decimation)
- Planning settings (step size, safety margins)
- Camera settings (resolution, FPS, source)

## Testing

```bash
# Run all tests (Go + Vue)
./scripts/test.sh --verbose

# Run a specific Go package
go test -v ./internal/planning/

# Run a single Go test
go test -v -run TestFunctionName ./internal/planning/

# Vue tests
cd ui && npm run test:run        # CI mode
cd ui && npm run test:coverage   # with coverage
```

## License

[MIT](LICENSE)
