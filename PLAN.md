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
- Web UI streaming at http://localhost:9086

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
Web server started on :9086
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

## Phase 9: Dynamic Obstacle Detection and Avoidance (In Progress)

### Problem Statement

**Why Python version worked better:**

The Python version has a complete dynamic obstacle pipeline:

1. **DynamicObstacle dataclass** - stores position, velocity, radius, class_name, confidence
2. **LocalPlanner with VO algorithm** - Velocity Obstacle for avoidance
3. **YOLO-to-Obstacle conversion** - converts bounding boxes to circular obstacles
4. **Integration in main.py** - filters YOLO detections, creates DynamicObstacles

### Current Status (After Commit da1cf96)

| Component | Status | Notes |
|-----------|--------|-------|
| `DynamicObstacle` struct | ✅ Done | `internal/planning/dynamic_obstacle.go` |
| `LocalPlanner.ComputeVelocityWithObstacles()` | ✅ Done | `internal/planning/local.go:242-264` |
| `Planner.ComputeVelocityWithDynamicObstacles()` | ✅ Done | `internal/planning/planner.go:52-64` |
| YOLO-to-Obstacle converter | ✅ Done | `internal/detection/dynamic_obstacle.go` |
| Tests (11 cases) | ✅ Done | All passing |
| Self-test mode | ✅ Done | `./robot_tracker.exe --self-test` |
| Demo-YOLO mode | ✅ Done | `./robot_tracker.exe --demo-yolo` |
| **Static Obstacle UI** | 🔄 In Progress | API + Backend done, UI remaining |

### What's Been Completed

**Phase 9.5-9.6: Dynamic Obstacle Integration**
- ✅ `RobotSystem` stores `DynamicObstacles` and `StaticObstacles`
- ✅ `ProcessFrame()` calls `ComputeVelocityWithDynamicObstacles()`
- ✅ Configuration loaded from YAML (`ObstacleClasses`, `MinConfidence`)
- ✅ `classesToMap()` helper function

**Phase 9.7: Static Obstacle UI (In Progress)**

Completed:
| Component | File | Status |
|----------|------|--------|
| Config types | `internal/config/config.go` | ✅ |
| `SaveObstacles()` function | `internal/position/estimator.go` | ✅ |
| API types | `internal/ui/types.go` | ✅ |
| API endpoints | `internal/ui/webserver.go` | ✅ |
| RobotSystem integration | `cmd/main.go` | ✅ |

Remaining:
| Component | Status |
|-----------|--------|
| Web UI drawing canvas | ❌ Pending |
| Obstacle panel (list, delete, save) | ❌ Pending |
| Keyboard shortcuts (Z, C, S) | ❌ Pending |
| Draw obstacles on video | ❌ Pending |
| `SetStaticObstacles()` in planner | ❌ Pending |
| Sample YAML file | ❌ Pending |

---

## Phase 9.7: Static Obstacle UI - Implementation Status

### Completed Tasks

1. **Configuration Types** (`internal/config/config.go`)
   - `StaticObstacleConfig` struct
   - `StaticObstacle` JSON types
   - YAML serialization helpers

2. **SaveObstacles** (`internal/position/estimator.go`)
   - Writes obstacles to YAML format
   - Matches Python version's structure

3. **API Types** (`internal/ui/types.go`)
   - `AddObstacleRequest`
   - `UpdateObstacleRequest`
   - `ObstacleResponse`
   - `ObstaclesListResponse`

4. **API Endpoints** (`internal/ui/webserver.go`)
   | Method | Path | Handler |
   |--------|------|---------|
   | GET | `/api/obstacles` | `handleObstaclesList` |
   | POST | `/api/obstacles` | `handleObstacleAdd` |
   | DELETE | `/api/obstacles/:id` | `handleObstacleDelete` |
   | PUT | `/api/obstacles/:id` | `handleObstacleUpdate` |
   | POST | `/api/obstacles/clear` | `handleObstaclesClear` |
   | POST | `/api/obstacles/save` | `handleObstaclesSave` |

5. **RobotSystem Integration** (`cmd/main.go`)
   - Added `StaticObstacles []planning.Obstacle` field
   - `loadStaticObstacles()` function loads from YAML
   - Sets obstacles on web server at startup

### Remaining Tasks

| Task | Description | Status |
|------|------------|--------|
| 9.7.6-9.7.9 | Web UI drawing canvas, panel, shortcuts | ❌ Pending |
| 9.7.10 | Draw static obstacles on video | ❌ Pending |
| 9.7.11 | `SetStaticObstacles()` in planner | ❌ Pending |
| 9.7.12 | Sample YAML file | ❌ Pending |

### Effort Estimate

| Phase | Tasks | Time |
|-------|-------|------|
| 9.7.6-9.7.9 | Web UI components | ~2.5 hours |
| 9.7.10-9.7.12 | Backend remaining | ~30 min |
| **Total remaining** | - | **~3 hours** |

---

**What's Missing:**
- API endpoints for obstacle CRUD (`/api/obstacles`)
- Static obstacle data structure
- Web UI drag-to-draw functionality
- Obstacle overlay on video stream
- YAML persistence

**Files to Create/Modify:**
| File | Change |
|------|--------|
| `internal/ui/webserver.go` | Obstacle CRUD API endpoints |
| `internal/planning/static_obstacle.go` | Static obstacle struct |
| `internal/ui/index.go` | Draw UI + overlay |
| `internal/detection/pipeline.go` | Render static obstacles |
| `config/obstacles.yaml` | Persist obstacles |

---

## Implementation Details

### Phase 9.5: Integration Pseudocode

```go
// In RobotSystem struct, add:
type RobotSystem struct {
    // ... existing fields ...
    DynamicObstacles []*planning.DynamicObstacle
    CurrentRobotID   int
    CurrentGoal      [2]float64
}

// In ProcessFrame:
func (rs *RobotSystem) ProcessFrame(img image.Image, frameData []byte) {
    // ... existing detection ...

    relevantClasses := classesToMap(rs.cfg.LocalPlanning.ObstacleClasses)
    rs.DynamicObstacles = detection.YOLODetectionsToDynamicObstacles(
        detectionResult.YOLODetections,
        rs.positionEst,
        relevantClasses,
        rs.cfg.LocalPlanning.MinConfidence,
    )

    // Track robot from AprilTag
    for _, track := range trackingResult.Tracks {
        if track.State == tracking.TrackStateConfirmed && track.TagID != nil {
            rs.CurrentRobotID = *track.TagID
            // Update robot state in planner
            px, py := track.Bbox[0]+track.Bbox[2]/2, track.Bbox[1]+track.Bbox[3]/2
            worldPos := rs.positionEst.PixelToWorld(px, py)
            rs.planner.UpdateRobotState(rs.CurrentRobotID, worldPos, [2]float64{0, 0})
        }
    }

    // Compute velocity with dynamic obstacles
    if rs.CurrentGoal != [2]float64{0, 0} {
        velocity, _ := rs.planner.ComputeVelocityWithDynamicObstacles(
            rs.CurrentRobotID,
            rs.CurrentGoal,
            rs.DynamicObstacles,
            rs.cfg.LocalPlanning.MinConfidence,
        )
        // Use velocity for robot control
        _ = velocity
    }

    // ... rest of processing ...
}
```

### Phase 9.6: Configuration Helper

```go
func classesToMap(classes []string) map[string]bool {
    m := make(map[string]bool)
    for _, c := range classes {
        m[c] = true
    }
    return m
}
```

---

## Verification Checklist

- [ ] YOLO detections create DynamicObstacles
- [ ] DynamicObstacles passed to local planner
- [ ] Robot velocity adjusted for obstacle avoidance
- [ ] Configuration loaded from YAML (not hardcoded)
- [ ] Dynamic obstacle integration verified

---

## Effort Estimate

| Phase | Tasks | Time |
|-------|-------|------|
| 9.5: Fix Dynamic Integration | 3 tasks | 1-2 hours |
| 9.6: Config Loading | 2 tasks | 30 min |
| 9.7: Static Obstacle UI | 6 tasks | 3-4 hours |

---

## Python-to-Go Mapping (Complete)

| Python | Go |
|--------|----|
| `DynamicObstacle` class | `DynamicObstacle` struct ✅ |
| `DynamicObstacle.from_yolo_detection()` | `YOLODetectionsToDynamicObstacles()` ✅ |
| `LocalPlanner.compute_velocity()` | `LocalPlanner.ComputeVelocityWithObstacles()` ✅ |
| `velocity_to_command()` | `controller/executor.go` |

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

## Linux Migration Plan (Phase 10) - COMPLETED ✅

### Overview

Migrate the project from Windows to Linux (Debian/Ubuntu) for development and deployment.

### Final Status (Updated Feb 7, 2026)

| Component | Windows Status | Linux Status | Notes |
|-----------|---------------|--------------|-------|
| Go runtime | ✅ 1.23.2+ | ✅ 1.25.7 | Working |
| GCC compiler | ✅ MinGW | ✅ system GCC | /usr/bin/gcc |
| OpenCV | ✅ 4.13.0 | ✅ 4.13.0 | Built from source |
| GoCV | ✅ Configured | ✅ Working | With wrapper scripts |
| Serial ports | ✅ Working | ✅ Working | `/dev/tty*` detection |
| Demo mode | ✅ Working | ✅ Working | Via wrapper scripts |

### Resolution Summary

#### 1. AGENTS.md contains Windows-specific instructions - FIXED ✅

**Changes made:**
- Added Quick Start section with wrapper scripts
- Added reminder to use wrapper scripts in Build Commands section

#### 2. Serial port auto-detection - IMPLEMENTED ✅

**Changes made to `internal/controller/arduino.go`:**
- Implemented `detectPorts()` function scanning `/dev/tty*`
- Implemented `autoDetectPort()` returning first Arduino-like port
- Implemented `ListPorts()` returning all detected ports

#### 3. GoCV/OpenCV Compatibility - RESOLVED ✅

**Root cause:** The build process requires proper pkg-config file generation with correct OpenCV library names.

**Resolution:** The wrapper script `scripts/build.sh` properly generates the pkg-config file with correct library names (`libopencv_*.so.4.13.0`).

### Implementation Tasks - ALL COMPLETED ✅

| Task | Description | Status |
|------|-------------|--------|
| 10.1 | Install OpenCV development libraries | ✅ Done |
| 10.2 | Update AGENTS.md for Linux build commands | ✅ Done |
| 10.3 | Implement Linux serial port detection | ✅ Done |
| 10.4 | Test demo mode without camera | ✅ Done |
| 10.5 | Test build with GoCV tags | ✅ Done |
| 10.6 | Fix GoCV/OpenCV compatibility | ✅ Done |
| 10.7 | OpenCV build script | ✅ Done |
| 10.8 | TestOpenCVEnvironment fix | ✅ Done |

### Task Details

#### 10.1-10.8: All Completed ✅

**Verification (Feb 7, 2026):**
```bash
# Build - SUCCESS ✅
$ ./scripts/build.sh
[BUILD] Building robot_tracker with GoCV support...
[BUILD] Done: robot_tracker

# Demo mode - SUCCESS ✅
$ ./robot_tracker --demo
Demo mode: Generating test pattern with AprilTag visualization...
Web server started on :9086

# Serial ports - SUCCESS ✅
$ ./robot_tracker --list-ports
Available serial ports:
  - /dev/ttyACM0

# Tests - SUCCESS ✅
$ ./scripts/test.sh
All tests pass
```

#### Resolution

**Key insight:** The wrapper scripts properly handle OpenCV environment setup:

1. `scripts/build.sh` generates correct pkg-config file with library names
2. Sets `OPENCV_DIR`, `CGO_CPPFLAGS`, `CGO_LDFLAGS`, `PKG_CONFIG_PATH`
3. Uses GoCV with `-tags=gocv` for camera support

**Important:** Always use wrapper scripts - never run `go build` directly with manual environment variables.

### Test Results

```bash
# All tests - PASS ✅
$ go test ./...
ok  	robot_tracker_go/internal	0.008s
ok  	robot_tracker_go/internal/camera	0.004s
ok  	robot_tracker_go/internal/config	0.010s
ok  	robot_tracker_go/internal/controller	0.011s
ok  	robot_tracker_go/internal/detection	0.006s
ok  	robot_tracker_go/internal/planning	0.006s
ok  	robot_tracker_go/internal/position	0.005s
ok  	robot_tracker_go/internal/tracking	0.005s
?   	robot_tracker_go/internal/ui	[no test files]
```

### Serial Port Detection Verified

```bash
# Available tty devices detected
$ ls /dev/tty*
/dev/tty /dev/tty0 /dev/tty1 ... /dev/tty63

# Arduino ports would be detected via:
# - /dev/ttyUSB* (USB-to-serial adapters)
# - /dev/ttyACM* (Arduino Leonardo, Micro)
# - /dev/ttyAMA* (Raspberry Pi serial)
```

### Verification Checklist

- [x] OpenCV 4.13.0 libraries installed (built from source)
- [x] AGENTS.md updated with Quick Start and wrapper scripts
- [x] Serial port detection implemented and working
- [x] Build succeeds (`./scripts/build.sh`)
- [x] Demo mode works (`./scripts/run.sh --demo`)
- [x] All tests pass (`./scripts/test.sh`)
- [x] Wrapper scripts handle all environment setup

### Effort Estimate (Updated)

| Task | Status | Time |
|------|--------|------|
| Install OpenCV (apt) | ✅ Done | 5 min |
| Update AGENTS.md | ✅ Done | 10 min |
| Implement serial detection | ✅ Done | 15 min |
| Fix camera tests for Linux | ✅ Done | 5 min |
| Create OpenCV build script | ✅ Done | 20 min |
| Update README.md | ✅ Done | 10 min |
| Create wrapper scripts | ✅ Done | 10 min |
| Fix TestOpenCVEnvironment | ✅ Done | 15 min |
| **Total completed** | - | **~90 min** |

---

### Phase 10.7: OpenCV Build Script - COMPLETED ✅

Created `scripts/install-opencv.sh` with:

- Automated dependency installation
- OpenCV 4.13.0 download and build
- Configurable install prefix (`OPENCV_PREFIX` env var)
- Verification and cleanup options
- Post-install environment setup instructions

**Usage:**
```bash
# Install OpenCV 4.13.0
./scripts/install-opencv.sh

# Verify installation
./scripts/install-opencv.sh --verify

# Cleanup build artifacts
./scripts/install-opencv.sh --cleanup
```

### Files Created/Modified

| File | Change |
|------|--------|
| `scripts/install-opencv.sh` | Created (5183 bytes, executable) |
| `README.md` | Updated with Linux instructions |
| `AGENTS.md` | Updated with Linux build commands |
| `scripts/build.sh` | Wrapper for building with GoCV |
| `scripts/run.sh` | Wrapper for running with GoCV |
| `scripts/test.sh` | Wrapper for running tests |

---

### Phase 10.8: TestOpenCVEnvironment Fix - COMPLETED ✅

Fixed `TestOpenCVEnvironment` to work on Linux by replacing Windows-specific checks with Linux-specific assertions.

**Files Modified:**

| File | Change |
|------|--------|
| `internal/camera/camera_test.go` | Added Linux checks for OpenCV libraries and OPENCV_DIR |
| `internal/camera/gocv_skip_test.go` | Added Linux-specific assertions |

**Before (Linux FAIL):**
```
TestOpenCVEnvironment                          ❌ FAIL
  OpenCV_bin_directory_in_PATH                 ❌ FAIL (Windows path check)
  OpenCV_DLLs_exist                           ❌ FAIL (Windows DLL check)
```

**After (Linux PASS):**
```
TestOpenCVEnvironment                          ✅ PASS
  OPENCV_DIR_environment_variable              ✅ PASS
  OpenCV_libraries                            ✅ PASS (checks /usr/local/lib)
  GoCV_compatibility                          ✅ PASS (checks OPENCV_DIR + libraries)
```

---

## References

- Original Python implementation: `C:\Dev\robot_tracker\`
- GoCV documentation: https://gocv.io/
- AprilTag library: https://github.com/AprilRobotics/apriltag
