# Bug Tracker - Robot Tracker Go

## Overview

This document tracks known bugs, issues, and technical debt in the Robot Tracker Go project.

## Bug Categories

- **[CRITICAL](#critical)** - Blocks core functionality
- **[HIGH](#high)** - Significant impact, workarounds available
- **[MEDIUM](#medium)** - Moderate impact, planned fix
- **[LOW](#low)** - Minor impact, cosmetic or documentation

---

## CRITICAL

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

## HIGH

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

### TEST-001: Missing Tests - Detection Package

| Field | Value |
|-------|-------|
| **Status** | In Progress |
| **Severity** | High |
| **Impact** | No test coverage for detection types and pipeline |
| **Component** | `internal/detection/` |

**Missing Tests:**
- `detection/types_test.go` - Detection, BoundingBox, Confidence tests
- `detection/pipeline_test.go` - Pipeline integration tests
- `detection/apriltag_test.go` - AprilTag detector tests
- `detection/yolo_test.go` - YOLO detector tests

**Fix Plan:**
1. Create `detection/types_test.go` with table-driven tests
2. Create `detection/pipeline_test.go` with mock inputs
3. Skip actual detector tests until implementation complete

---

### TEST-002: Missing Tests - Position Package

| Field | Value |
|-------|-------|
| **Status** | Pending |
| **Severity** | High |
| **Impact** | No test coverage for position types and math |
| **Component** | `internal/position/` |

**Missing Tests:**
- `position/types_test.go` - Point2D, Pose, Velocity, Transform tests
- `position/homography_test.go` - Homography matrix tests
- `position/estimator_test.go` - Position estimator tests

**Fix Plan:**
1. Create `position/types_test.go` with edge cases
2. Create `position/homography_test.go` with known transform tests
3. Create `position/estimator_test.go` with calibration data

---

### TEST-003: Missing Tests - Tracking Package

| Field | Value |
|-------|-------|
| **Status** | Pending |
| **Severity** | High |
| **Impact** | No test coverage for ByteTrack, Kalman, Hungarian |
| **Component** | `internal/tracking/` |

**Missing Tests:**
- `tracking/types_test.go` - Track, TrackHistoryPoint tests
- `tracking/kalman_test.go` - Kalman filter tests
- `tracking/hungarian_test.go` - Assignment algorithm tests
- `tracking/bytetrack_test.go` - ByteTrack tests

**Fix Plan:**
1. Create `tracking/types_test.go` with track lifecycle tests
2. Create `tracking/kalman_test.go` with prediction/update tests
3. Create `tracking/hungarian_test.go` with known assignments
4. Create `tracking/bytetrack_test.go` with tracking scenarios

---

### TEST-004: Missing Tests - Planning Package

| Field | Value |
|-------|-------|
| **Status** | Pending |
| **Severity** | High |
| **Impact** | No test coverage for path planning |
| **Component** | `internal/planning/` |

**Missing Tests:**
- `planning/types_test.go` - Path, Node, VelocityObstacle tests
- `planning/astar_test.go` - A* pathfinding tests
- `planning/collision_test.go` - Collision detection tests
- `planning/local_test.go` - Local planner tests
- `planning/coordinator_test.go` - Multi-robot coordination tests

**Fix Plan:**
1. Create `planning/types_test.go` with data structure tests
2. Create `planning/astar_test.go` with known paths
3. Create `planning/collision_test.go` with obstacle scenarios
4. Create `planning/local_test.go` with velocity scenarios
5. Create `planning/coordinator_test.go` with multi-robot scenarios

---

## MEDIUM

### CAM-001: Camera Interface Has No Error Handling

| Field | Value |
|-------|-------|
| **Status** | Open |
| **Severity** | Medium |
| **Impact** | No graceful degradation on camera failure |
| **Component** | `internal/camera/camera.go` |

**Symptom:**
Camera interface methods return empty values on failure, no error propagation.

**Fix Plan:**
- Add error returns to Camera interface methods
- Implement graceful fallback in main.go

---

### UI-001: WebSocket Connection Has No Reconnect Logic

| Field | Value |
|-------|-------|
| **Status** | Open |
| **Severity** | Medium |
| **Impact** | Clients cannot reconnect after disconnection |
| **Component** | `internal/ui/websocket.go` |

**Fix Plan:**
- Add WebSocket reconnect logic to frontend
- Implement heartbeat/ping-pong in WebSocket handler

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
```go
// cmd/main.go:800-830 (after fix)
if rs.tracker != nil {
    trackingDetections := rs.convertFusedToTrackingDetections(result.FusedDetections)
    trackingResult := rs.tracker.Update(trackingDetections, timestamp, rs.frameNum)

    for i := range trackingResult.Tracks {
        track := &trackingResult.Tracks[i]
        if track.State == tracking.TrackStateConfirmed && track.TagID != nil {
            rs.CurrentRobotID = *track.TagID
            if rs.positionEst != nil {
                px, py := track.Bbox[0]+track.Bbox[2]/2, track.Bbox[1]+track.Bbox[3]/2
                worldPos := rs.positionEst.PixelToWorld(px, py)
                rs.positionEst.UpdatePosition(track.TrackID, worldPos.X, worldPos.Y)
                track.WorldPos = [2]float64{worldPos.X, worldPos.Y}
                if robotConfig := rs.cfg.GetRobotByTagID(*track.TagID); robotConfig != nil {
                    track.PixelRadius = (robotConfig.Diameter / 2) * rs.cfg.YOLO.PixelsPerMeter
                }
            }
        }
    }

    rs.webServer.BroadcastTracks(trackingResult.Tracks)
}
```

**Verification:**
After fix, logs show:
```
BroadcastTracks: 3 tracks, 3 with pixel_radius
```

**Related Files:**
- `cmd/main.go` - Added missing `BroadcastTracks` call and `PixelRadius` calculation
- `internal/ui/webserver.go` - Already had `BroadcastTracks` method
- `ui/src/composables/useCanvas.ts` - `renderFootprints()` function (already implemented)
- `ui/src/stores/robotStore.ts` - `confirmedTracks` computed property (already implemented)
- `ui/src/stores/uiStore.ts` - `showFootprints` state (already implemented)

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
Backend sends tracks in nested format:
```json
{
  "type": "tracks",
  "tracks": {
    "tracks": [...],
    "count": 3
  }
}
```

But frontend expected flat format:
```json
{
  "type": "tracks",
  "tracks": [...]
}
```

The frontend's `handleWebSocketMessage()` was checking `Array.isArray(data.tracks)` which returned false for the nested object, so tracks were never assigned to the store.

**Fix Applied:**
Modified `ui/src/stores/robotStore.ts` to handle both formats:
```typescript
case 'tracks':
    let tracksArray: Track[] | undefined
    if (Array.isArray((data as any).tracks)) {
        // Flat format
        tracksArray = (data as any).tracks
    } else if ((data as any).tracks && typeof (data as any).tracks === 'object') {
        // Nested format from backend
        tracksArray = (data as any).tracks.tracks
    }
    if (Array.isArray(tracksArray)) {
        tracks.value = tracksArray
    }
    break
```

**Verification:**
After fix, logs show:
```
[WebSocket] Setting tracks, count: 3
[Watch] Tracks changed, calling render(), tracks: 3 confirmed: 3
[Footprint] renderFootprints called: {trackCount: 3, showFootprints: true, ...}
```

**Related Files:**
- `ui/src/stores/robotStore.ts` - Updated `handleWebSocketMessage()` to parse nested tracks format
- `ui/src/types/api.ts` - `TracksMessage` interface (unchanged)

---

### UI-002: MJPEG Stream Has No Error Recovery

| Field | Value |
|-------|-------|
| **Status** | Open |
| **Severity** | Medium |
| **Impact** | Stream fails permanently on capture error |
| **Component** | `internal/ui/mjpeg.go` |

**Fix Plan:**
- Add error recovery and stream restart in MJPEG server

---

### UI-003: Obstacle Drawing Disappears If User Pauses

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | Medium |
| **Impact** | Box being drawn disappears if user takes too long |
| **Component** | `internal/ui/index.go` |

**Symptom:**
When drawing an obstacle box, if the user pauses for a moment (e.g., thinking about where to position the box), the box disappears from the overlay even though the mouse button is still held down.

**Root Cause:**
Canvas API behavior - setting `overlay.width` or `overlay.height` always clears the canvas:
1. `syncCanvasSize()` runs every second via `setInterval(syncCanvasSize, 1000)`
2. When it sets `overlay.width = rect.width` (even to the same value), the canvas is automatically cleared
3. This happens regardless of whether the user is actively drawing
4. The drawing rectangle is lost when the canvas is cleared

**Fix:**
Modified `syncCanvasSize()` to only update dimensions when they've actually changed:
```javascript
// Before (buggy):
function syncCanvasSize() {
    var rect = video.getBoundingClientRect();
    if (rect.width > 0 && rect.height > 0) {
        overlay.width = rect.width;
        overlay.height = rect.height;
    }
}

// After (fixed):
function syncCanvasSize() {
    var rect = video.getBoundingClientRect();
    if (rect.width > 0 && rect.height > 0) {
        if (overlay.width !== rect.width || overlay.height !== rect.height) {
            overlay.width = rect.width;
            overlay.height = rect.height;
            if (currentDraw) {
                redrawOverlay();
            }
        }
    }
}
```

**Code Changes:**
1. Added dimension change check before setting `overlay.width/height`
2. Added `redrawOverlay()` call after dimension change to restore drawing state

---

## LOW

### UI-005: Destination Cursor Only Checks Center Point - RESOLVED (Feb 9, 2026)

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | High |
| **Impact** | Destination cursor shows green even when robot footprint overlaps obstacle |
| **Component** | `ui/src/composables/useCanvas.ts` |

**Symptom:**
When setting a destination, the green cursor circle only turned red when the mouse cursor center was directly over an obstacle. However, if any part of the robot's footprint (the circle representing robot size) overlapped an obstacle, it should also show red.

**Root Cause:**
The `isPointInObstacle()` function only checked if the center point (mouse position) was inside an obstacle:
```typescript
// OLD - only checks center point
function isPointInObstacle(x: number, y: number): boolean {
    // Only checks if point (x,y) is inside any obstacle
    return x >= obs.pixel_top_left[0] && x <= obs.pixel_bottom_right[0] && ...
}
```

**Fix Applied:**
Added `isCircleInObstacle()` function that uses circle-rectangle collision detection:
```typescript
function isCircleInObstacle(cx: number, cy: number, radius: number): boolean {
    // Find closest point on rectangle to circle center
    const closestX = Math.max(rectX1, Math.min(cx, rectX2))
    const closestY = Math.max(rectY1, Math.min(cy, rectY2))

    // Calculate distance from closest point to circle center
    const distanceSquared = (cx - closestX)² + (cy - closestY)²

    // Collision if distance < radius
    return distanceSquared < radius * radius
}
```

Updated `renderDestinationCursor()` to use the new function:
```typescript
const isInvalid = isCircleInObstacle(pos.x, pos.y, radius)
```

**Test Added:**
Added `ui/src/composables/__tests__/useCanvas.test.ts` with FOOTPRINT-001 test cases:
- Edge overlap when center is outside
- Full containment
- Corner overlap
- Cross-edge overlap
- Comparison test demonstrating old vs new behavior

**Verification:**
```
✅ Edge overlap detection - circle extends into obstacle
✅ Full containment - circle completely inside obstacle
✅ Corner overlap - circle corner touches obstacle corner
✅ Partial overlap - circle crosses edge
✅ No overlap - circle completely outside
✅ All 7 tests passing
```

---

### UI-006: Canvas Resize Causes Overlay Misalignment - RESOLVED (Feb 9, 2026)

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | Critical (Blocking) |
| **Impact** | Static obstacles become misaligned when browser window is resized |
| **Component** | `ui/src/composables/useCanvas.ts`, `ui/src/utils/coordinates.ts` |

**Symptom:**
When the browser window is resized, obstacles drawn on the frontend canvas overlay became misaligned with the video.

**Root Cause:**
Frontend canvas overlay doesn't scale with the video - only AprilTags (drawn by backend) remained aligned.

**Solution - Backend-Based Rendering:**
Instead of trying to fix frontend scaling, obstacles are now drawn **by the backend** directly on the video frame, just like AprilTags:

```
Backend (Go):
  1. Receive obstacle coordinates from frontend
  2. Draw obstacles on video frame using pixel coordinates
  3. Send annotated frame to frontend
  4. Obstacles scale perfectly - they're part of the video!

Frontend:
  1. Send obstacle coordinates to backend
  2. Display received video frame
  3. No scaling logic needed!
```

**Files Added:**
| File | Purpose |
|------|---------|
| `internal/detection/obstacle_drawer.go` | GOCV implementation - draws obstacles on video |
| `internal/detection/obstacle_drawer_stub.go` | Non-GOCV stub for CI |
| `internal/detection/obstacle_drawer_test.go` | Unit tests |

**Files Modified:**
| File | Changes |
|------|---------|
| `internal/detection/types.go` | Added `Obstacle` struct and `SetObstacles()` method |
| `internal/detection/pipeline.go` | Draw obstacles in `DrawResults()` |
| `cmd/main.go` | Pass obstacles from webserver to detection pipeline |
| `ui/src/composables/useCanvas.ts` | Removed frontend obstacle rendering |

**Benefits:**
| Benefit | Description |
|--------|-------------|
| **Perfect Alignment** | Obstacles are part of the video frame itself |
| **No Scaling Logic** | Backend handles all coordinate transforms |
| **Consistent Architecture** | Same approach as AprilTags |
| **Simpler Frontend** | Removed 50+ lines of scaling code |

**Verification:**
```
✅ All 19 integration tests pass
✅ All Vue unit tests pass
✅ Build succeeds
✅ Obstacles drawn by backend match AprilTag alignment
```

**Note:** The frontend still uses the canvas overlay for:
- Destination cursor (green/red circle)
- Destination marker (purple goal circle)
- Drawing new obstacles (user interaction)
- Calibration tag overlays

Only static obstacles are now rendered by the backend.

**Benefits of this architecture:**

| Benefit | Description |
|---------|-------------|
| **Single Source of Truth** | All coordinate conversions use the same utility functions |
| **Future-Proof** | New features simply import the utilities |
| **Testable** | Utilities can be unit tested independently |
| **Consistent** | All render functions use the same pattern |
| **Maintainable** | Bug fixes apply everywhere automatically |

**Files Changed:**

| File | Changes |
|------|---------|
| `ui/src/utils/coordinates.ts` | NEW - Centralized coordinate utilities |
| `ui/src/composables/useCanvas.ts` | Refactored to use utilities |
| `ui/src/components/VideoOverlay.vue` | Uses `canvasToNatural` from composable |
| `ui/src/stores/robotStore.ts` | Inlined conversion (store context) |

**Pattern for Future Development:**

```typescript
import { canvasToNatural, naturalToCanvas } from '@/utils/coordinates'

// When rendering data from backend (stored in natural coords):
const scaled = naturalToCanvas(data.x, data.y)
ctx.drawImage(..., scaled.x, scaled.y)

// When capturing user input:
const natural = canvasToNatural(mouseX, mouseY)
// Send natural coords to backend
```

**Note:** Existing obstacles drawn before this fix will appear misaligned until the next draw operation. New obstacles use the correct coordinate system.

**Verification:**
```
✅ All 19 integration tests pass
✅ All Vue unit tests pass
✅ Build succeeds
✅ Centralized utilities are tested
```

2. **Scale obstacles during render** (`useCanvas.ts:114-150`):
```typescript
const scaleX = dimensions.value.width / naturalWidth
const scaleY = dimensions.value.height / naturalHeight
const scaledX1 = x1 * scaleX
const scaledY1 = y1 * scaleY
```

3. **Scale destinations during render** (`useCanvas.ts:257-286`):
```typescript
const scaledX = dest.x * scaleX
const scaledY = dest.y * scaleY
```

4. **Scale collision detection** (`useCanvas.ts:228-237`):
```typescript
const naturalX = canvasX / scaleX
const naturalY = canvasY / scaleY
isCircleInObstacle(naturalX, naturalY, naturalRadius)
```

5. **Scale click detection** (`useCanvas.ts:419-470`):
```typescript
const scaledX1 = x1 * scaleX
const scaledY1 = y1 * scaleY
```

**Files Changed:**
| File | Changes |
|------|---------|
| `ui/src/components/VideoOverlay.vue` | Convert canvas → natural coords before API call |
| `ui/src/composables/useCanvas.ts` | Scale all rendering functions |
| `ui/src/stores/robotStore.ts` | Convert canvas → natural coords in confirmDestination |

**Note:** Existing obstacles drawn before this fix will appear misaligned until the next draw operation. New obstacles will use the correct coordinate system.

**Verification:**
```
✅ All tests pass
✅ Build succeeds
✅ Render functions now use consistent scaling
```

---

### DOC-001: API Documentation Incomplete

| Field | Value |
|-------|-------|
| **Status** | Open |
| **Severity** | Low |
| **Impact** | Hard to understand API usage |
| **Component** | All packages |

**Fix Plan:**
- Add godoc comments to all exported types and functions
- Add example code to package documentation

---

### CONFIG-001: No Validation for YAML Configuration

| Field | Value |
|-------|-------|
| **Status** | Open |
| **Severity** | Low |
| **Impact** | Silent failures on invalid config |
| **Component** | `internal/config/config.go` |

**Fix Plan:**
- Add validation for required fields
- Add default values for optional fields

---

## Test Checklist

### Package Test Status

| Package | Status | Test Files | Coverage |
|---------|--------|------------|----------|
| `config` | ✅ Complete | `config_test.go` | 100% |
| `controller` | ✅ Complete | `protocol_test.go`, `arduino_test.go` | 100% |
| `camera` | ⚠️ Partial | `camera_test.go`, `gocv_skip_test.go` | 0% (gocv blocked) |
| `detection` | ❌ Missing | None | 0% |
| `position` | ❌ Missing | None | 0% |
| `tracking` | ❌ Missing | None | 0% |
| `planning` | ❌ Missing | None | 0% |
| `ui` | ❌ Missing | None | 0% |

### Test Patterns

Use table-driven tests for consistent coverage:

```go
func TestFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    Type
        expected Type
    }{
        {"case1", input1, expected1},
        {"case2", input2, expected2},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Function(tt.input)
            if result != tt.expected {
                t.Errorf("Function(%v) = %v, want %v", tt.input, result, tt.expected)
            }
        })
    }
}
```

---

### UI-007: Obstacles Not Visible - Backend Drawing Bug - RESOLVED (Feb 9, 2026)

| Field | Value |
|-------|-------|
| **Status** | Resolved |
| **Severity** | High |
| **Impact** | Static obstacles not displayed on video overlay |
| **Component** | `cmd/main.go` |

**Symptom:**
User reported that obstacles exist in the obstacle list but are not visible on the video display, despite being listed and added via the UI.

**Root Cause:**
In demo mode (`--demo`), the `initDemoMode()` function was called instead of `Initialize()`. However, `initDemoMode()` only created the web server but did NOT initialize the `detectionPipe` field.

Since `rs.detectionPipe` was nil:
1. The check `if rs.detectionPipe != nil && rs.webServer != nil` in `ProcessDemoFrame()` failed
2. `DrawResults()` was never called
3. Obstacles were never drawn on the video

**Fix Applied:**
Modified `initDemoMode()` to also initialize the detection pipeline:

```go
// cmd/main.go:139-175 (AFTER FIX)
func (rs *RobotSystem) initDemoMode() {
    rs.webServer = ui.NewWebServer(":9086")

    tagConfig := detection.AprilTagConfig{
        Family:       "tag36h11",
        QuadDecimate: 2.0,
    }
    rs.detectionPipe = detection.NewDetectionPipeline(nil, tagConfig)
    log.Printf("Demo mode: Detection pipeline initialized (YOLO disabled)")

    rs.webServer.Start()
    log.Print(getWebUIURLs("9086"))

    rs.webServer.OnObstaclesChanged = func(obstacles []planning.Obstacle) {
        rs.planner.SetObstacles(obstacles)

        detectionObstacles := make([]detection.Obstacle, len(obstacles))
        for i, obs := range obstacles {
            detectionObstacles[i] = detection.Obstacle{
                ID:               obs.Name,
                PixelTopLeft:     [2]int{obs.PixelsTopLeft[0], obs.PixelsTopLeft[1]},
                PixelBottomRight: [2]int{obs.PixelsBottomRight[0], obs.PixelsBottomRight[1]},
                WorldTopLeft:     obs.WorldTopLeft,
                WorldBottomRight: obs.WorldBottomRight,
                Clearance:        0.05,
            }
        }
        rs.detectionPipe.SetObstacles(detectionObstacles)
    }

    rs.webServer.OnDestinationSet = func(robotID int, pixelPos [2]float64) {
        log.Printf("Demo mode: Destination set for robot %d at pixel(%d,%d)",
            robotID, int(pixelPos[0]), int(pixelPos[1]))
    }
}
```

**Why Integration Tests Didn't Catch This Earlier:**
The integration tests for `SetObstacles()` and `DrawResults()` tested the detection pipeline in isolation. They did NOT test the runtime behavior where `initDemoMode()` was called without initializing the pipeline.

**Files Changed:**
| File | Changes |
|------|---------|
| `cmd/main.go` | `initDemoMode()` now initializes detection pipeline and sets up `OnObstaclesChanged` callback |

**Tests Added:**
| Test File | Tests |
|----------|-------|
| `cmd/main_test.go` | `TestConvertDemoObstacles_ToDetection` and related tests |
| `internal/detection/pipeline_test.go` | `TestDetectionPipeline_SetObstacles_Integration` (5 subtests) |
| `internal/detection/pipeline_test.go` | `TestDetectionPipeline_DrawResults_Obstacles_Integration` (4 subtests) |

**Verification:**
```
✅ All detection pipeline tests pass (13 tests)
✅ All Vue UI tests pass (68 tests)
✅ Demo mode now initializes detection pipeline
✅ Obstacles from UI are converted and stored in pipeline
```

---

## Resolution Procedures

### When Fixing a Bug

1. Add bug entry to this file with ID (e.g., `GOCV-001`)
2. Describe symptom, root cause, and fix
3. Update checklist if new pattern applies
4. Create unit tests to prevent regression
5. Create integration tests if applicable
6. Mark bug as resolved with date and PR#

### Bug ID Format

- **GOCV-XXX**: GoCV/OpenCV related issues
- **DET-XXX**: Detection related issues
- **PIPELINE-XXX**: Pipeline integration issues
- **TEST-XXX**: Test coverage issues
- **CAM-XXX**: Camera related issues
- **UI-XXX**: User interface issues
- **CONFIG-XXX**: Configuration issues
- **DOC-XXX**: Documentation issues
- **PERF-XXX**: Performance issues

---

## Related Documents

- [PLAN.md](PLAN.md) - Implementation plan and architecture
- [AGENTS.md](AGENTS.md) - Agent guidelines and build commands
- `internal/camera/camera.go` - Camera interface
- `internal/detection/` - Detection pipeline
- `internal/tracking/` - Tracking algorithms
- `internal/planning/` - Path planning
