//go:build gocv

package ui

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/chrisl8/robot_tracker_go/internal/planning"
)

// TestSaveObstaclesToFile_WritesParsableYAML guards saveObstaclesToFile's
// switch from hand-built YAML text to yaml.Marshal: it checks the resulting
// file still has the exact keys/nesting that
// PositionEstimator.LoadObstacles's generic map-based parser expects
// (docs/code-review-2026-09-27.md tech-debt: hand-rolled YAML string
// building).
func TestSaveObstaclesToFile_WritesParsableYAML(t *testing.T) {
	server := NewWebServer(":0")
	obstacles := []planning.Obstacle{
		planning.NewRectObstacle(`weird "name" with quotes`, [2]float64{1.25, -2.5}, [2]float64{3.75, 4.125}),
	}
	obstacles[0].PixelsTopLeft = [2]int{10, 20}
	obstacles[0].PixelsBottomRight = [2]int{300, 400}

	path := filepath.Join(t.TempDir(), "obstacles.yaml")
	if err := server.saveObstaclesToFile(path, obstacles); err != nil {
		t.Fatalf("saveObstaclesToFile failed: %v", err)
	}

	var parsed obstacleFile
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved obstacles file: %v", err)
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse saved obstacles file: %v", err)
	}

	if parsed.Version != 1 {
		t.Errorf("Version = %d, want 1", parsed.Version)
	}
	if len(parsed.Obstacles) != 1 {
		t.Fatalf("got %d obstacles, want 1", len(parsed.Obstacles))
	}
	entry := parsed.Obstacles[0]
	if entry.Name != `weird "name" with quotes` {
		t.Errorf("Name = %q, want %q", entry.Name, `weird "name" with quotes`)
	}
	if entry.Pixels.TopLeft != [2]int{10, 20} {
		t.Errorf("Pixels.TopLeft = %v, want [10 20]", entry.Pixels.TopLeft)
	}
	if entry.Pixels.BottomRight != [2]int{300, 400} {
		t.Errorf("Pixels.BottomRight = %v, want [300 400]", entry.Pixels.BottomRight)
	}
	if entry.World.TopLeft != [2]float64{1.25, -2.5} {
		t.Errorf("World.TopLeft = %v, want [1.25 -2.5]", entry.World.TopLeft)
	}
	if entry.World.BottomRight != [2]float64{3.75, 4.125} {
		t.Errorf("World.BottomRight = %v, want [3.75 4.125]", entry.World.BottomRight)
	}
}
