# Robot Tracker Go - Implementation Summary

## What Was Done

### Core Systems

| System | Description | Status |
|--------|-------------|--------|
| Configuration | YAML loading with validation helpers | Complete |
| Serial Protocol | Arduino command encoding (F/B/L/R/S) | Complete |
| Position Estimation | Homography, pixel↔world transforms | Complete |
| Multi-Object Tracking | ByteTrack, Kalman filter, Hungarian algorithm | Complete |
| Path Planning | A*, Coordinator (state store) | Complete |
| Dynamic Obstacles | Background-subtraction detection, collision avoidance | Complete |
| Static Obstacles | UI drawing + YAML persistence | Complete |

### User Interface

| Component | Description |
|-----------|-------------|
| Vue 3 Frontend | TypeScript + Pinia + Composition API |
| WebSocket Overlay | Real-time tracks, obstacles, paths |
| MJPEG Streaming | Video feed to browser |
| Calibration Wizard | Auto-detect tags, save to YAML |
| Robot Footprints | Cyan circles showing robot size |

### Build System

- GoCV + OpenCV 4.13.0 support
- Cross-platform (Windows/Linux)
- Wrapper scripts for environment setup
- Demo mode (no camera required)

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         This Computer (Host)                            │
│                                                                         │
│  ┌──────────────┐    ┌──────────────┐    ┌────────────────────────┐    │
│  │   Camera     │───>│   Detector   │───>│     Tracker           │    │
│  │   (gocv)     │    │  (AprilTag)  │    │   (ByteTrack)         │    │
│  └──────────────┘    └──────────────┘    └────────────┬───────────┘    │
│                                                         │                │
│  ┌──────────────────────────────────────────────────────┼───────────┐    │
│  │                                                      │           │    │
│  │  ┌────────────────┐    ┌────────────────────┐       │           │    │
│  │  │ Position       │───>│ Planner            │───>┌──┴──────┐    │    │
│  │  │ Estimator      │    │ (A* + Local)      │    │ Arduino │    │    │
│  │  └────────────────┘    └────────────────────┘    └─────────┘    │    │
│  │                                                       │              │
│  │  ┌───────────────────────────────────────────────────┼──────────┐   │
│  │  │                     UI Layer                     │          │   │
│  │  │  ┌─────────────┐   ┌────────────────┐  ┌───────▼────────┐  │   │
│  │  │  │ MJPEG       │   │ WebSocket      │  │ Gin Web Server │  │   │
│  │  │  │ Streaming   │   │ Overlay         │  │ (REST API)     │  │   │
│  │  │  └─────────────┘   └────────────────┘  └────────────────┘  │   │
│  │  │                       Vue 3 + TypeScript + Pinia             │   │
│  │  └──────────────────────────────────────────────────────────────┘   │
│  └──────────────────────────────────────────────────────────────────────┘
```

### Component Flow

```
[Camera Frame]
       │
       ▼
[AprilTag Detector] ──> [Detection Pipeline]
                                    │
                                    ▼
                           [ByteTrack Tracker]
                                    │
                                    ▼
                        [Position Estimator] ────────┐
                                    │                  │
                                    ▼                  ▼
                             [A* Path Planning]   [WebSocket UI]
                                    │                  │
                                    ▼                  ▼
                        [Bearing steering] ──> [Arduino Serial]
```

### Key Packages

| Package | Purpose | Files |
|---------|---------|-------|
| `internal/config` | YAML configuration | `config.go` |
| `internal/camera` | Camera abstraction | `camera.go`, `gocv_camera.go`, `video.go`, `ip.go`, `usb.go` |
| `internal/position` | Homography, transforms | `types.go`, `homography.go`, `estimator.go` |
| `internal/detection` | AprilTag + background-subtraction obstacles | `types.go`, `apriltag.go`, `foreground.go`, `pipeline.go` |
| `internal/tracking` | Multi-object tracking | `types.go`, `kalman.go`, `hungarian.go`, `bytetrack.go` |
| `internal/planning` | Path planning | `types.go`, `astar.go`, `coordinator.go`, `collision.go`, `planner.go` |
| `internal/controller` | Serial control | `protocol.go`, `arduino.go`, `queue.go`, `executor.go` |
| `internal/ui` | Web server | `webserver.go`, `embed.go`, `types.go` |

### Serial Protocol

| Command | Char | Description |
| ------- | ---- | ----------- |
| FORWARD | `F` | Move forward |
| BACKWARD | `B` | Move backward |
| LEFT | `L` | Rotate CCW (tank turn) |
| RIGHT | `R` | Rotate CW (tank turn) |
| STOP | `S` | Stop immediately |

Format: `{command}\r\n` (e.g., `F\r\n`)

### REST API Endpoints

| Method | Path | Purpose |
| ------ | ---- | ------- |
| POST | `/api/obstacles` | Add obstacle |
| DELETE | `/api/obstacles/:id` | Delete obstacle |
| POST | `/api/obstacles/save` | Save to YAML |
| POST | `/api/command` | Send robot command |
| POST | `/api/destination` | Set navigation target |

### WebSocket Message Types

| Type | Purpose |
| ---- | ------- |
| `track` | Single track update |
| `tracks` | Batch track update |
| `obstacles` | Obstacle list |
| `status` | System status |
| `calibration` | Calibration state |

### Go ↔ Vue Type Mapping

| Go Type | Vue Type | File |
| ------- | -------- | ---- |
| `Track` | `Track` | `ui/src/types/api.ts` |
| `Obstacle` | `Obstacle` | `ui/src/types/api.ts` |

---

## What Remains

### Open Bugs

See [BUGS.md](BUGS.md) for active issues:

| ID | Component | Description |
|----|-----------|-------------|
| UI-001 | WebSocket | Reconnect logic needed |
| UI-002 | MJPEG | Error recovery needed |
| DOC-001 | All | API documentation incomplete |
| CONFIG-001 | Config | YAML validation missing |

---

## Project Structure

```
robot_tracker_go/
├── cmd/main.go                    # Main application entry
├── go.mod                         # Go modules
├── internal/
│   ├── config/                    # YAML configuration
│   ├── camera/                    # Camera sources
│   ├── position/                  # Homography & transforms
│   ├── detection/                 # AprilTag + background-subtraction obstacles
│   ├── tracking/                 # ByteTrack
│   ├── planning/                 # A* + waypoints
│   ├── controller/               # Arduino serial
│   └── ui/                       # Web server + Vue UI
├── ui/                           # Vue 3 frontend source
│   ├── src/
│   │   ├── components/           # Vue components
│   │   ├── composables/          # Vue composables
│   │   ├── stores/              # Pinia stores
│   │   └── types/                # TypeScript types
│   └── package.json
├── scripts/                       # Build scripts
├── config/                        # Configuration files
├── assets/                       # AprilTag print sheets
└── docs/
    ├── PLAN.md                   # This file
    ├── AGENTS.md                 # Build commands
    ├── BUGS.md                   # Active bugs
    └── archived/
        ├── ARCHIVED_BUGS.md      # Resolved bug history
        ├── ARCHIVED_PLAN.md      # Historical implementation notes
        └── PHASE_11_MIGRATION/   # Vue 3 migration documentation
            ├── PHASE_11_PLAN.md
            └── PHASE_11_REFERENCE.md
```

---

## Related Documents

| Document | Purpose |
|----------|---------|
| [AGENTS.md](AGENTS.md) | Build commands and environment setup |
| [BUGS.md](BUGS.md) | Active bug tracker |
| [docs/archived/ARCHIVED_BUGS.md](docs/archived/ARCHIVED_BUGS.md) | Resolved bug history |
| [docs/archived/ARCHIVED_PLAN.md](docs/archived/ARCHIVED_PLAN.md) | Historical implementation notes |

---

## Quick Start

```bash
# Build Vue UI and Go backend
./scripts/build.sh

# Run demo mode (no camera)
./scripts/run.sh --demo

# Run with camera
./scripts/run.sh

# Run tests
./scripts/test.sh --verbose
```
