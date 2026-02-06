# AGENTS.md - Robot Tracker Go Project

# General Agent Guidelines

- Always make a plan before doing anything.
- Document and save everything you plan to do in the `PLAN.md` file.
- If something causes you to deviate from the plan, stop and ask the user before proceeding. Do not modify the plan without consent.
- My intention is for the agent to write all of the code, but to do it in in small chunks and iterate on top of each part after testing

# Bug Tracking

## After making code changes:

- read `BUGS.md` and verify your changes don't introduce documented issues.

## When a bug is discovered and fixed

1. Add it to an appropriate category in `BUGS.md` with ID (e.g., `PATTERN-001`)
2. Describe the symptom, root cause, and fix
3. Update checklist if new pattern applies
4. Create unit tests to ensure that the bug does not happen again
5. Create integration tests to ensure that the bug does not happen again

# Code generation guidelines

- Always update `PLAN.md` with your plan before writing code
- After code is finished you must create unit and integration tests to cover the code before presenting it as complete
- Always update `PLAN.md` after writing code

# Go advice

- To see source files from a dependency, or to answer questions
  about a dependency, run `go mod download -json MODULE` and use
  the returned `Dir` path to read the files.

- Use `go doc foo.Bar` or `go doc -all foo` to read documentation
  for packages, types, functions, etc.

- Use `go run .` or `go run ./cmd/foo` instead of `go build` to
  run programs, to avoid leaving behind build artifacts.

# Go Code Exploration Preferences

When exploring or analyzing Go code, **always prefer using gopls MCP server (local-gopls-mcp) tools** over local grep, find, or codebase_search tools.

## Tool Priority for Go Code

1. **First choice**: Use gopls MCP tools:
   - `go_search` - for searching Go symbols
   - `go_package_api` - for exploring package APIs
   - `go_file_context` - for understanding file dependencies
   - `go_symbol_references` - for finding symbol references
   - `go_workspace` - for workspace information
   - `go_diagnostics` - for checking errors

2. **Fallback only**: Use local tools (grep, codebase_search) only if:
   - The gopls MCP server is unavailable
   - The query is not Go-specific (e.g., searching for configuration files)
   - The gopls tools don't provide the needed information

## Examples

- ✅ "Find all usages of function X" → Use `go_symbol_references`
- ✅ "What does package Y export?" → Use `go_package_api`
- ✅ "Search for type Z" → Use `gopls_go_search`
- ❌ Don't use `grep` or `codebase_search` for Go symbol searches

## Rationale

gopls MCP tools provide:

- Semantic understanding of Go code
- Accurate symbol resolution
- Type-aware search
- Better context about package structure
- More accurate than text-based search

# Commands

## Build Commands

```bash
# Install dependencies
go mod tidy

# Build the application
go build -o robot_tracker.exe ./cmd/main.go

# Build with race detector
go build -race -o robot_tracker_race.exe ./cmd/main.go

# Run the application
./robot_tracker.exe

# Run with specific serial port
./robot_tracker.exe --port COM3

# List available serial ports
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

# Run a single test function
go test -v -run TestSerialProtocol_EncodeCommand ./internal/controller/

# Run tests matching pattern
go test -v -run "TestSerialProtocol" ./internal/controller/

# Run all tests in internal packages
go test -v ./internal/...
```

# Code Style Guidelines

## General Principles

- Write idiomatic Go code following effective go conventions
- Keep functions short and focused on single responsibility
- Use interfaces for abstraction (e.g., `Camera`, `Tracker` interfaces)
- Prefer composition over inheritance

## Naming Conventions

- **Packages**: lowercase, concise, usually single word
- **Types**: PascalCase (e.g., `Track`, `Detection`, `CameraConfig`)
- **Variables**: camelCase (e.g., `trackID`, `bbox`, `confidence`)
- **Constants**: PascalCase for grouped constants, camelCase for values (e.g., `BaudRate`, `DefaultWidth`)
- **Interfaces**: named for what they do (e.g., `Camera`, `Tracker`), not prefixed with `I`
- **Error variables**: `ErrXxx` format (e.g., `ErrInvalidData`, `ErrNotConnected`)

## Import Organization

Group imports in this order with blank line between groups:

1. Standard library imports
2. Third-party imports
3. Internal/local imports (using module path prefix)

```go
import (
    "fmt"
    "os"
    "time"

    "github.com/gin-gonic/gin"
    "gopkg.in/yaml.v3"

    "robot_tracker_go/internal/config"
    "robot_tracker_go/internal/controller"
)
```

## Error Handling

- Use `fmt.Errorf("message: %w", err)` for wrapped errors
- Define package-level error variables for sentinel/expected errors
- Handle errors explicitly; avoid discarding with `_`
- Return errors early rather than nesting

```go
var ErrInvalidData = fmt.Errorf("invalid serial data")

func Decode(data []byte) (Command, error) {
    if len(data) < 1 {
        return 0, ErrInvalidData
    }
    return Command(data[0]), nil
}
```

## Struct Definitions

- Use yaml tags for configuration structs
- Use JSON-like map returns for API/serialization (`ToDict()` pattern)
- Initialize structs inline where natural

```go
type CameraConfig struct {
    Name     string  `yaml:"name"`
    CameraID int     `yaml:"camera_id"`
    URL      string  `yaml:"url"`
}

type Track struct {
    TrackID    int
    Bbox       [4]int
    Confidence float64
    State      TrackState
    History    []TrackHistoryPoint
}
```

## Testing Patterns

Use table-driven tests for consistent test coverage:

```go
func TestSerialProtocol_EncodeCommand(t *testing.T) {
    tests := []struct {
        name     string
        cmd      Command
        expected []byte
    }{
        {"Forward", CommandForward, []byte{'F', '\r', '\n'}},
        {"Backward", CommandBackward, []byte{'B', '\r', '\n'}},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := p.EncodeCommand(tt.cmd)
            if result[0] != byte(tt.cmd) {
                t.Errorf("EncodeCommand()[0] = %c, want %c", result[0], tt.cmd)
            }
        })
    }
}
```

## Project Structure

```
robot_tracker_go/
├── cmd/main.go              # Application entry point
├── internal/
│   ├── camera/              # Camera abstraction and implementations
│   ├── config/              # Configuration loading (YAML)
│   ├── controller/          # Arduino serial communication
│   ├── detection/           # AprilTag + YOLO detection pipeline
│   ├── planning/            # Path planning algorithms
│   ├── position/            # Position estimation (homography)
│   ├── tracking/            # Multi-object tracking (ByteTrack, Kalman)
│   └── ui/                  # Web UI and display
├── assets/                  # YOLO ONNX models
├── config/                  # Configuration YAML files
└── scripts/                 # Utility scripts
```

## Configuration

- Use YAML for configuration files
- Store in `config/` directory
- Load with `internal/config/config.go` `Load()` function
- Configuration structs use yaml struct tags

## Module Dependencies

- Module name: `robot_tracker_go`
- Go version: 1.23.2+
- Key dependencies:
  - `gopkg.in/yaml.v3` - YAML parsing
  - `github.com/gin-gonic/gin` - Web framework
  - `gocv.io/x/gocv` - Computer vision (OpenCV bindings)
  - `go.bug.st/serial` - Serial port communication
  - `github.com/gorilla/websocket` - WebSocket support

## Command-Line Flags

Use Go's `flag` package for CLI arguments:

- `--config`: Path to configuration file
- `--port`: Serial port (use "auto" for auto-detection)
- `--list-ports`: List available serial ports
- `--web-port`: Web server port
- `--demo`: Run demo mode with test pattern

## Common Tasks

**Add a new detection type:**

1. Define struct in `internal/detection/types.go`
2. Add to detection pipeline in `internal/detection/pipeline.go`
3. Add tests for new detection type

**Add a new camera type:**

1. Implement `Camera` interface from `internal/camera/camera.go`
2. Register in camera factory if applicable
3. Add configuration options in config types

**Add new robot command:**

1. Add `Command` constant in `internal/controller/protocol.go`
2. Update `SerialProtocol` encoding methods
3. Add test cases in `controller_test.go`
4. Update README command table

## Notes

- This is a multi-robot tracking system using AprilTags and YOLO
- Integrates with Arduino microcontrollers via serial communication
- Web UI provides real-time video streaming and robot control
- Path planning uses A\* algorithm with collision avoidance
