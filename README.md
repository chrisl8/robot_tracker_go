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

### Linux

```bash
# Build with GoCV support
go build -tags=gocv -o robot_tracker ./cmd/main.go

# Build with race detector
go build -race -o robot_tracker ./cmd/main.go
```

### Windows

```powershell
# Set up environment (run as administrator)
.\scripts\setup-gocv.ps1

# Build
go build -tags=gocv -o robot_tracker.exe ./cmd/main.go
```

## Running

### Linux

```bash
# Run demo mode (no camera required)
./robot_tracker --demo

# Run with camera
./robot_tracker

# Run with specific serial port
./robot_tracker --port /dev/ttyUSB0

# List available ports
./robot_tracker --list-ports

# Use custom config
./robot_tracker --config config/custom.yaml
```

### Windows

```powershell
# Run demo mode
.\robot_tracker.exe --demo

# Run with specific port
.\robot_tracker.exe --port COM3

# List available ports
.\robot_tracker.exe --list-ports
```

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
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Run tests without GoCV (demo mode only)
go test ./...
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
- [x] Phase 10: Linux Migration

## License

MIT
