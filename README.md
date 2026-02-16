# Robot Tracking System (Go)

Go implementation of the multi-robot tracking and control system.

## Prerequisites

- Go 1.23.2+
- CMake 3.16+ (for building OpenCV from source)
- Git

## Dependencies

Install Go dependencies:

```bash
go mod tidy
```

Install OpenCV (required for camera support):

### Linux (Ubuntu/Debian)

**Option 1: Use provided install script (recommended)**

```bash
./scripts/install-opencv.sh

# Add to ~/.bashrc for future sessions:
echo 'export OPENCV_DIR="/usr/local"' >> ~/.bashrc
echo 'export LD_LIBRARY_PATH="/usr/local/lib:$LD_LIBRARY_PATH"' >> ~/.bashrc
source ~/.bashrc
```

**Option 2: Manual installation**

See [scripts/install-opencv.sh](scripts/install-opencv.sh) for details.

### Windows

See [AGENTS.md](AGENTS.md) for Windows-specific setup instructions.

## YOLO Model Export

Before running the application with obstacle detection, export the YOLOv8 model to ONNX format:

```bash
# Run the export script
./scripts/export_model.sh

# Or manually:
python -c "from ultralytics import YOLO; YOLO('yolov8n.pt').export(format='onnx')"
mv yolov8n.onnx assets/
```

## Building

### Linux (Recommended: use wrapper scripts)

```bash
# Build with GoCV support (automatically sets environment)
./scripts/build.sh

# Build with race detector
export OPENCV_DIR="/usr/local"
export LD_LIBRARY_PATH="/usr/local/lib:$LD_LIBRARY_PATH"
go build -race -o robot_tracker ./cmd/main.go
```

**Note:** The wrapper scripts (`build.sh`, `run.sh`, `test.sh`) automatically set up the OpenCV environment (`OPENCV_DIR`, `LD_LIBRARY_PATH`, `PKG_CONFIG_PATH`). No need to modify `.bashrc`.

### Windows

```powershell
# Set up environment (run as administrator)
.\scripts\setup-gocv.ps1

# Build
go build -tags=gocv -o robot_tracker.exe ./cmd/main.go
```

## Running

### Linux (Recommended: use wrapper scripts)

```bash
# Run demo mode (no camera required)
./scripts/run.sh --demo

# Run with camera
./scripts/run.sh

# List available ports
./scripts/run.sh --list-ports

# Use custom config
./scripts/run.sh --config config/custom.yaml
```

**Note:** Wrapper scripts automatically set `OPENCV_DIR` and `LD_LIBRARY_PATH`.

### Windows

```powershell
# Run demo mode
.\robot_tracker.exe --demo

# Run with specific port
.\robot_tracker.exe --port COM3

# List available ports
.\robot_tracker.exe --list-ports
```

### macOS: Running Remotely over SSH

macOS blocks camera access for processes started via SSH (the SSH daemon lacks camera permission). To start/stop the tracker remotely, use the LaunchAgent service manager:

```bash
# One-time setup (generates a LaunchAgent plist and loads it)
./scripts/service.sh install

# Start/stop from any terminal — including SSH sessions
./scripts/service.sh start
./scripts/service.sh stop
./scripts/service.sh restart
./scripts/service.sh status
./scripts/service.sh log        # tail -f the log file
./scripts/service.sh uninstall  # remove the LaunchAgent
```

The LaunchAgent runs in your GUI login session, so it inherits camera permissions. Logs go to `~/Library/Logs/robot-tracker.log`.

**After a reboot**, someone must log in to the Mac (GUI) before the LaunchAgent is available. To make this fully hands-off, enable automatic login in **System Settings > Users & Groups > Automatic Login**.

## Project Structure

```
robot_tracker_go/
├── cmd/main.go                 # Application entry point
├── internal/
│   ├── config/                 # Configuration loading
│   ├── camera/                 # Camera abstraction
│   ├── detection/              # AprilTag + YOLO detection
│   ├── tracking/               # Multi-object tracking
│   ├── position/               # Position estimation
│   ├── planning/               # Path planning
│   ├── controller/              # Arduino communication
│   └── ui/                     # Display and navigation
├── assets/
│   └── yolov8n.onnx           # YOLO model
├── config/                     # Configuration files
├── scripts/
│   ├── install-opencv.sh       # OpenCV installation script (Linux)
│   ├── setup-gocv.ps1          # OpenCV setup script (Windows)
│   └── export_model.sh         # YOLO model export script
└── tests/                      # Unit and integration tests
```

## Configuration

Edit `config/tracking_config.yaml` to configure:

- Robot definitions (tag IDs, sizes, speeds)
- Detection parameters (confidence thresholds)
- Planning settings (step size, safety margins)
- Camera settings

## Hardware Recommendations

### Camera

A USB webcam is strongly recommended over WiFi/IP cameras. WiFi cameras (DroidCam, etc.) introduce 1-1.5 second frame delivery gaps due to WiFi jitter, causing the robot to lose steering corrections and pause repeatedly. USB cameras deliver frames at a consistent ~33ms interval.

| Option | Price | Resolution | FOV | Notes |
|--------|-------|-----------|-----|-------|
| **Logitech C920s Pro** | ~$50 | 1080p/30fps | 78° | **Recommended.** Manual focus via UVC, excellent OpenCV compatibility. |
| Logitech Brio 100 | ~$25 | 1080p/30fps | 58° | Budget option. Fixed focus works well for overhead mounting. Narrower FOV limits arena size. |
| ELP USB (wide-angle) | ~$25-40 | 1080p/30fps | 100-120° | Good for larger arenas. Avoid ultra-wide (>150°) — excessive distortion degrades AprilTag detection. |

**Config change for USB cameras:** In `config/tracking_config.yaml`, replace the IP camera block with:
```yaml
cameras:
  - id: 0
    name: "usb_camera"
    width: 1280
    height: 720
    fps: 30
```

**Avoid:** 4K cameras (the detection pipeline can't use the extra pixels at ~5fps), ultra-wide fisheye lenses (>150° FOV). The AprilTag should be at least ~40 pixels across in the image for reliable detection.

### Compute Platform

| Platform | AprilTag | YOLOv8 nano | Track + Plan | Effective FPS |
|----------|----------|-------------|-------------|---------------|
| **Mac Mini M4** | ~40-70ms | ~20-35ms | ~15ms | **~8-12fps** |
| **x86 desktop (4+ cores)** | ~100-150ms | ~50-70ms | ~33ms | **~5fps** |
| Raspberry Pi 5 | ~150-250ms | ~100-200ms | ~50ms | ~2-3fps |

**Developed and tested on** x86_64 desktop/laptop with 4+ CPU cores (Linux). This is the current primary platform.

**Mac Mini M4 (Apple Silicon)** is the fastest option — roughly 2-3x faster than a typical x86 desktop, giving ~8-12fps steering updates for noticeably tighter navigation. Requires macOS-specific setup:
- Install OpenCV via Homebrew (`brew install opencv`) instead of the Linux install script
- Build scripts (`build.sh`, `run.sh`, `test.sh`) are Linux-specific and need adaptation for macOS paths
- Serial port uses `/dev/cu.usbserial-*` or `/dev/tty.usbmodem-*` instead of `/dev/ttyUSB0`
- USB cameras work via AVFoundation backend in OpenCV — no driver issues expected

**Raspberry Pi 5** works but navigation is noticeably more sluggish at ~2-3fps. Mitigations if using Pi 5:
- Disable YOLO (set `conf_thres: 1.0`) if dynamic obstacle detection isn't needed — recovers ~100-200ms per frame
- Increase `quad_decimate` from 2.0 to 3.0 for faster AprilTag detection at slight accuracy cost
- Use the 8GB RAM model (4GB is tight with YOLO loaded)
- Building OpenCV/GoCV on ARM64 requires compiling from source (~1-2 hours)

**Portable alternative:** Intel N100-based mini PCs (~$100-150) deliver near-desktop performance in a small form factor and run the standard x86 Linux build without modification.

## Serial Protocol

Commands are single ASCII characters:

| Command  | Char | Description      |
| -------- | ---- | ---------------- |
| FORWARD  | F    | Move forward     |
| BACKWARD | B    | Move backward    |
| LEFT     | L    | Rotate CCW       |
| RIGHT    | R    | Rotate CW        |
| STOP     | S    | Stop immediately |

Format: `{command}\r\n` (e.g., `F\r\n`)

## Testing

```bash
# Run all tests (automatically sets environment)
./scripts/test.sh

# Run all tests manually
export OPENCV_DIR="/usr/local"
export LD_LIBRARY_PATH="/usr/local/lib:$LD_LIBRARY_PATH"
go test ./... -v

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Phase Status

- [x] Phase 1: Foundation (Config, Serial Protocol, Arduino Controller, Command Queue)
- [x] Phase 2: Core Mathematics (Position types, Homography, Position Estimator)
- [x] Phase 3: Detection Pipeline (AprilTag + YOLO)
- [x] Phase 4: Tracking (ByteTrack, Kalman Filter, Hungarian Algorithm)
- [x] Phase 5: Path Planning (A*, Local Planner, Coordinator, Collision Detection)
- [x] Phase 6: Web UI (Gin Web Server, MJPEG Streaming, WebSocket Overlay)
- [x] Phase 7: Integration & Testing
- [x] Phase 8: Web-Based Calibration
- [x] Phase 9: Obstacle Detection (YOLO + Static Obstacles + Path Planning)
- [x] Phase 10: Linux Migration (Updated Feb 7, 2026)

## License

MIT
