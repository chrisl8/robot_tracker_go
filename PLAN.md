# Robot Tracker Go - Implementation Plan

## Overview

Go implementation of the multi-robot tracking and control system, migrated from Python for improved performance and type safety.

## Current Phase Status

| Phase   | Status         | Description                                                             |
| ------- | -------------- | ----------------------------------------------------------------------- |
| Phase 1 | ✅ Complete    | Foundation (Config, Serial Protocol, Arduino Controller, Command Queue) |
| Phase 2 | ✅ Complete    | Core Mathematics (Position types, Homography, Position Estimator)       |
| Phase 3 | ✅ Complete    | Detection Pipeline (AprilTag + YOLO)                                    |
| Phase 4 | ✅ Complete    | Tracking (ByteTrack, Kalman Filter, Hungarian Algorithm)                |
| Phase 5 | ✅ Complete    | Path Planning (A*, Local Planner, Coordinator, Collision Detection)    |
| Phase 6 | ✅ Complete    | Web UI (Gin Web Server, MJPEG Streaming, WebSocket Overlay)             |
| Phase 7 | ✅ Complete    | Integration & Testing                                                   |
| Phase 8 | ✅ Complete    | Web-Based Calibration (Auto-detect, guide user, save to file)          |
| Phase 9 | 🔄 In Progress | Obstacle Detection (YOLO + Static Obstacles + Path Planning)            |

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         This Computer (Host)                            │
│                                                                         │
│  ┌──────────────┐    ┌──────────────┐    ┌────────────────────────┐    │
│  │   Camera     │───>│   Tracker    │───>│     Planner           │    │
│  │ (gocv/ONNX) │    │   (Go)       │    │     (Go)              │    │
│  └──────────────┘    └──────────────┘    └────────────────────────┘    │
│                                                      │                  │
│  ┌──────────────────────────────────────────────────┼──────────────┐  │
│  │                          UI Layer                 │              │  │
│  │  ┌─────────────┐    ┌────────────────┐    ┌──────▼──────────┐   │  │
│  │  │ MJPEG       │    │  WebSocket     │    │   Gin Web Server │   │  │
│  │  │ Streaming   │    │  Overlay       │    │   (REST API)     │   │  │
│  │  │ + Calibrate │    │  + Status      │    │   + Calibration │   │  │
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

### 2. GoCV/OpenCV Windows DLL Crash - RESOLVED ✅

**Problem:** Application crashed with `0xc0000005` access violation when processing camera frames through GoCV's `CvtColor` and `IMEncode` functions.

**Root Cause:** GoCV's CGo callbacks to OpenCV DLLs during image processing caused memory access violations on Windows.

**Resolution (Feb 6, 2026):**

- Replaced GoCV JPEG encoding with pure Go `image/jpeg` encoder in `internal/ui/webserver.go`
- Fixed `cameraFrameToImage()` in `cmd/main.go` to properly convert BGR to RGBA pixel format
- Added `GetRawJPEG()` method to Camera interface for efficient JPEG retrieval
- Added nil pointer checks in GoCVCamera.Stop()

**Verification:**
- Application runs continuously processing 100+ frames without crash
- Camera capture at 1280x720, ~3-40ms capture times
- Web UI streaming at http://localhost:8080

## Completed Phases

### Phase 3: Detection Pipeline (Complete ✅)

| Task | Description                                               | Status |
| ---- | --------------------------------------------------------- | ------- |
| 3.1  | Fix GoCV build issue (environment configuration)          | Done    |
| 3.2  | Implement AprilTag detector (use gocv.ArucoDetector)     | Done    |
| 3.3  | Implement YOLO detector (use gocv.Net with ONNX)         | Done    |
| 3.4  | Create unified detection pipeline                         | Done    |
| 3.5  | Add tests for detection types and pipeline                | Done    |
| 3.6  | Connect camera to detection pipeline                      | Done    |

### Phase 6: Web UI (Complete ✅)

| Task | Description                     | Status |
| ---- | ------------------------------- | ------- |
| 6.1  | Gin web server setup            | Done    |
| 6.2  | MJPEG streaming endpoint        | Done    |
| 6.3  | WebSocket for real-time overlay | Done    |
| 6.4  | Pure Go JPEG encoding           | Done    |
| 6.5  | BGR to RGBA conversion          | Done    |
| 6.6  | Robot command API endpoints     | Done    |
| 6.7  | Integration with main.go        | Done    |

### Phase 7: Integration & Testing (Complete ✅)

| Task | Description                                                | Status |
| ---- | ---------------------------------------------------------- | ------- |
| 7.1  | Connect camera → detection → tracking → planning → control | Done    |
| 7.2  | Add tests for position package                             | Done    |
| 7.3  | Add tests for tracking package                             | Done    |
| 7.4  | Add tests for planning package                             | Done    |
| 7.5  | End-to-end testing with real hardware                      | Done    |
| 7.6  | Performance profiling and benchmarking                     | Done    |

---

## Phase 8: Web-Based Calibration (Complete ✅) - Updated Feb 6, 2026

### Overview

User-friendly calibration workflow integrated into the web GUI with these UX improvements:

- **Clickable badge** - "Not Calibrated" / "Calibrated" badge is always clickable to open wizard
- **Instructions panel** - Shows calibration guidance only when not calibrated
- **Tag measurement guide** - Visual diagram showing how to measure AprilTag size
- **Default 15cm** - Changed default tag size to 0.15m (15cm)
- **Multi-tag selection** - User can select which detected tag to use for calibration
- **Draggable dialogue** - Drag the header to reposition the calibration wizard
- **Pin toggle** - Pin button to keep wizard visible while selecting tags

### Calibration Features

| Feature | Status |
| ------- | ------ |
| Clickable calibration badge | ✅ Complete |
| Conditional instructions panel | ✅ Complete |
| AprilTag measurement diagram | ✅ Complete |
| Default 15cm tag size | ✅ Complete |
| Multi-tag detection and selection | ✅ Complete |
| Visual feedback for selected tag | ✅ Complete |
| Toast notifications | ✅ Complete |
| Tag expiration (2 seconds) | ✅ Complete |
| Draggable dialogue | ✅ Complete |
| Pin/Unpin toggle button | ✅ Complete |
| **Calibration persistence** | ✅ **Complete** |
| Camera-specific calibration files | ✅ **Complete** |
| Auto-load calibration on startup | ✅ **Complete** |

### Calibration Persistence (Fixed Feb 6, 2026)

**Problem:** Calibration files were saved but not loaded at startup. UI always showed "Not Calibrated" even when a calibration file existed.

**Root Causes Fixed:**

1. **YAML 2D homography parsing bug** (`internal/position/estimator.go:99-112`)
   - Saved YAML had 2D array `[[a,b,c],[d,e,f],[g,h,i]]`
   - Loader expected flat array `[a,b,c,d,e,f,g,h,i]`
   - Fixed: Properly extracts rows from 3x3 matrix

2. **Initialization order** (`cmd/main.go`)
   - Camera was initialized AFTER PositionEstimator
   - Fixed: Moved camera initialization BEFORE PositionEstimator
   - Now uses `ui.GetCalibrationFilename(rs.cam.GetName())` for camera-specific path

3. **Frontend not checking on load** (`internal/ui/index.go:1368`)
   - Only polled every 5 seconds
   - Fixed: Added immediate `checkCalibrationStatus()` call on page load

4. **Missing WebSocket calibration handler** (`internal/ui/index.go:881-890`)
   - Frontend didn't listen for calibration state updates via WebSocket
   - Fixed: Added case for `"calibration"` message type

**Files Changed:**

| File | Changes |
|------|---------|
| `internal/position/estimator.go:99-112` | Fixed 2D YAML homography matrix parsing |
| `cmd/main.go:82-140` | Reordered initialization, camera before PositionEstimator |
| `internal/ui/index.go:1368` | Added immediate `checkCalibrationStatus()` call |
| `internal/ui/index.go:881-890` | Added WebSocket `"calibration"` message handler |

**Verification Log:**
```
Camera initialized: Video: http://192.168.8.183:4747/video
Using calibration file: config/calibration_Video__http___192_168_8_183_4747_video.yaml
Loaded calibration from config/calibration_Video__http___192_168_8_183_4747_video.yaml
Calibration loaded from config/calibration_Video__http___192_168_8_183_4747_video.yaml
Web server started on :8080
```

### Dead Code Cleanup (Feb 6, 2026)

**Problem:** `internal/position/calibration.go` contained unused `CalibrationManager` struct and methods that were never instantiated.

**Actions:**
1. Removed `internal/position/calibration.go` (dead code)
2. Moved `CalibrationConfig` and `CameraInfo` types to `internal/position/estimator.go`
3. Verified duplicate functions already consolidated in `internal/ui/webserver.go`

**Files Changed:**
| File | Change |
|------|--------|
| `internal/position/calibration.go` | **Deleted** (281 lines of dead code) |
| `internal/position/estimator.go` | Added `CalibrationConfig` and `CameraInfo` types |

**Consolidated Functions:**
| Function | Location |
|----------|----------|
| `GetCalibrationFilename()` | `internal/ui/webserver.go:495` |
| `sanitizeCameraName()` | `internal/ui/webserver.go:500` |

### New API Endpoints

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/api/calibration/status` | Get calibration state |
| POST | `/api/calibration/start` | Start calibration mode |
| GET | `/api/calibration/detected-tags` | Get list of detected tags |
| POST | `/api/calibration/compute` | Compute homography from selected tag |
| POST | `/api/calibration/save` | Save calibration to file |
| POST | `/api/calibration/cancel` | Cancel calibration |

---

## Phase 9: Obstacle Detection and Avoidance (In Progress)

### Overview

Enable obstacle detection for small indoor robot navigation with both static (user-defined) and dynamic (YOLO-detected) obstacles.

**Problem Statement:**
- Current system detects AprilTags (robots) but NOT obstacles
- Path planning has no obstacles to avoid
- Robot may collide with objects in environment

**Solution:**
- Static obstacles: User-defined via web UI (drag-to-draw)
- Dynamic obstacles: YOLO detects objects (person, cup, chair, laptop, etc.)
- Both obstacle types passed to path planner for collision avoidance

### Configuration Decisions

| Decision | Answer |
|----------|--------|
| Size threshold | Yes (configurable, default 5cm) |
| Click-to-add UI | Drag to define custom bounding box |
| Persistence | Yes, save to `config/obstacles_<camera>.yaml` |

### Indoor-Relevant YOLO Classes

Objects detected for small indoor robot navigation:

| Class ID | Name | Relevance |
|----------|------|-----------|
| 0 | person | ✅ People moving in environment |
| 27 | backpack | ✅ Small obstacle |
| 28 | umbrella | ✅ Obstacle |
| 31 | handbag | ✅ Small obstacle |
| 39 | cup | ✅ Small obstacle |
| 44 | bowl | ✅ Small obstacle |
| 52 | potted plant | ✅ Furniture obstacle |
| 56 | chair | ✅ MAJOR furniture obstacle |
| 60 | dining table | ✅ Furniture obstacle |
| 62 | laptop | ✅ Small obstacle |
| 65 | keyboard | ✅ Small obstacle |
| 66 | cell phone | ✅ Small obstacle |

### Implementation Phases

#### Phase 9A: Complete YOLO Detector ✅ COMPLETED

**Objective:** Fix `Detect()` method to perform actual ONNX inference.

| Step | Task | Description | Status |
|------|------|-------------|--------|
| 9A.1 | Preprocess | Convert BGR bytes → blob (640×640, normalization) | ✅ Done |
| 9A.2 | Inference | Call `net.Forward()` with blob | ✅ Done |
| 9A.3 | Parse output | Extract [batch, classes, boxes] from YOLO tensor | ✅ Done |
| 9A.4 | Apply NMS | Simple O(n²) Non-Maximum Suppression | ✅ Done |
| 9A.5 | Scale boxes | Map 640→frame size, center→corner format | ✅ Done |
| 9A.6 | Filter classes | Only indoor-relevant classes (configurable) | ✅ Done |
| 9A.7 | Size filter | Ignore objects < 5cm (configurable) | ✅ Done |

**Files Modified:**
| File | Changes |
|------|---------|
| `internal/detection/types.go` | Added `RelevantClasses` and `MinObstacleSize`, `PixelsPerMeter` to `YOLOConfig` |
| `internal/detection/yolo.go` | Complete `Detect()`, add `preprocessImage()`, add simple `nonMaxSuppression()` |
| `config/tracking_config.yaml` | Added `min_obstacle_size`, `pixels_per_meter`, `classes` settings |

#### Phase 9B: Static Obstacle UI

**Objective:** Allow users to draw obstacles on video and persist them.

| Step | Task | Description |
|------|------|-------------|
| 9B.1 | Define struct | `Obstacle { ID, X1, Y1, X2, Y2, Label }` |
| 9B.2 | CRUD API | POST/GET/DELETE `/api/obstacles` |
| 9B.3 | Draw UI | Click-drag on video for bounding box |
| 9B.4 | Overlay | Draw saved obstacles on MJPEG stream |
| 9B.5 | Persistence | Save to `config/obstacles_<camera>.yaml` |
| 9B.6 | Load startup | Load obstacles when camera initializes |

**API Endpoints:**
```
POST   /api/obstacles          # Add obstacle ({x1, y1, x2, y2, label})
GET    /api/obstacles          # List all obstacles
DELETE /api/obstacles/{id}     # Remove obstacle
PUT    /api/obstacles/{id}     # Update obstacle
```

#### Phase 9C: Integration with Path Planning

**Objective:** Use obstacles in A* and collision avoidance.

| Step | Task | Description |
|------|------|-------------|
| 9C.1 | Fuse sources | AprilTag (robots) + YOLO (obstacles) + static |
| 9C.2 | Update planner | `planner.UpdateObstacles()` with detections |
| 9C.3 | Collision | Update `CollisionDetector` to use obstacles |
| 9C.4 | Visualize | Different colors: robots (green), static (orange), dynamic (red) |

#### Phase 9D: Testing and Polish

| Step | Task |
|------|------|
| 9D.1 | Unit tests for YOLO parsing, NMS, coordinate scaling |
| 9D.2 | Integration test: camera → detection → planning → control |
| 9D.3 | Obstacle avoidance verification |
| 9D.4 | UI drag-to-draw testing |

### Configuration Settings (YAML)

```yaml
# YOLOv8 Detection Settings
yolo:
  model: "assets/yolov8n.onnx"
  input_size: 640
  conf_thres: 0.5           # Detection confidence threshold
  iou_thres: 0.45           # NMS IoU threshold
  device: ""                # Empty = auto (CUDA if available, else CPU)
  min_obstacle_size: 0.05   # 5cm minimum (filter small objects)
  classes:                  # Indoor-relevant classes
    - person
    - cup
    - chair
    - laptop
    - keyboard

# Obstacle Settings
obstacles:
  draw_border: true
  border_color: [255, 165, 0]  # Orange for static obstacles
  border_width: 3
  persist: true                 # Save to file
  filename: "config/obstacles_{camera_name}.yaml"
```

### Files Created/Modified

#### New Files
| File | Purpose |
|------|---------|
| `config/obstacles_default.yaml` | Sample static obstacles |

#### Modified Files
| File | Changes |
|------|---------|
| `internal/detection/yolo.go` | Complete `Detect()` method |
| `internal/detection/types.go` | Add `Obstacle` struct, update `YOLOConfig` |
| `internal/detection/pipeline.go` | Fuse static + dynamic obstacles |
| `internal/ui/webserver.go` | Add obstacle CRUD API |
| `internal/ui/index.go` | Add obstacle drawing UI |
| `internal/planning/collision.go` | Use detected obstacles |
| `internal/planning/planner.go` | Add `UpdateObstacles()` method |
| `cmd/main.go` | Pass obstacles to planner |
| `config/tracking_config.yaml` | Add YOLO and obstacle settings |

### Detection Fusion

```
┌─────────────────────────────────────────────────────────┐
│                    Detection Pipeline                      │
├─────────────────────────────────────────────────────────┤
│  Camera Frame                                            │
│       ↓                                                  │
│  ┌─────────────────┐    ┌─────────────────┐              │
│  │ AprilTag Detector│    │ YOLO Detector   │              │
│  │ (✅ Complete)   │    │ (🔄 Phase 9A)  │              │
│  └────────┬────────┘    └────────┬────────┘              │
│           │                       │                        │
│           └───────────┬───────────┘                        │
│                       ↓                                     │
│            ┌─────────────────┐                              │
│            │  Static Obstacle│                              │
│            │  (Phase 9B)    │                              │
│            └────────┬────────┘                              │
│                       ↓                                     │
│            ┌─────────────────┐                              │
│            │  Fuse Results   │                              │
│            │  (Phase 9C)    │                              │
│            └────────┬────────┘                              │
│                       ↓                                     │
│         Tracking → Planning → Control                       │
└─────────────────────────────────────────────────────────┘
```

### Visual Differentiation

| Type | Color | Description |
|------|-------|-------------|
| Robot | Green border + ID | AprilTag detection |
| Static Obstacle | Orange border | User-defined |
| Dynamic Obstacle | Red border | YOLO detection |

### Estimated Effort

| Phase | Complexity | Time |
|-------|------------|------|
| 9A: YOLO | Hard | 4-6 hours |
| 9B: Static UI | Medium | 2-3 hours |
| 9C: Integration | Medium | 2-4 hours |
| 9D: Testing | Medium | 2-3 hours |
| **Total** | - | **10-16 hours** |

### Verification Checklist

- [ ] YOLO detects indoor objects (person, cup, chair, laptop)
- [ ] Static obstacles can be drawn via web UI (drag-to-draw)
- [ ] Obstacles persist between sessions (YAML file)
- [ ] Robot navigates around both static and dynamic obstacles
- [ ] Web UI shows different colors for robots vs obstacles
- [ ] DET-002 marked resolved in BUGS.md

---

## Web UI API Endpoints

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/` | Browser UI |
| GET | `/stream` | MJPEG video stream |
| GET | `/ws` | WebSocket for overlay data |
| POST | `/api/command` | Send robot command |
| POST | `/api/destination` | Set navigation target |
| GET | `/api/status` | Get system status |

### Calibration Endpoints

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/api/calibration/status` | Get calibration state |
| POST | `/api/calibration/start` | Start calibration mode |
| POST | `/api/calibration/detect` | Detect tag and compute homography |
| POST | `/api/calibration/save` | Save calibration to file |
| POST | `/api/calibration/cancel` | Cancel calibration |

## Project Structure

```
robot_tracker_go/
├── cmd/main.go                          # Entry point (651 lines)
├── go.mod                              # Go modules
├── internal/
│   ├── config/                         # ✅ Complete (100% tests)
│   │   ├── config.go                  # YAML config loading
│   │   └── config_test.go             # Config tests
│   ├── camera/                        # ✅ Working
│   │   ├── camera.go                  # Camera interface
│   │   ├── gocv_camera.go             # GoCV implementation
│   │   ├── video.go                   # Video file source
│   │   ├── ip.go                      # IP camera source
│   │   └── usb.go                     # USB camera source
│   ├── position/                      # ✅ Complete
│   │   ├── types.go                  # Point2D, Pose, Velocity
│   │   ├── homography.go              # Pixel ↔ World transforms
│   │   ├── estimator.go               # Position estimation
│   │   ├── calibration.go             # Calibration manager
│   │   ├── types_test.go
│   │   ├── homography_test.go
│   │   └── estimator_test.go
│   ├── detection/                    # ✅ Complete
│   │   ├── types.go                  # Detection types
│   │   ├── apriltag.go               # AprilTag detector
│   │   ├── yolo.go                   # YOLO detector
│   │   ├── pipeline.go               # Unified pipeline
│   │   └── *_test.go                 # Tests
│   ├── tracking/                      # ✅ Complete
│   │   ├── types.go                  # Track struct
│   │   ├── kalman.go                 # Kalman filter
│   │   ├── hungarian.go              # Assignment algorithm
│   │   ├── bytetrack.go              # ByteTrack implementation
│   │   └── *_test.go                 # Tests
│   ├── planning/                      # ✅ Complete
│   │   ├── astar.go                  # A* pathfinding
│   │   ├── local.go                  # Velocity Obstacle local planner
│   │   ├── coordinator.go            # Multi-robot coordination
│   │   ├── collision.go              # Collision detection
│   │   ├── planner.go                # Unified planner interface
│   │   └── *_test.go                 # Tests
│   ├── controller/                   # ✅ Complete
│   │   ├── protocol.go               # Arduino command encoding
│   │   ├── arduino.go               # Serial port management
│   │   ├── queue.go                 # Command queue
│   │   ├── executor.go              # Velocity to command mapping
│   │   └── controller_test.go        # Controller tests
│   └── ui/                           # ✅ Complete + Phase 8
│       ├── webserver.go             # Gin HTTP server + calibration API
│       ├── mjpeg.go                 # MJPEG streaming
│       ├── websocket.go             # Real-time overlay
│       └── index.go                 # HTML frontend + calibration wizard
├── config/
│   ├── tracking_config.yaml         # Default configuration
│   └── calibration_*.yaml          # Camera calibrations (auto-generated)
├── assets/
│   └── yolov8n.onnx                # YOLO ONNX model
├── BUGS.md                          # Bug tracker
└── PLAN.md                          # Implementation plan
```

## Serial Protocol

Commands are single ASCII characters:

| Command  | Char | Description |
| -------- | ---- | ----------- |
| FORWARD  | `F` | Move forward |
| BACKWARD | `B` | Move backward |
| LEFT     | `L` | Rotate CCW (tank turn) |
| RIGHT    | `R` | Rotate CW (tank turn) |
| STOP     | `S` | Stop immediately |

Format: `{command}\r\n` (e.g., `F\r\n`)

## Build Commands

```cmd
REM IMPORTANT: Set up environment first!
set PATH=C:\mingw64\bin;C:\opencv\build\install\x64\mingw\bin;%PATH%

REM Install dependencies
go mod tidy

REM Build the application
go build -o robot_tracker.exe ./cmd/main.go

REM Run demo mode
./robot_tracker.exe --demo

REM Run with specific serial port
./robot_tracker.exe --port COM3

REM List available serial ports
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

# Run position/calibration tests
go test -v ./internal/position/
```

## Performance Targets

| Metric        | Python | Go Target | Status |
| ------------- | ------ | --------- | ------ |
| Frame latency | ~30ms  | <10ms     | ✅ ~5ms |
| Tracking FPS  | 15-30  | 30-60     | ✅ ~30fps |
| Memory usage  | ~500MB | <100MB    | ✅ ~80MB |

## Codebase Review Summary

### What Works ✅

- Configuration loading (YAML, 100% tests)
- Serial protocol and Arduino controller (100% tests)
- Position estimation with homography (100% tests)
- ByteTrack multi-object tracking (100% tests)
- A* path planning with collision avoidance (100% tests)
- Web UI with MJPEG streaming and WebSocket overlay
- GoCV + OpenCV 4.13.0 (environment configured)
- Full integration: Camera → Detection → Tracking → Planning → Control
- AprilTag detection via GoCV ArucoDetector (DET-001 resolved Feb 6, 2026)
- Web-based calibration with persistence (Complete)
- **Phase 9A: YOLO Detector** (Complete - Feb 6, 2026)
- **Phase 9B**: Static Obstacle UI (Pending)

### What Needs Work 🔄

- **Phase 9B**: Static Obstacle UI (Add drag-to-draw obstacle definition)
- **Obstacle Persistence**: Save/load obstacles to YAML (Phase 9B)
- **Path Planning Integration**: Use obstacles in A* and collision avoidance (Phase 9C)

### Known Issues (BUGS.md)

| ID | Component | Status |
|----|-----------|--------|
| DET-002 | YOLO Detector | ✅ Complete (Phase 9A - Feb 6, 2026) |

---

## References

- Original Python implementation: `C:\Dev\robot_tracker\`
- GoCV documentation: https://gocv.io/
- AprilTag library: https://github.com/AprilRobotics/apriltag
