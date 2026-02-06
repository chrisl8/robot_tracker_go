# Robot Tracker Go - Implementation Plan

## Overview

Go implementation of the multi-robot tracking and control system, migrated from Python for improved performance and type safety.

## Current Phase Status

| Phase   | Status         | Description                                                             |
| ------- | -------------- | ----------------------------------------------------------------------- |
| Phase 1 | ✅ Complete    | Foundation (Config, Serial Protocol, Arduino Controller, Command Queue) |
| Phase 2 | ✅ Complete    | Core Mathematics (Position types, Homography, Position Estimator)       |
| Phase 3 | 🔄 In Progress | Detection Pipeline (AprilTag + YOLO)                                    |
| Phase 4 | ✅ Complete    | Tracking (ByteTrack, Kalman Filter, Hungarian Algorithm)                |
| Phase 5 | ✅ Complete    | Path Planning (A\*, Local Planner, Coordinator, Collision Detection)    |
| Phase 6 | 🔄 In Progress | Web UI (Gin Web Server, MJPEG Streaming, WebSocket Overlay)             |
| Phase 7 | ⏳ Pending     | Integration & Testing                                                   |

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

| Package                        | Purpose                                     | Status     |
| ------------------------------ | ------------------------------------------- | ---------- |
| `go.bug.st/serial`             | Arduino serial communication                | ✅ Working |
| `gopkg.in/yaml.v3`             | YAML configuration                          | ✅ Working |
| `github.com/gin-gonic/gin`     | Web framework                               | ✅ Working |
| `github.com/gorilla/websocket` | WebSocket for real-time overlay             | ✅ Working |
| `github.com/hybridgroup/mjpeg` | MJPEG encoding                              | ✅ Working |
| `gocv.io/x/gocv`               | OpenCV bindings (camera + image processing) | ✅ Working |

## Critical Issues - RESOLVED

### 1. GoCV Build Failure - RESOLVED ✅

**Problem:** GoCV failed to build with undefined constants.

**Root Cause:** Missing environment configuration (GCC and OpenCV DLLs not in PATH), NOT incompatibility.

**Resolution (Feb 6, 2026):**

- Add MinGW GCC to PATH: `C:\mingw64\bin`
- Add OpenCV DLLs to PATH: `C:\opencv\build\install\x64\mingw\bin`
- Build command: `set PATH=C:\mingw64\bin;C:\opencv\build\install\x64\mingw\bin;%PATH% && go build ...`

**Verification:**

```
GoCV version: 0.43.0
OpenCV version: 4.13.0
```

### 2. Stubbed Detection Pipeline - IN PROGRESS 🔄

| Detector | Status  | Next Action                        |
| -------- | ------- | ---------------------------------- |
| AprilTag | Stubbed | Implement using gocv.ArucoDetector |
| YOLO     | Stubbed | Implement using gocv.Net with ONNX |

### 3. No Integration in Main Loop - IN PROGRESS 🔄

Camera → Detection → Tracking pipeline needs to be connected in `cmd/main.go`.

Packages with tests (contrary to previous assessment):

- `config` - ✅ Complete (100% coverage)
- `controller` - ✅ Complete (100% coverage)
- `position` - ✅ Complete (types_test.go, homography_test.go, estimator_test.go)
- `tracking` - ✅ Complete (types_test.go, kalman_test.go, hungarian_test.go, bytetrack_test.go)
- `planning` - ✅ Complete (astar_test.go, collision_test.go)
- `detection` - ⚠️ Partial (types_test.go, pipeline_test.go - no detector tests)
- `camera` - ⚠️ Skipped (camera_test.go, gocv_skip_test.go - GoCV blocked)
- `ui` - ❌ Missing (no tests)

## Remaining Work by Phase

### Phase 3: Detection Pipeline (In Progress)

| Task | Description                                               | Status                  |
| ---- | --------------------------------------------------------- | ----------------------- |
| 3.1  | Fix GoCV build issue (environment configuration)          | **Done** (Feb 6, 2026)  |
| 3.2  | Implement AprilTag detector (use apriltag-go or bindings) | Pending                 |
| 3.3  | Implement YOLO detector (use ONNX Runtime Go)             | Pending                 |
| 3.4  | Create unified detection pipeline                         | Done (types + pipeline) |
| 3.5  | Add tests for detection types and pipeline                | Done                    |
| 3.6  | Connect camera to detection pipeline                      | Pending                 |

### Phase 6: Web UI (In Progress)

| Task | Description                     | Status  |
| ---- | ------------------------------- | ------- |
| 6.1  | Gin web server setup            | Done    |
| 6.2  | MJPEG streaming endpoint        | Done    |
| 6.3  | WebSocket for real-time overlay | Done    |
| 6.4  | Browser frontend (HTML/Canvas)  | Pending |
| 6.5  | Keyboard/mouse controls         | Pending |
| 6.6  | Robot command API endpoints     | Partial |
| 6.7  | Integration with main.go        | Partial |
| 6.8  | Tests for UI components         | Pending |

### Phase 7: Integration & Testing

| Task | Description                                                | Status  |
| ---- | ---------------------------------------------------------- | ------- |
| 7.1  | Connect camera → detection → tracking → planning → control | Pending |
| 7.2  | Add tests for position package                             | Done    |
| 7.3  | Add tests for tracking package                             | Done    |
| 7.4  | Add tests for planning package                             | Done    |
| 7.5  | End-to-end testing with real hardware                      | Pending |
| 7.6  | Performance profiling and benchmarking                     | Pending |
| 7.7  | Docker containerization (optional)                         | Pending |

## Detection Implementation Approach - RESOLVED

GoCV + OpenCV 4.13.0 is now working. The implementation approach is:

**Camera:**

- Use `gocv.io/x/gocv` with OpenCV 4.13.0
- IP camera (DroidCam) configured in `config/tracking_config.yaml`

**AprilTag Detection:**

- Use gocv's built-in `ArucoDetector` with AprilTag dictionaries
- Configure quad_decimate, quad_sigma from config
- Already imported in `internal/detection/apriltag.go`

**YOLO Detection:**

- Use gocv's DNN module for ONNX inference
- Load `assets/yolov8n.onnx`
- Already partially implemented in `internal/detection/yolo.go`

**Links:**

- GoCV documentation: https://gocv.io/
- ONNX Runtime Go: https://github.com/ozexpert/onnxruntime-go (reference)
- AprilTag library: https://github.com/AprilRobotics/apriltag (reference)

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
├── cmd/main.go                          # Entry point (334 lines)
├── go.mod                              # Go modules
├── internal/
│   ├── config/                         # ✅ Complete (100% tests)
│   │   ├── config.go                  # YAML config loading
│   │   └── config_test.go             # Config tests
│   ├── camera/                        # ✅ Working (GoCV + OpenCV 4.13.0)
│   │   ├── camera.go                  # Camera interface
│   │   ├── gocv_camera.go             # GoCV implementation (blocked)
│   │   ├── gocv_skip_test.go          # Skip test for gocv
│   │   ├── video.go                   # Video file source
│   │   ├── ip.go                      # IP camera source
│   │   └── usb.go                     # USB camera source
│   ├── position/                      # ✅ Complete
│   │   ├── types.go                  # Point2D, Pose, Velocity
│   │   ├── homography.go              # Pixel ↔ World transforms
│   │   ├── estimator.go               # Position estimation (280 lines)
│   │   ├── types_test.go
│   │   ├── homography_test.go
│   │   └── estimator_test.go
│   ├── detection/                    # 🔄 In Progress (stubs)
│   │   ├── types.go                  # Detection types
│   │   ├── apriltag.go               # AprilTag detector (stubbed)
│   │   ├── apriltag_stub.go          # Stub implementation
│   │   ├── yolo.go                   # YOLO detector (stubbed)
│   │   ├── yolo_stub.go              # Stub implementation
│   │   ├── pipeline.go               # Unified pipeline
│   │   ├── types_test.go
│   │   └── pipeline_test.go
│   ├── tracking/                      # ✅ Complete
│   │   ├── types.go                  # Track struct
│   │   ├── kalman.go                 # Kalman filter
│   │   ├── hungarian.go              # Assignment algorithm
│   │   ├── bytetrack.go              # ByteTrack implementation (284 lines)
│   │   ├── types_test.go
│   │   ├── kalman_test.go
│   │   ├── hungarian_test.go
│   │   └── bytetrack_test.go
│   ├── planning/                      # ✅ Complete
│   │   ├── astar.go                  # A* pathfinding (215 lines)
│   │   ├── local.go                  # Velocity Obstacle local planner
│   │   ├── coordinator.go           # Multi-robot coordination
│   │   ├── collision.go              # Collision detection
│   │   ├── planner.go                # Unified planner interface
│   │   ├── astar_test.go
│   │   └── collision_test.go
│   ├── controller/                   # ✅ Complete (100% tests)
│   │   ├── protocol.go              # Arduino command encoding (71 lines)
│   │   ├── arduino.go               # Serial port management (148 lines)
│   │   ├── queue.go                 # Command queue
│   │   ├── executor.go              # Velocity to command mapping
│   │   └── controller_test.go        # Controller tests (172 lines)
│   └── ui/                            # 🔄 In Progress
│       ├── webserver.go              # Gin HTTP server (246 lines)
│       ├── mjpeg.go                  # MJPEG streaming
│       ├── websocket.go              # Real-time overlay
│       └── index.go                  # HTML frontend
├── config/
│   ├── tracking_config.yaml           # Default configuration
│   ├── calibration_camera0.yaml      # Camera calibration
│   └── obstacles.yaml                # Static obstacles
├── assets/
│   └── yolov8n.onnx                  # YOLO ONNX model
├── BUGS.md                            # Bug tracker
└── PLAN.md                            # Implementation plan
```

## Serial Protocol

Commands are single ASCII characters:

| Command  | Char | Description            |
| -------- | ---- | ---------------------- |
| FORWARD  | `F`  | Move forward           |
| BACKWARD | `B`  | Move backward          |
| LEFT     | `L`  | Rotate CCW (tank turn) |
| RIGHT    | `R`  | Rotate CW (tank turn)  |
| STOP     | `S`  | Stop immediately       |

Format: `{command}\r\n` (e.g., `F\r\n`)

## Web UI API Endpoints

| Method | Path               | Description                |
| ------ | ------------------ | -------------------------- |
| GET    | `/`                | Browser UI                 |
| GET    | `/stream`          | MJPEG video stream         |
| GET    | `/ws`              | WebSocket for overlay data |
| POST   | `/api/command`     | Send robot command         |
| POST   | `/api/destination` | Set navigation target      |
| GET    | `/api/status`      | Get system status          |

## Build Commands

```bash
# Install dependencies
go mod tidy

# Build the application
go build -tags=gocv -o robot_tracker.exe ./cmd/main.go

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

| Metric        | Python | Go Target | Status |
| ------------- | ------ | --------- | ------ |
| Frame latency | ~30ms  | <10ms     | TBD    |
| Tracking FPS  | 15-30  | 30-60     | TBD    |
| Memory usage  | ~500MB | <100MB    | TBD    |

## Next Steps (Updated Feb 6, 2026)

### Implementation Order (Logical Dependency Chain)

| Step | Task                         | Prerequisite | Status          |
| ---- | ---------------------------- | ------------ | --------------- |
| 1    | Enable Real Camera Capture   | None         | **In Progress** |
| 2    | Connect Detection Pipeline   | Step 1       | Pending         |
| 3    | Implement AprilTag Detection | Step 2       | Pending         |
| 4    | Implement YOLO Detection     | Step 2       | Pending         |
| 5    | Connect Tracking Pipeline    | Steps 3 & 4  | Pending         |
| 6    | Full Integration             | Step 5       | Pending         |

### Step 1: Enable Real Camera Capture 🔄

**Goal:** Replace demo mode with actual camera capture

**Actions:**

- Initialize real camera using `NewGoCVCamera()` from `config/tracking_config.yaml`
- Verify camera frame acquisition works
- Push frames to MJPEG stream

**Deliverable:** Camera → MJPEG stream working

**Code Changes:**

- `cmd/main.go`: Replace `demoPatternGenerator` with `gocvCamera`

### Step 2: Connect Detection Pipeline 🔄

**Goal:** Wire detection to camera frames

**Actions:**

- Pass camera frames to `Detect()` methods
- Verify detection stubs receive frames
- Add debug visualization

**Deliverable:** Detection pipeline receiving real input

### Step 3: Implement AprilTag Detection 🔄

**Goal:** Replace stub with real detection

**Actions:**

- Use gocv's `ArucoDetector` (already imported)
- Configure parameters from config
- Return real `[]AprilTag` detections

**Deliverable:** Real AprilTag detection working

### Step 4: Implement YOLO Detection 🔄

**Goal:** Replace stub with real detection

**Actions:**

- Use gocv's DNN module for ONNX inference
- Load `assets/yolov8n.onnx`
- Configure confidence thresholds
- Return real `[]YOLODetection`

**Deliverable:** Real YOLO obstacle detection working

### Step 5: Connect Tracking Pipeline 🔄

**Goal:** Wire detections to tracking

**Actions:**

- Pass fused detections to ByteTrack
- Verify tracks update correctly
- Add track visualization to MJPEG

**Deliverable:** Camera → Detection → Tracking pipeline

### Step 6: Full Integration 🔄

**Goal:** Complete multi-robot tracking system

**Actions:**

- Tracking → Position estimation
- Position → Path planning
- Planning → Controller commands
- Verify end-to-end functionality

**Deliverable:** Complete multi-robot tracking system

---

## Build Commands (Updated)

```cmd
REM IMPORTANT: Set up environment first!
set PATH=C:\mingw64\bin;C:\opencv\build\install\x64\mingw\bin;%PATH%

REM Install dependencies
go mod tidy

REM Build the application
go build -o robot_tracker.exe ./cmd/main.go

REM Run the application
./robot_tracker.exe --demo

REM Run with specific serial port
./robot_tracker.exe --port COM3
```

**Full documentation:** See `DIAGNOSTIC_RESULTS.md`

## Codebase Review Summary (Feb 6, 2026)

### What Works ✅

- Configuration loading (YAML, 100% tests)
- Serial protocol and Arduino controller (100% tests)
- Position estimation with homography (100% tests)
- ByteTrack multi-object tracking (100% tests)
- A\* path planning with collision avoidance (100% tests)
- Web UI with MJPEG streaming and WebSocket overlay
- GoCV + OpenCV 4.13.0 (environment configured Feb 6, 2026)

### What Needs Work 🔄

- **Camera Capture**: Needs to replace demo mode in main.go
- **AprilTag Detector**: Returns empty array (stubbed - DET-001)
- **YOLO Detector**: Returns empty array (stubbed - DET-002)
- **Main Integration**: Demo mode only, no real camera pipeline
- **UI Tests**: No test coverage

### File Statistics

- Total Go files: 42
- Test files (\*\_test.go): 18
- Test coverage: ~60% (most core packages)
- Configuration YAML: 1
- Documentation: PLAN.md, BUGS.md, AGENTS.md, DIAGNOSTIC_RESULTS.md

## References

- Original Python implementation: `C:\Dev\robot_tracker\`
- GoCV documentation: https://gocv.io/
- ONNX Runtime Go: https://github.com/ozexpert/onnxruntime-go
- AprilTag C library: https://github.com/AprilRobotics/apriltag
