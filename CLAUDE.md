# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

**Always use wrapper scripts** — they automatically set up required OpenCV environment variables (`OPENCV_DIR`, `LD_LIBRARY_PATH`, `PKG_CONFIG_PATH`). Do NOT run `go build` directly.

```bash
./scripts/build.sh           # Build Vue 3 UI + Go backend
./scripts/run.sh --demo      # Run demo mode (no camera required)
./scripts/run.sh             # Run with camera
./scripts/test.sh --verbose  # Run all tests (Go + Vue)
```

## Testing

```bash
# Run all Go tests
./scripts/test.sh --verbose

# Run a specific Go package
go test -v ./internal/planning/

# Run a single Go test function
go test -v -run TestFunctionName ./internal/package/

# Vue tests
cd ui && npm run test:run      # CI mode
cd ui && npm run test:coverage # With coverage
```

## Architecture

This is a multi-robot tracking and control system. Data flows through a pipeline:

**Camera → Detection → Tracking → Planning → Arduino Controller**

The `RobotSystem` struct in `cmd/main.go` owns and orchestrates all subsystems via `Initialize()` and `ProcessFrame()`.

### Key Packages

- **`internal/detection/`** — AprilTag (robot ID) + YOLOv8 (obstacle) detection; results fused by bounding box overlap
- **`internal/tracking/`** — ByteTrack multi-object tracker with Kalman filter; outputs `Track` objects with world positions
- **`internal/position/`** — Homography calibration maps pixel↔world coordinates; saved to `config/calibration_<camera>.yaml`
- **`internal/planning/`** — Three-layer: A\* global path, velocity obstacle local planner, multi-robot `Coordinator`
- **`internal/controller/`** — Arduino serial at 115200 baud; single ASCII commands (F/B/L/R/S + `\r\n`); `CommandQueue` + `PathExecutor`
- **`internal/ui/`** — Gin HTTP server; MJPEG stream at `/stream`; WebSocket at `/ws` for real-time overlay; REST API for calibration/obstacles/goals
- **`ui/src/`** — Vue 3 + TypeScript frontend; Pinia stores (`robotStore`, `obstacleStore`, `uiStore`); canvas overlay renders tracks/paths

### Configuration

`config/tracking_config.yaml` — cameras, robot definitions (tag IDs, sizes), detection thresholds, planning parameters, serial settings.

## Go Code Style

- Import groups: stdlib → third-party → internal, separated by blank lines
- Errors: `fmt.Errorf("context: %w", err)`; return early; define `ErrXxx` sentinel errors
- Use table-driven tests
