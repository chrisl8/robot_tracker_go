# Archived Implementation Plan - Robot Tracker Go

> This file contains historical implementation notes, bug fix history, and detailed phase documentation. See [PLAN.md](PLAN.md) for the current architecture and summary.

---

## Original Planning Phases

### Phase 1: Foundation (Complete)
- Configuration (YAML loading)
- Serial Protocol
- Arduino Controller
- Command Queue

### Phase 2: Core Mathematics (Complete)
- Position types (Point2D, Pose, Velocity)
- Homography transforms
- Position Estimator

### Phase 3: Detection Pipeline (Complete)
- AprilTag detector
- YOLO detector
- Unified detection pipeline

### Phase 4: Tracking (Complete)
- ByteTrack implementation
- Kalman Filter
- Hungarian Algorithm

### Phase 5: Path Planning (Complete)
- A* pathfinding
- Local Planner (Velocity Obstacles)
- Coordinator for multi-robot
- Collision Detection

### Phase 6: Web UI (Complete)
- Gin Web Server
- MJPEG Streaming
- WebSocket Overlay
- REST API

---

## Historical Bug Fixes

### GoCV Build Failure - Feb 6, 2026

**Problem:** GoCV failed to build with undefined constants.

**Root Cause:** Missing environment configuration (GCC and OpenCV DLLs not in PATH).

**Resolution:**
- Add MinGW GCC to PATH: `C:\mingw64\bin`
- Add OpenCV DLLs to PATH: `C:\opencv\build\install\x64\mingw\bin`
- Build command: `set PATH=C:\mingw64\bin;C:\opencv\build\install\x64\mingw\bin;%PATH% && go build ...`

### GoCV/OpenCV Windows DLL Crash - Feb 6, 2026

**Problem:** Application crashed with `0xc0000005` access violation.

**Resolution:**
- Replaced GoCV JPEG encoding with pure Go `image/jpeg` encoder
- Fixed `cameraFrameToImage()` to properly convert BGR to RGBA
- Added `GetRawJPEG()` method for efficient JPEG retrieval

### Calibration Persistence - Feb 6, 2026

**Problem:** Calibration files saved but not loaded at startup.

**Root Causes:**
1. YAML 2D homography parsing bug - saved YAML had 2D array, loader expected flat
2. Initialization order - camera initialized AFTER PositionEstimator
3. Frontend not checking on load - only polled every 5 seconds
4. Missing WebSocket calibration handler

**Files Changed:**
- `internal/position/estimator.go:99-112` - Fixed 2D YAML parsing
- `cmd/main.go:82-140` - Reordered initialization
- `internal/ui/index.go:1368` - Added immediate check

### Obstacle Drawing Disappears - Feb 7, 2026

**Problem:** Drawing box disappears if user pauses.

**Root Cause:**
1. `redrawOverlay()` clears canvas on WebSocket updates
2. `mouseleave` handler cleared drawing state
3. Timing issues during active drawing

**Solution:**
1. Added `isDrawing` flag
2. Added `lastDrawnRect` variable
3. Modified `redrawOverlay()` to skip full redraw when drawing
4. Removed `mouseleave` handler

### WebSocket Track Format Mismatch - Feb 8, 2026

**Problem:** Frontend couldn't parse nested track format.

**Root Cause:** Backend sends nested format, frontend expected flat.

**Fix:** Modified `robotStore.ts` to handle both formats.

### Robot Footprints Not Displayed - Feb 8, 2026

**Problem:** Footprint display not working in demo mode.

**Root Causes:**
1. Missing `BroadcastTracks` call in `ProcessDemoFrame`
2. Missing `PixelRadius` calculation in `ProcessDemoFrame`

### Bbox Operator Precedence Bug - Feb 11, 2026

**Problem:** Center calculation `track.Bbox[0]+track.Bbox[2]/2` computed wrong value.

**Root Cause:** Go evaluates division before addition.

**Fix:** Changed to `(track.Bbox[0]+track.Bbox[2])/2`

---

## Phase 8: Web-Based Calibration Details

### Features Implemented
- Clickable calibration badge
- Conditional instructions panel
- AprilTag measurement diagram
- Default 15cm tag size
- Multi-tag detection and selection
- Visual feedback for selected tag
- Toast notifications
- Tag expiration (2 seconds)
- Draggable dialogue
- Pin/Unpin toggle button
- Calibration persistence
- Camera-specific calibration files
- Auto-load calibration on startup

### API Endpoints

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/api/calibration/status` | Get calibration state |
| POST | `/api/calibration/start` | Start calibration mode |
| GET | `/api/calibration/detected-tags` | Get list of detected tags |
| POST | `/api/calibration/compute` | Compute homography |
| POST | `/api/calibration/save` | Save calibration to file |
| POST | `/api/calibration/cancel` | Cancel calibration |

---

## Phase 9: Dynamic Obstacle Detection Details

### Components Implemented
- `DynamicObstacle` struct
- `LocalPlanner.ComputeVelocityWithObstacles()`
- `Planner.ComputeVelocityWithDynamicObstacles()`
- YOLO-to-Obstacle converter
- 11 test cases (all passing)
- Self-test mode
- Demo-YOLO mode

### Static Obstacle UI

**Features:**
- Obstacle Drawing UI (click and drag)
- Obstacle Management (list, delete, clear)
- Keyboard shortcuts (Z, C, S)
- Visual feedback (red dashed rectangles)
- YAML persistence

---

## Phase 10: Linux Migration Details

### Final Status

| Component | Windows Status | Linux Status |
|-----------|---------------|--------------|
| Go runtime | ✅ 1.23.2+ | ✅ 1.25.7 |
| GCC compiler | ✅ MinGW | ✅ system GCC |
| OpenCV | ✅ 4.13.0 | ✅ 4.13.0 |
| GoCV | ✅ Configured | ✅ Working |
| Serial ports | ✅ Working | ✅ Working |
| Demo mode | ✅ Working | ✅ Working |

### Serial Port Detection

Implemented functions in `internal/controller/arduino.go`:
- `detectPorts()` - scans `/dev/tty*`
- `autoDetectPort()` - returns first Arduino-like port
- `ListPorts()` - returns all detected ports

### Wrapper Scripts Created

| Script | Purpose |
|--------|---------|
| `scripts/build.sh` | Build with GoCV support |
| `scripts/run.sh` | Run with environment setup |
| `scripts/test.sh` | Run tests |
| `scripts/install-opencv.sh` | Install OpenCV 4.13.0 |

---

## Phase 11: Vue 3 UI Migration Details

### Goals Achieved
- Type Safety - TypeScript throughout
- Maintainability - Component-based architecture
- State Management - Pinia stores
- Developer Experience - Vue 3 Composition API
- Performance - Vite build system

### Vue 3 Architecture

```
ui/
├── src/
│   ├── main.ts              # App entry point
│   ├── App.vue              # Root component
│   ├── types/               # TypeScript types
│   ├── stores/              # Pinia stores
│   ├── composables/         # Vue composables
│   ├── components/          # Vue components
│   └── styles/              # SCSS styles
├── package.json
├── tsconfig.json
└── vite.config.ts
```

### Files Created

| Category | Files |
|----------|-------|
| Configuration | `package.json`, `tsconfig.json`, `vite.config.ts` |
| Entry | `index.html`, `main.ts`, `App.vue` |
| Types | 4 files in `types/` |
| Stores | 3 files in `stores/` |
| Composables | 2 files in `composables/` |
| Components | 8 files in `components/` |
| Styles | 2 files in `styles/` |

### Backend Changes
- `internal/ui/embed.go` - embeds static Vue build
- `internal/ui/webserver.go` - serves static files
- `internal/ui/index.go` - **Deleted** (1687 lines)

### Build Output

```
../internal/ui/static/index.html                   0.47 kB
../internal/ui/static/assets/index-*.css         372.80 kB
../internal/ui/static/assets/index-*.js          999.10 kB
```

### Unit Tests (15 tests passing)

| Test File | Tests |
|-----------|-------|
| `src/types/__tests__/api.test.ts` | 5 tests |
| `src/types/__tests__/robot.test.ts` | 5 tests |
| `src/types/__tests__/obstacle.test.ts` | 5 tests |

### Integration Tests (Playwright)

| Test Suite | Description |
|------------|-------------|
| `tests/app.spec.ts` | Main page, UI elements, navigation |
| API Endpoints | Calibration, obstacles, status |
| Video Stream | Video and overlay canvas |
| Control Panel | Manual controls |

---

## Phase 12: Robot Footprint Display Details

### Requirements
- Visual Style: Semi-transparent cyan circles (`rgba(0, 255, 255, 0.2)`)
- Toggleable via UI button
- Backend pre-computes pixel radius
- Only confirmed tracks

### Implementation Tasks
1. Update Track struct (`internal/tracking/types.go`)
2. Populate new fields in ProcessFrame
3. Update TrackMessage (`internal/ui/webserver.go`)
4. Update Track interface (`ui/src/types/api.ts`)
5. Add showFootprints toggle (`ui/src/stores/uiStore.ts`)
6. Add renderFootprints() (`ui/src/composables/useCanvas.ts`)
7. Add toggle button (`ui/src/components/ControlPanel.vue`)

### Rendering Order
1. `renderObstacles()` - Static obstacles (red)
2. `renderFootprints()` - Robot footprints (cyan)
3. `renderDrawingBox()` - User selection (orange)
4. `renderTracks()` - Bounding boxes

---

## Phase 14: Code Cleanup Details

### Issues Fixed

| Category | Count |
|----------|-------|
| Dead code (unused calculations) | 2 |
| Duplicate helper functions | 8 |
| Unused diagnostic files | 5 |
| Redundant build tags | 2 |
| Empty/stub functions | 2 |
| Unused helper functions | 1 |
| Redundant conditionals | 1 |

### Dead Code Fixed

**cmd/main.go:557** - Velocity computed but never used:
```go
// BEFORE:
velocity, _ := rs.planner.ComputeVelocityWithDynamicObstacles(...)
_ = velocity  // DEAD: result is discarded

// AFTER: Remove velocity variable entirely
rs.planner.ComputeVelocityWithDynamicObstacles(...)
```

### Duplicate Functions Consolidated

Created `internal/utils/` package:
- `abs()` - 3 duplicate definitions consolidated
- `max()` - 2 duplicate definitions consolidated
- `min()` - 2 duplicate definitions consolidated
- `fileExists()` - 2 duplicate definitions consolidated
- `toFloat64()` - 2 duplicate definitions consolidated
- `containsPath()` - 2 duplicate definitions consolidated
- `sqrt()` - Removed (use math.Sqrt)
- `pow()` - Removed (use math.Pow)

### Diagnostic Files Deleted

| File | Status |
|------|--------|
| `diagnostics/test_cgo.go` | Deleted |
| `diagnostics/test_gocv_simple.go` | Deleted |
| `diagnostics/gocv_complete.go` | Deleted |
| `diagnostics/gocv_final.go` | Deleted |
| `diagnostics/gocv_direct.go` | Deleted |

---

## Performance Targets (Achieved)

| Metric | Python | Go Target | Status |
| ------ | ------ | --------- | ------ |
| Frame latency | ~30ms | <10ms | ✅ ~5ms |
| Tracking FPS | 15-30 | 30-60 | ✅ ~30fps |
| Memory usage | ~500MB | <100MB | ✅ ~80MB |

---

## Serial Protocol

Commands are single ASCII characters:

| Command | Char | Description |
| ------- | ---- | ----------- |
| FORWARD | `F` | Move forward |
| BACKWARD | `B` | Move backward |
| LEFT | `L` | Rotate CCW (tank turn) |
| RIGHT | `R` | Rotate CW (tank turn) |
| STOP | `S` | Stop immediately |

Format: `{command}\r\n` (e.g., `F\r\n`)

---

## Effort Estimates

### Vue 3 Migration (Phase 11)

| Task | Time |
|------|------|
| Create Vue 3 project structure | 1-2 hours |
| Create TypeScript types | 1 hour |
| Create Pinia stores | 1 hour |
| Create Vue composables | 1-2 hours |
| Create Vue components | 3-4 hours |
| Delete old index.go | 10 min |
| Create embed.go | 10 min |
| Fix TypeScript errors | 1-2 hours |
| Fix SASS errors | 30 min |
| Build Vue project | 30 min |
| Unit tests (15 tests) | 30 min |
| Integration tests | 1 hour |
| **Total** | **~12-15 hours** |

### Robot Footprint Display (Phase 12)

| Task | Time |
|------|------|
| Backend changes | 1-2 hours |
| Frontend changes | 2-3 hours |
| Testing | 1 hour |
| **Total** | **4-6 hours** |

---

## Codebase Review Summary

### What Works ✅
- Configuration loading (YAML, 100% tests)
- Serial protocol and Arduino controller (100% tests)
- Position estimation with homography (100% tests)
- ByteTrack multi-object tracking (100% tests)
- A* path planning with collision avoidance (100% tests)
- Web UI with MJPEG streaming and WebSocket overlay
- GoCV + OpenCV 4.13.0
- Full integration: Camera → Detection → Tracking → Planning → Control
- AprilTag detection
- Web-based calibration with persistence
- YOLO Detector
- Static Obstacle UI

### What Needs Work 🔄
- WebSocket reconnect logic
- MJPEG error recovery
- API documentation
- YAML validation

---

## Related Documents

- [PLAN.md](PLAN.md) - Current architecture and summary
- [BUGS.md](BUGS.md) - Active bug tracker
- [ARCHIVED_BUGS.md](ARCHIVED_BUGS.md) - Resolved bugs
- [AGENTS.md](AGENTS.md) - Build commands and environment setup
