# Archived Bug Tracker - Robot Tracker Go

> This file contains resolved bugs that have been fixed and are kept for historical reference and institutional memory. See [BUGS.md](BUGS.md) for open issues.

---

## CRITICAL - Resolved

### GOCV-001: GoCV Build Failure - RESOLVED (Environment Issue, Not Compatibility)

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | Critical |
| **Impact** | Camera capture and image processing cannot be implemented |
| **Component** | `internal/camera` |

**Symptom:**
```
C:\Users\chris\go\pkg\mod\gocv.io\x\gocv@v0.43.0\core_string.go:5:9: undefined: MatType
C:\Users\chris\go\pkg\mod\gocv.io\x\gocv@v0.43.0\core_string.go:57:9: undefined: CompareType
... (many more errors)
```

**Root Cause (UPDATED):**
**GoCV v0.43.0 IS COMPATIBLE with OpenCV 4.13.0.** The build failures were caused by **missing environment configuration**, NOT incompatibility.

**Verified Results (Feb 6, 2026):**
```
GoCV version: 0.43.0
OpenCV version: 4.13.0
```

**Actual Issues:**
1. **Missing GCC Compiler**: MinGW GCC was installed but not in PATH
   - Location: `C:\mingw64\bin\gcc.exe`
   - Solution: Add to PATH before building

2. **Missing OpenCV DLLs at Runtime**: OpenCV libraries not accessible
   - Location: `C:\opencv\build\install\x64\mingw\bin\*.dll`
   - Solution: Add to PATH before running

**Environment:**
- Go: 1.25.7
- GoCV: v0.43.0
- OpenCV: 4.13.0
- OS: Windows 11

**Resolution:**
1. Add proper PATH configuration for GCC and OpenCV DLLs
2. Build command with correct environment:
   ```cmd
   set PATH=C:\mingw64\bin;C:\opencv\build\install\x64\mingw\bin;%PATH%
   go build -tags=gocv -o robot_tracker.exe ./cmd/main.go
   ```

**Related Files:**
- `internal/camera/camera.go`
- `internal/camera/gocv_camera.go`
- `scripts/install-opencv-for-gocv.ps1`
- `diagnostics/DIAGNOSTIC_RESULTS.md`

---

### DET-001: AprilTag Detector - RESOLVED (Feb 6, 2026)

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | Critical |
| **Impact** | Robot detection via AprilTags works correctly |
| **Component** | `internal/detection/apriltag.go` |

**Resolution:**
The AprilTag detector was implemented on Feb 6, 2026. The implementation:

1. Uses `gocv.ArucoDetector` with `gocv.GetPredefinedDictionary(gocv.ArucoDictAprilTag_36h11)`
2. Supports multiple tag families: tag16h5, tag25h9, tag36h10, tag36h11
3. Configurable quad decimate for performance
4. Returns proper `[]AprilTag` with ID, corners, center, and size

**File Structure:**
| File | Build Tag | Purpose |
|------|-----------|---------|
| `apriltag.go` | `//go:build gocv` | Full implementation using GoCV |
| `apriltag_stub.go` | `//go:build !gocv` | Stub for non-GoCV builds |

**Code Location:**
```go
// internal/detection/apriltag.go:23-59
func NewAprilTagDetector(config AprilTagConfig) (*AprilTagDetector, error) {
    dictionary := gocv.GetPredefinedDictionary(gocv.ArucoDictAprilTag_36h11)
    params := gocv.NewArucoDetectorParameters()
    detector := gocv.NewArucoDetectorWithParams(dictionary, params)
    ...
}
```

**Test Verification:**
- Tests at `internal/detection/types_test.go:287-352` verify detector creation and detection
- Integration tests at `internal/integration_test.go:15-64` verify full pipeline

**Known Behavior:**
- Empty/invalid images return empty slice (expected)
- Must build with `-tags=gocv` for full implementation

---

### DET-002: YOLO Detector - RESOLVED (Feb 9, 2026)

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | Critical (was) |
| **Impact** | None - Fully implemented |
| **Component** | `internal/detection/yolo.go` |

**Resolution (Feb 9, 2026):**

YOLO detector is fully implemented and working. The bug entry was stale documentation.

**Implementation Details:**
- **Model**: `assets/yolov8n.onnx` (12.8 MB)
- **Backend**: Uses GoCV's ONNX support (`gocv.ReadNetFromONNX`)
- **Input Size**: 640x640 (configurable)
- **Confidence Threshold**: 0.5 (configurable)
- **IOU Threshold**: 0.45 (configurable)
- **Relevant Classes**: person, backpack, umbrella, handbag, cup, bowl, potted plant, chair, dining table, laptop, keyboard, cell phone

**Code Location:**
```go
// internal/detection/yolo.go:24-84
func NewYOLODetector(config *YOLOConfig) (*YOLODetector, error) {
    // Loads ONNX model, sets up preprocessing, returns configured detector
    net := gocv.ReadNetFromONNX(config.ModelPath)
    // ...
}

// internal/detection/yolo.go:91-154
func (d *YOLODetector) Detect(imageBytes []byte, width, height int) []YOLODetection {
    // Full implementation: preprocess, inference, NMS, filtering
    // Returns actual YOLODetection results
}
```

**Files:**
| File | Purpose |
|------|---------|
| `internal/detection/yolo.go` | Full implementation (361 lines) |
| `internal/detection/yolo_stub.go` | Stub only for `!gocv` builds (CI/headless) |
| `assets/yolov8n.onnx` | YOLOv8n model file |
| `config/tracking_config.yaml` | YOLO configuration |

**Verification:**
- Model file exists: ✅
- Implementation complete: ✅
- Tests passing: ✅
- Demo mode works: ✅

---

### PIPELINE-001: Pipeline Integration - RESOLVED (Feb 9, 2026)

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | High (was) |
| **Impact** | None - Pipeline fully connected |
| **Component** | `cmd/main.go` |

**Resolution (Feb 9, 2026):**

This entry was stale documentation. The pipeline is fully connected and operational.

**Implementation (`cmd/main.go:435-516`):**

```go
func (rs *RobotSystem) ProcessFrame(img image.Image, frameData []byte) {
    // 1. Detection Pipeline
    detectionResult := rs.detectionPipe.Detect(frameData, width, height, timestamp, rs.frameNum)

    // 2. Dynamic Obstacles from YOLO
    rs.DynamicObstacles = detection.YOLODetectionsToDynamicObstacles(
        detectionResult.YOLODetections, rs.positionEst, relevantClasses, minConfidence)

    // 3. Tracking Update
    trackingResult := rs.tracker.Update(trackingDetections, timestamp, rs.frameNum)

    // 4. Robot State & Planner Update
    for i := range trackingResult.Tracks {
        // ... update robot state ...
        rs.planner.AddRobot(track.TrackID, worldPos, 0.18)
    }

    // 5. Broadcast tracks to UI
    rs.webServer.BroadcastTracks(trackingResult.Tracks)

    // 6. Compute Velocity with Obstacles
    velocity, _ := rs.planner.ComputeVelocityWithDynamicObstacles(...)

    // 7. Draw results and push to web UI
    overlay := rs.detectionPipe.DrawResults(...)
    rs.webServer.PushRawJPEG(overlay)
}
```

**RobotSystem struct (`cmd/main.go:31-47`):**
```go
type RobotSystem struct {
    cfg           *config.Config
    cam           camera.Camera
    detectionPipe *detection.DetectionPipeline  // ✅ Connected
    tracker       tracking.Tracker              // ✅ Connected
    planner       *planning.Planner              // ✅ Connected
    positionEst   *position.PositionEstimator   // ✅ Connected
    arduino       *controller.ArduinoController // ✅ Connected
    // ...
}
```

---

### UI-003: Robot Footprints Not Displayed in Demo Mode - RESOLVED (Feb 8, 2026)

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | Critical |
| **Impact** | Footprint display feature not working in demo mode |
| **Component** | `cmd/main.go`, `internal/ui/webserver.go` |

**Symptom:**
Frontend never received tracks via WebSocket in demo mode, so no robot footprints were displayed even when tracks were detected and confirmed.

**Root Cause:**
Two bugs in `cmd/main.go`:

1. **Missing `BroadcastTracks` call in `ProcessDemoFrame`**: The demo mode function never called `rs.webServer.BroadcastTracks()`, so tracks were never sent to the frontend.

2. **Missing `PixelRadius` calculation in `ProcessDemoFrame`**: Even if tracks were broadcast, the `PixelRadius` field was only set in `ProcessFrame`, not in `ProcessDemoFrame`.

**Code Location:**
```go
// cmd/main.go:800-823 (before fix - missing BroadcastTracks and PixelRadius)
if rs.tracker != nil {
    trackingDetections := rs.convertFusedToTrackingDetections(result.FusedDetections)
    trackingResult := rs.tracker.Update(trackingDetections, timestamp, rs.frameNum)

    for _, track := range trackingResult.Tracks {
        // PixelRadius never set!
        if track.State == tracking.TrackStateConfirmed && track.TagID != nil {
            // ... no BroadcastTracks call!
        }
    }
}
```

**Fix Applied:**
Added `BroadcastTracks` call and `PixelRadius` calculation in demo mode.

**Verification:**
After fix, logs show:
```
BroadcastTracks: 3 tracks, 3 with pixel_radius
```

---

### UI-004: WebSocket Track Message Format Mismatch - RESOLVED (Feb 8, 2026)

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | Critical |
| **Impact** | Robot footprints not displayed - frontend received tracks but couldn't parse them |
| **Component** | `ui/src/stores/robotStore.ts`, `internal/ui/webserver.go` |

**Symptom:**
Backend logs showed `BroadcastTracks: 3 tracks, 3 with pixel_radius`, but frontend showed `trackCount: 0` and no footprints were rendered.

**Root Cause:**
Backend sends tracks in nested format, but frontend expected flat format. Fixed to handle nested format.

**Fix Applied:**
Modified `ui/src/stores/robotStore.ts` to handle both formats.

---

### UI-003: Obstacle Drawing Disappears If User Pauses - RESOLVED

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | Medium |
| **Impact** | Box being drawn disappears if user takes too long |
| **Component** | `internal/ui/index.go` |

**Symptom:**
When drawing an obstacle box, if the user pauses, the box disappears.

**Root Cause:**
Canvas API clears canvas when dimensions are set, even to same value.

**Fix:**
Modified `syncCanvasSize()` to only update dimensions when changed.

---

### UI-005: Destination Cursor Only Checks Center Point - RESOLVED (Feb 9, 2026)

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | High |
| **Impact** | Destination cursor shows green even when robot footprint overlaps obstacle |
| **Component** | `ui/src/composables/useCanvas.ts` |

**Symptom:**
Green cursor only turned red when center was directly over obstacle, ignoring footprint overlap.

**Fix Applied:**
Added `isCircleInObstacle()` function using circle-rectangle collision detection.

**Test Added:**
Added tests for edge overlap, full containment, corner overlap, cross-edge overlap.

---

### UI-006: Canvas Resize Causes Overlay Misalignment - RESOLVED (Feb 9, 2026)

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | Critical (Blocking) |
| **Impact** | Static obstacles become misaligned when browser window is resized |
| **Component** | `ui/src/composables/useCanvas.ts`, `ui/src/utils/coordinates.ts` |

**Symptom:**
Obstacles drawn on frontend canvas overlay became misaligned with video on resize.

**Solution - Backend-Based Rendering:**
Instead of fixing frontend scaling, obstacles are now drawn **by the backend** directly on the video frame.

**Files Added:**
| File | Purpose |
|------|---------|
| `internal/detection/obstacle_drawer.go` | GOCV implementation - draws obstacles on video |
| `internal/detection/obstacle_drawer_stub.go` | Non-GOCV stub for CI |
| `internal/detection/obstacle_drawer_test.go` | Unit tests |

---

### UI-007: Obstacles Not Visible - Backend Drawing Bug - RESOLVED (Feb 9, 2026)

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | High |
| **Impact** | Static obstacles not displayed on video overlay |
| **Component** | `cmd/main.go` |

**Symptom:**
User reported that obstacles exist in the obstacle list but are not visible on the video display.

**Root Cause:**
In demo mode (`--demo`), `initDemoMode()` did NOT initialize the `detectionPipe` field, so `DrawResults()` was never called.

**Fix Applied:**
Modified `initDemoMode()` to initialize the detection pipeline.

---

## HIGH - Test Coverage Resolved

### TEST-001: Missing Tests - Detection Package - RESOLVED

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | High |
| **Impact** | Test coverage now exists |
| **Component** | `internal/detection/` |

**Resolution:**
All test files exist and are being maintained:

| File | Tests |
|------|-------|
| `types_test.go` | Detection, BoundingBox, Confidence tests |
| `pipeline_test.go` | Pipeline integration tests |
| `dynamic_obstacle_test.go` | Dynamic obstacle conversion |
| `obstacle_drawer_test.go` | Obstacle rendering tests |

---

### TEST-002: Missing Tests - Position Package - RESOLVED

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | High |
| **Impact** | Test coverage now exists |
| **Component** | `internal/position/` |

**Resolution:**
All test files exist and are being maintained:

| File | Tests |
|------|-------|
| `types_test.go` | Point2D, Pose, Velocity, Transform tests |
| `homography_test.go` | Homography matrix tests with known transforms |
| `estimator_test.go` | Position estimator tests |

---

### TEST-003: Missing Tests - Tracking Package - RESOLVED

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | High |
| **Impact** | Test coverage now exists |
| **Component** | `internal/tracking/` |

**Resolution:**
All test files exist and are being maintained:

| File | Tests |
|------|-------|
| `types_test.go` | Track, TrackHistoryPoint tests |
| `kalman_test.go` | Kalman filter prediction/update tests |
| `hungarian_test.go` | Assignment algorithm tests |
| `bytetrack_test.go` | ByteTrack tracking scenarios |

---

### TEST-004: Missing Tests - Planning Package - PARTIAL

| Field | Value |
|-------|-------|
| **Status** | Partially Resolved |
| **Severity** | High |
| **Impact** | Partial test coverage exists |
| **Component** | `internal/planning/` |

**Resolution:**
Most test files exist:

| File | Status |
|------|--------|
| `types_test.go` | ✅ Exists |
| `astar_test.go` | ✅ Exists |
| `collision_test.go` | ✅ Exists |
| `dynamic_obstacle_test.go` | ✅ Exists |
| `local_test.go` | ❌ Missing |
| `coordinator_test.go` | ❌ Missing |

---

### CAM-001: Camera Interface Has No Error Handling - RESOLVED

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | Medium |
| **Impact** | Error handling already implemented |
| **Component** | `internal/camera/camera.go` |

**Resolution:**
Camera interface already has proper error returns:

```go
type Camera interface {
    Start() error
    Stop()
    GetFrame() (*Frame, error)
    GetRawJPEG() ([]byte, error)
    GetName() string
    IsConnected() bool
    GetWidth() int
    GetHeight() int
    GetFPS() int
}

type CameraError struct {
    Message string
}

func (e *CameraError) Error() string {
    return e.Message
}
```

Error types and propagation are already implemented.

---

## Patterns (Archived)

### PATTERN-010: Bbox Center Calculation Operator Precedence Bug - RESOLVED

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | Critical |
| **Impact** | Robot world positions computed incorrectly; planned paths start from wrong location and go off screen |
| **Component** | `cmd/main.go`, `cmd/demo_mode.go` |

**Symptom:**
Paths generated by A* planner go off screen to the right instead of connecting the robot to its destination.

**Root Cause:**
`track.Bbox` stores `[X1, Y1, X2, Y2]` (corner coordinates). The center calculation `track.Bbox[0]+track.Bbox[2]/2` computes `X1 + (X2/2)` due to Go's operator precedence (division before addition), instead of the correct `(X1+X2)/2`.

**Fix Applied:**
Changed all instances from:
```go
px, py := track.Bbox[0]+track.Bbox[2]/2, track.Bbox[1]+track.Bbox[3]/2
```
to:
```go
px, py := (track.Bbox[0]+track.Bbox[2])/2, (track.Bbox[1]+track.Bbox[3])/2
```

Files fixed: `cmd/main.go` (3 instances), `cmd/demo_mode.go` (2 instances).

---

## Related Documents

- [BUGS.md](BUGS.md) - Active bug tracker with open issues
- [PLAN.md](PLAN.md) - Implementation plan and architecture
- [AGENTS.md](AGENTS.md) - Agent guidelines and build commands
