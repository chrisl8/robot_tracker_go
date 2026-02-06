# Robot Tracker Go - Implementation Plan

## Overview

Go implementation of the multi-robot tracking and control system, migrated from Python for improved performance and type safety.

## Current Phase Status

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1 | ✅ Complete | Foundation (Config, Serial Protocol, Arduino Controller, Command Queue) |
| Phase 2 | ✅ Complete | Core Mathematics (Position types, Homography, Position Estimator) |
| Phase 3 | 🔄 In Progress | Detection Pipeline (AprilTag + YOLO) |
| Phase 4 | ✅ Complete | Tracking (ByteTrack, Kalman Filter, Hungarian Algorithm) |
| Phase 5 | ✅ Complete | Path Planning (A*, Local Planner, Coordinator, Collision Detection) |
| Phase 6 | 🔄 In Progress | Web UI (Gin Web Server, MJPEG Streaming, WebSocket Overlay) |
| Phase 7 | ⏳ Pending | Integration & Testing |

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         This Computer (Host)                            │
│                                                                         │
│  ┌──────────────┐    ┌──────────────┐    ┌────────────────────────┐    │
│  │   Camera     │───>│   Tracker    │───>│     Planner           │    │
│  │ (gocv/ONNX)  │    │   (Go)       │    │     (Go)              │    │
│  └──────────────┘    └──────────────┘    └────────────────────────┘    │
│                                                      │                  │
│  ┌──────────────────────────────────────────────────┼──────────────┐  │
│  │                          UI Layer                 │              │  │
│  │  ┌─────────────┐    ┌────────────────┐    ┌──────▼──────────┐   │  │
│  │  │ MJPEG       │    │  WebSocket     │    │   Gin Web Server │   │  │
│  │  │ Streaming   │    │  Overlay       │    │   (REST API)     │   │  │
│  │  └─────────────┘    └────────────────┘    └──────────────────┘   │  │
│  └────────────────────────────────────────────────────────────────────┘  │
│                                      │                                   │
│                           ┌──────────▼──────────┐                        │
│                           │   Serial/USB        │                        │
│                           │   (Arduino)         │                        │
│                           └─────────────────────┘                        │
└─────────────────────────────────────────────────────────────────────────┘
```

## Dependencies

| Package | Purpose | Status |
|---------|---------|--------|
| `go.bug.st/serial` | Arduino serial communication | ✅ Working |
| `gopkg.in/yaml.v3` | YAML configuration | ✅ Working |
| `github.com/gin-gonic/gin` | Web framework | ✅ Working |
| `github.com/gorilla/websocket` | WebSocket for real-time overlay | ✅ Working |
| `github.com/hybridgroup/mjpeg` | MJPEG encoding | ✅ Working |
| `gocv.io/x/gocv` | OpenCV bindings (camera + image processing) | ⚠️ Build issues |

## Critical Issues

### 1. GoCV Build Failure

**Problem:** `internal/camera` fails to build with undefined constants in gocv package.

**Error:**
```
C:\Users\chris\go\pkg\mod\gocv.io\x\gocv@v0.43.0\core_string.go:5:9: undefined: MatType
C:\Users\chris\go\pkg\mod\gocv.io\x\gocv@v0.43.0\core_string.go:57:9: undefined: CompareType
... (many more errors)
```

**Impact:** Camera capture and image processing cannot be implemented.

**Research Required:** See "Camera/Detection Options" section below.

### 2. Stubbed Detection Pipeline

**Problem:** Both AprilTag and YOLO detectors return empty arrays.

- `internal/detection/apriltag.go:21-29` - `Detect()` returns empty slice
- `internal/detection/yolo.go:50-58` - `Detect()` returns empty slice

**Impact:** No actual robot detection, tracking has no input.

### 3. No Integration in Main Loop

**Problem:** Camera → Detection → Tracking pipeline not connected in `main.go`.

**Current main.go:**
- Creates test pattern generator (demo mode)
- Initializes Arduino controller
- Starts web server
- Does NOT capture frames from camera
- Does NOT run detection pipeline
- Does NOT update tracking

### 4. Missing Tests

Packages without tests:
- `detection` - no test files
- `position` - no test files
- `tracking` - no test files
- `planning` - no test files
- `camera` - no test files (and fails to build)

## Remaining Work by Phase

### Phase 3: Detection Pipeline (In Progress)

| Task | Description | Status |
|------|-------------|--------|
| 3.1 | Fix camera capture implementation | Blocked by GoCV |
| 3.2 | Implement AprilTag detector (use apriltag-go or bindings) | Pending |
| 3.3 | Implement YOLO detector (use ONNX Runtime Go) | Pending |
| 3.4 | Create unified detection pipeline | Done (stubbed) |
| 3.5 | Add tests for detection types and pipeline | Pending |
| 3.6 | Connect camera to detection pipeline | Pending |

### Phase 6: Web UI (In Progress)

| Task | Description | Status |
|------|-------------|--------|
| 6.1 | Gin web server setup | Done |
| 6.2 | MJPEG streaming endpoint | Done |
| 6.3 | WebSocket for real-time overlay | Done |
| 6.4 | Browser frontend (HTML/Canvas) | Pending |
| 6.5 | Keyboard/mouse controls | Pending |
| 6.6 | Robot command API endpoints | Partial |
| 6.7 | Integration with main.go | Partial |
| 6.8 | Tests for UI components | Pending |

### Phase 7: Integration & Testing

| Task | Description | Status |
|------|-------------|--------|
| 7.1 | Connect camera → detection → tracking → planning → control | Pending |
| 7.2 | Add tests for position package | Pending |
| 7.3 | Add tests for tracking package | Pending |
| 7.4 | Add tests for planning package | Pending |
| 7.5 | End-to-end testing with real hardware | Pending |
| 7.6 | Performance profiling and benchmarking | Pending |
| 7.7 | Docker containerization (optional) | Pending |

## Camera/Detection Options Research

### Option 1: Fix GoCV Build

**Approach:** Resolve GoCV compilation issues.

**Steps:**
1. Check GoCV version compatibility with Go 1.23.2
2. Try updating to latest gocv release
3. Consider downgrading Go version if needed
4. Alternative: Use pre-built Docker image with GoCV

**Pros:**
- Mature OpenCV bindings
- Full computer vision capabilities
- Already in use in codebase

**Cons:**
- Build issues on Windows
- Heavy dependency
- May require CGO configuration

**Links:**
- https://github.com/hybridgroup/gocv
- https://gocv.io/

### Option 2: Pure Go Camera with External Detection

**Approach:** Use pure Go for camera capture (via system APIs) + external process for detection.

**Camera Options:**
- `github.com/blackjack/webcam` - Pure Go webcam access (Windows/Linux)
- `github.com/pion/webrtc` - WebRTC for IP camera streams
- Direct OS APIs via cgo (minimal)

**Detection Options:**
- AprilTag: Use `github.com/apriltags/apriltag-go` or C bindings
- YOLO: Use `github.com/ozexpert/onnxruntime-go` for ONNX inference

**Pros:**
- No heavy OpenCV dependency
- Better cross-platform support
- Cleaner separation of concerns

**Cons:**
- Multiple dependencies to manage
- Inter-process communication overhead
- More complex architecture

### Option 3: Go + Python Bridge

**Approach:** Keep Python detection pipeline, bridge via ZeroMQ/HTTP.

**Camera:**
- Python OpenCV for capture and detection
- Go receives pre-processed detections via message queue

**Detection Pipeline:**
- Existing Python code can be used as-is
- AprilTag + YOLO in Python
- Go handles tracking, planning, control

**Pros:**
- Reuse existing proven Python detection
- Leverage best tools for each job
- Easy migration path

**Cons:**
- Additional complexity (IPC)
- Two processes to manage
- Latency overhead

### Option 4: Pure Go with ONNX Runtime

**Approach:** Use ONNX Runtime Go for YOLO, pure Go for camera.

**Camera:**
- `github.com/blackjack/webcam` or similar

**AprilTag:**
- Compile apriltag C library with cgo wrapper
- Or find pure Go implementation

**YOLO:**
- `github.com/ozexpert/onnxruntime-go` for inference

**Pros:**
- Good performance (native code)
- Type safety in Go
- No Python dependency

**Cons:**
- Multiple packages to integrate
- CGO still required for some parts
- Less mature ecosystem

### Recommendation Summary

| Option | Use Case |
|--------|----------|
| Fix GoCV | If we need full OpenCV capabilities and can resolve build issues |
| Go + Python Bridge | If detection accuracy is priority and Python code already works |
| Pure Go (ONNX + webcam) | If we want minimal dependencies and cross-platform support |

**Questions for Discussion:**
1. What detection accuracy do we need? (Python OpenCV vs Go alternatives)
2. Is IP camera (DroidCam) available, or do we need USB camera support?
3. Performance requirements? (latency tolerance)
4. Deployment environment? (Windows development, Linux production?)

## Integration Flow (Target)

```
main() {
    1. Load configuration
    2. Initialize camera
    3. Initialize tracker (ByteTrack)
    4. Initialize planner (load obstacles)
    5. Initialize controller (connect to Arduino)
    6. Initialize web server (MJPEG + WebSocket)
    7. Start main loop (goroutine):
    │   ├── Capture frame
    │   ├── Detect AprilTags
    │   ├── Detect YOLO obstacles
    │   ├── Fuse detections
    │   ├── Update tracks
    │   ├── Estimate positions
    │   ├── Compute velocity commands
    │   ├── Send commands to Arduino
    │   └── Push frame to MJPEG stream
    └── Start web server goroutine
}
```

## Project Structure

```
robot_tracker_go/
├── cmd/main.go                          # Entry point
├── go.mod                              # Go modules
├── internal/
│   ├── config/                         # ✅ Complete
│   │   └── config.go                  # YAML config loading
│   ├── camera/                        # ⏳ Needs camera implementation
│   │   ├── camera.go                  # Camera interface
│   │   ├── gocv_camera.go             # GoCV implementation (blocked)
│   │   ├── video.go                   # Video file source
│   │   ├── ip.go                      # IP camera source
│   │   └── usb.go                     # USB camera source
│   ├── position/                      # ✅ Complete
│   │   ├── types.go                  # Point2D, Pose, Velocity
│   │   ├── homography.go              # Pixel ↔ World transforms
│   │   └── estimator.go               # Position estimation
│   ├── detection/                    # 🔄 In Progress (stubbed)
│   │   ├── types.go                  # Detection types
│   │   ├── apriltag.go               # AprilTag detector (stubbed)
│   │   ├── yolo.go                   # YOLO detector (stubbed)
│   │   └── pipeline.go               # Unified pipeline (stubbed)
│   ├── tracking/                      # ✅ Complete
│   │   ├── types.go                  # Track struct
│   │   ├── kalman.go                 # Kalman filter
│   │   ├── hungarian.go              # Assignment algorithm
│   │   └── bytetrack.go              # ByteTrack implementation
│   ├── planning/                      # ✅ Complete
│   │   ├── astar.go                  # A* pathfinding
│   │   ├── local.go                  # Velocity Obstacle local planner
│   │   ├── coordinator.go           # Multi-robot coordination
│   │   ├── collision.go              # Collision detection
│   │   └── planner.go                # Unified planner interface
│   ├── controller/                   # ✅ Complete
│   │   ├── serial_protocol.go        # Arduino command encoding
│   │   ├── arduino.go                # Serial port management
│   │   ├── command_queue.go          # Continuous command sending
│   │   └── executor.go               # Velocity to command mapping
│   └── ui/                            # 🔄 In Progress
│       ├── webserver.go              # Gin HTTP server
│       ├── mjpeg.go                  # MJPEG streaming
│       ├── websocket.go              # Real-time overlay
│       └── index.go                  # HTML frontend
├── config/
│   ├── tracking_config.yaml           # Default configuration
│   ├── calibration_camera0.yaml      # Camera calibration
│   └── obstacles.yaml                # Static obstacles
├── assets/
│   └── yolov8n.onnx                  # YOLO ONNX model
└── tests/                             # Test files
```

## Serial Protocol

Commands are single ASCII characters:

| Command | Char | Description |
|---------|------|-------------|
| FORWARD | `F` | Move forward |
| BACKWARD | `B` | Move backward |
| LEFT | `L` | Rotate CCW (tank turn) |
| RIGHT | `R` | Rotate CW (tank turn) |
| STOP | `S` | Stop immediately |

Format: `{command}\r\n` (e.g., `F\r\n`)

## Web UI API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Browser UI |
| GET | `/stream` | MJPEG video stream |
| GET | `/ws` | WebSocket for overlay data |
| POST | `/api/command` | Send robot command |
| POST | `/api/destination` | Set navigation target |
| GET | `/api/status` | Get system status |

## Build Commands

```bash
# Install dependencies
go mod tidy

# Build the application
go build -o robot_tracker.exe ./cmd/main.go

# Build with race detector
go build -race -o robot_tracker_race.exe ./cmd/main.go

# Run the application
./robot_tracker.exe

# Run with specific serial port
./robot_tracker.exe --port COM3

# List available serial ports
./robot_tracker.exe --list-ports
```

## Testing Commands

```bash
# Run all tests
go test ./... -v

# Run tests with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Run a single test file
go test -v ./internal/controller/
```

## Performance Targets

| Metric | Python | Go Target | Status |
|--------|--------|-----------|--------|
| Frame latency | ~30ms | <10ms | TBD |
| Tracking FPS | 15-30 | 30-60 | TBD |
| Memory usage | ~500MB | <100MB | TBD |

## Next Steps

1. **Research Camera/Detection Options** - Evaluate GoCV vs alternatives
2. **Fix/Replace Camera Implementation** - Implement chosen approach
3. **Implement Detection** - AprilTag and YOLO detectors
4. **Connect Pipeline** - Camera → Detection → Tracking in main.go
5. **Add Tests** - Coverage for detection, position, tracking, planning
6. **Complete Web UI** - Frontend integration
7. **Integration Testing** - End-to-end with real hardware

## References

- Original Python implementation: `C:\Dev\robot_tracker\`
- GoCV documentation: https://gocv.io/
- ONNX Runtime Go: https://github.com/ozexpert/onnxruntime-go
- AprilTag C library: https://github.com/AprilRobotics/apriltag
