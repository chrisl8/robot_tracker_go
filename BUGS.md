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
   go build -o robot_tracker.exe ./cmd/main.go
   ```

**Related Files:**
- `internal/camera/camera.go`
- `internal/camera/gocv_camera.go`
- `scripts/install-opencv-for-gocv.ps1`
- `diagnostics/DIAGNOSTIC_RESULTS.md`

---

### DET-001: Stubbed AprilTag Detector

| Field | Value |
|-------|-------|
| **Status** | Open |
| **Severity** | Critical |
| **Impact** | No robot detection, tracking has no input |
| **Component** | `internal/detection/apriltag.go` |

**Symptom:**
`Detect()` method returns empty slice for all inputs.

**Root Cause:**
AprilTag detector is stubbed - returns empty array without processing.

**Code Location:**
```go
// internal/detection/apriltag.go:21-29
func (d *AprilTagDetector) Detect(frame *gocv.Mat) []Detection {
    // Stub: returns empty slice
    return []Detection{}
}
```

**Fix Required:**
1. Use AprilTag C library bindings (github.com/apriltags/apriltag-go) or
2. Implement tag detection algorithm in Go

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
