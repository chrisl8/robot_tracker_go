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

### DET-002: Stubbed YOLO Detector

| Field | Value |
|-------|-------|
| **Status** | Open |
| **Severity** | Critical |
| **Impact** | No obstacle detection, path planning has no input |
| **Component** | `internal/detection/yolo.go` |

**Symptom:**
`Detect()` method returns empty slice for all inputs.

**Root Cause:**
YOLO detector is stubbed - returns empty array without processing.

**Code Location:**
```go
// internal/detection/yolo.go:50-58
func (d *YOLODetector) Detect(frame *gocv.Mat) []Detection {
    // Stub: returns empty slice
    return []Detection{}
}
```

**Fix Required:**
1. Use ONNX Runtime Go for inference
2. Load YOLOv8n.onnx model from assets/
3. Implement preprocessing and postprocessing

---

## HIGH

### PIPELINE-001: No Camera → Detection → Tracking Integration

| Field | Value |
|-------|-------|
| **Status** | Open |
| **Severity** | High |
| **Impact** | Complete pipeline not connected in main.go |
| **Component** | `cmd/main.go` |

**Symptom:**
main.go:
- Creates test pattern generator (demo mode)
- Initializes Arduino controller
- Starts web server
- Does NOT capture frames from camera
- Does NOT run detection pipeline
- Does NOT update tracking

**Target Integration:**
```go
// In main loop (goroutine):
for {
    frame := camera.Capture()
    detections := detectionPipeline.Run(frame)
    tracking.Update(detections)
    positions := positionEstimator.Estimate(detections)
    planner.Update(positions)
    commands := executor.ComputeCommands(planner.GetVelocities())
    controller.SendCommands(commands)
}
```

**Fix Required:**
Connect the pipeline components once detection is implemented.

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
