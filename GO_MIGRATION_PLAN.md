# Robot Tracker Go Migration Plan

## Overview

Migration of Python robot tracking system to Go for improved performance and type safety.

## Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                        This Computer                            │
│  ┌──────────────┐    ┌────────────┐    ┌─────────────────┐   │
│  │ Camera       │───>│ Tracker    │───>│ Browser         │   │
│  │ (OpenCV)     │    │ (Go)       │    │ • Video Stream  │   │
│  └──────────────┘    └────────────┘    │ • Overlays       │   │
│                                        │ • Controls        │   │
│                                        └─────────────────┘   │
└────────────────────────────────────────────────────────────────┘
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `go.bug.st/serial` | Arduino serial communication |
| `gopkg.in/yaml.v3` | YAML configuration |
| `github.com/gin-gonic/gin` | Web framework |
| `github.com/gorilla/websocket` | WebSocket for real-time overlay |
| `github.com/gen2brain/mjpeg` | MJPEG encoding |

## Project Structure

```
robot_tracker_go/
├── cmd/main.go                          # Entry point
├── go.mod                              # Go modules
├── internal/
│   ├── config/                          # ✅ Complete
│   │   └── config.go                   # YAML config loading
│   ├── camera/                          # ⏳ Bridge to Python/OpenCV
│   ├── position/                        # ✅ Complete
│   │   ├── types.go                    # Point2D, Pose, Velocity
│   │   └── estimator.go                # Position estimation
│   ├── detection/                      # ⏳ Bridge to Python/YOLO
│   │   ├── types.go                    # Detection types
│   │   └── detector.go                 # Detection interface
│   ├── tracking/                        # ✅ Complete
│   │   ├── types.go                    # Track struct
│   │   ├── kalman.go                   # Kalman filter
│   │   ├── hungarian.go                # Assignment algorithm
│   │   └── bytetrack.go                # ByteTrack implementation
│   ├── planning/                        # ✅ Complete
│   │   ├── astar.go                    # A* pathfinding
│   │   ├── local.go                    # Velocity Obstacle local planner
│   │   ├── coordinator.go              # Multi-robot coordination
│   │   ├── collision.go                # Collision detection
│   │   └── planner.go                  # Unified planner interface
│   ├── controller/                      # ✅ Complete
│   │   ├── serial_protocol.go          # Arduino command encoding
│   │   ├── arduino.go                  # Serial port management
│   │   └── command_queue.go            # Continuous command sending
│   └── ui/                              # 🚧 Phase 6 - Web UI
│       ├── webserver.go                 # Gin HTTP server
│       ├── mjpeg.go                     # MJPEG streaming
│       ├── websocket.go                 # Real-time overlay
│       └── assets/
│           └── index.html               # Browser frontend
└── config/
    └── tracking_config.yaml            # Default configuration
```

## Phased Implementation

### Phase 1: Foundation ✅ Complete
- [x] Config loading (YAML)
- [x] Serial protocol (Command/Mode/Submode encoding)
- [x] Arduino controller with auto-detection
- [x] Command queue with heartbeat
- [x] Tests passing

### Phase 2: Core Mathematics ✅ Complete
- [x] Position types (Point2D, Pose, Velocity)
- [x] Homography matrix (pixel ↔ world transforms)
- [x] Position estimator with calibration loading
- [x] Tests passing

### Phase 3: Detection Pipeline ⏳ In Progress
- [x] Detection types (BoundingBox, AprilTag, YOLODetection)
- [x] AprilTag detector placeholder
- [x] YOLO detector with ONNX config
- [x] Unified detection pipeline with fusion
- [ ] Bridge to Python/OpenCV camera
- [ ] Tests

### Phase 4: Tracking (ByteTrack) ✅ Complete
- [x] Track struct with history and state management
- [x] KalmanFilter for motion prediction
- [x] Hungarian algorithm for optimal assignment
- [x] ByteTrack with high/low confidence detection handling
- [x] Tests passing

### Phase 5: Path Planning ✅ Complete
- [x] A* global pathfinding
- [x] Velocity Obstacle local planner
- [x] Multi-robot coordination
- [x] Collision detection
- [x] Unified planner interface
- [x] Tests passing

### Phase 6: Web UI 🚧 In Progress
- [ ] Gin web server setup
- [ ] MJPEG streaming endpoint
- [ ] WebSocket for real-time overlay
- [ ] Browser frontend (HTML/Canvas)
- [ ] Keyboard/mouse controls
- [ ] Robot command API endpoints
- [ ] Integration with main.go
- [ ] Tests

### Phase 7: Integration
- [ ] End-to-end testing
- [ ] Performance profiling
- [ ] Benchmarking vs Python
- [ ] Docker containerization (optional)

## Serial Protocol

Robot commands are single ASCII characters sent via USB Serial:

| Command  | Char | Description            |
| -------- | ---- | ---------------------- |
| FORWARD  | `F`  | Move forward           |
| BACKWARD | `B`  | Move backward          |
| LEFT     | `L`  | Rotate CCW (tank turn) |
| RIGHT    | `R`  | Rotate CW (tank turn)  |
| WEAPON   | `W`  | Weapon action          |
| STOP     | `S`  | Stop immediately       |

Format: `{command}\r\n` (e.g., `F\r\n`)

## Web UI API Endpoints

### HTTP Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Browser UI |
| GET | `/stream` | MJPEG video stream |
| GET | `/ws` | WebSocket for overlay data |
| POST | `/api/command` | Send robot command |
| POST | `/api/destination` | Set navigation target |
| GET | `/api/status` | Get system status |

### WebSocket Message Types

**Server → Client:**
```json
{"type": "detection", "bbox": [x1, y1, x2, y2], "label": "robot", "confidence": 0.95}
{"type": "track", "id": 1, "bbox": [x1, y1, x2, y2], "history": [[x,y],...]}
{"type": "path", "points": [[x,y],...]}
{"type": "status", "connected": true, "fps": 30}
```

**Client → Server:**
```json
{"type": "command", "action": "forward"}
{"type": "destination", "x": 100, "y": 200}
{"type": "key", "key": "w"}
```

## Build & Run

```bash
# Activate venv (for Python camera if needed)
.\venv\Scripts\Activate.ps1

# Build Go binary
cd C:/Dev/robot_tracker_go
go build -o robot_tracker.exe ./cmd/main.go

# Run
./robot_tracker.exe --camera-id 0

# Access UI
# Open browser to http://localhost:8080
```

## Key Design Decisions

### 1. Web UI vs Desktop (GoCV)
- **Choice:** Web UI via Gin
- **Reason:** Portable, no OpenCV display dependency, works headless
- **Trade-off:** Slight latency (usually imperceptible)

### 2. Camera Source
- **Choice:** Bridge to Python/OpenCV for camera
- **Reason:** AprilTag detection relies on Python libraries
- **Implementation:** ZeroMQ or HTTP between Go and Python

### 3. Detection Pipeline
- **Choice:** ONNX Runtime for YOLO in Go
- **Reason:** No Python dependency for inference
- **Trade-off:** Need to export YOLO to ONNX

## Performance Targets

| Metric | Python | Go Target | Status |
|--------|--------|-----------|--------|
| Frame latency | ~30ms | <10ms | TBD |
| Tracking FPS | 15-30 | 30-60 | TBD |
| Memory usage | ~500MB | <100MB | TBD |

## Notes

- OpenCV only needed for camera capture, not display
- All rendering done in browser via Canvas
- WebSocket provides real-time overlay synchronization
- MJPEG stream for video, JSON for overlay data
