# Robot Tracking System (Go)

Go implementation of the multi-robot tracking and control system.

## Prerequisites

- Go 1.21+
- Python 3.8+ (for YOLO model export only)
- Arduino IDE (for firmware)

## Dependencies

Install Go dependencies:

```bash
go mod tidy
```

## YOLO Model Export

Before running the application, export the YOLOv8 model to ONNX format:

```bash
# Run the export script
./scripts/export_model.sh

# Or manually:
python -c "from ultralytics import YOLO; YOLO('yolov8n.pt').export(format='onnx')"
mv yolov8n.onnx assets/
```

## Building

### Environment

You must set up the environment before building so that OpenCV can be found

```powershell
.\scripts\setup-gocv.ps1
```

### Build

```powershell
go build -tags=gocv -o robot_tracker.exe ./cmd/main.go
```

```bash
# Build the application
go build -tags=gocv -o robot_tracker.exe ./cmd/main.go

# Build with race detector
go build -race -o robot_tracker_race.exe ./cmd/main.go
```

## Testing

Start with the Demo

```
.\robot_tracker.exe --demo
```

Open the website and it should show a little demo video.

## Running

```bash
# Run with auto-detected Arduino port
./robot_tracker.exe

# Run with specific port
./robot_tracker.exe --port COM3

# List available ports
./robot_tracker.exe --list-ports

# Use custom config
./robot_tracker.exe --config config/custom.yaml
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
│   ├── controller/             # Arduino communication
│   ├── ui/                     # Display and navigation
│   └── api/                    # REST/WebSocket API
├── assets/
│   └── yolov8n.onnx           # YOLO model
├── config/                     # Configuration files
├── tests/                      # Unit and integration tests
└── scripts/                    # Utility scripts
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
```

## Phase Status

- [x] Phase 1: Foundation (Config, Serial Protocol, Arduino Controller, Command Queue)
- [ ] Phase 2: Camera & Core Math
- [ ] Phase 3: Detection Pipeline
- [ ] Phase 4: Tracking
- [ ] Phase 5: Path Planning
- [ ] Phase 6: UI & Navigation
- [ ] Phase 7: Web API
- [ ] Phase 8: Integration & Testing

## License

MIT
