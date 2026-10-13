package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/position"
)

func loadYAML(t *testing.T, content string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func TestLoad_RejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string // substring of the error
	}{
		{"unknown follower", "path_execution:\n  follower: carrt\n", `path_execution.follower is "carrt"`},
		{"negative diameter", "robots:\n  - tag_id: 1\n    name: a\n    diameter: -0.3\n", "robots[0] (\"a\"): diameter is -0.3"},
		{"negative tag id", "robots:\n  - tag_id: -1\n", "tag_id is -1"},
		{"reserved tag id", fmt.Sprintf("robots:\n  - tag_id: %d\n    name: a\n", position.TargetIDMin), "reserved for the calibration target"},
		{"duplicate tag id", "robots:\n  - tag_id: 1\n    name: a\n  - tag_id: 1\n    name: b\n", "tag_id 1 is already used by robots[0]"},
		{"duplicate name", "robots:\n  - tag_id: 1\n    name: a\n  - tag_id: 2\n    name: a\n", "name is already used by robots[0]"},
		{"unknown camera type", "cameras:\n  - type: usbb\n", `type is "usbb"`},
		{"ip camera without url", "cameras:\n  - type: ip\n", "needs a url"},
		{"negative camera width", "cameras:\n  - camera_id: 0\n    width: -1\n", "width is -1"},
		{"unknown tag family", "april_tags:\n  family: tag99h9\n", `april_tags.family is "tag99h9"`},
		{"track threshold above one", "tracking:\n  track_thresh: 5\n", "tracking.track_thresh is 5"},
		{"negative track buffer", "tracking:\n  track_buffer: -3\n", "tracking.track_buffer"},
		{"smoothing alpha above one", "position:\n  heading_smoothing_alpha: 2\n", "heading_smoothing_alpha is 2"},
		{"negative burst frames", "path_execution:\n  burst_frames: -1\n", "burst_frames"},
		{"negative baud rate", "controller:\n  serial:\n    baudrate: -9600\n", "baudrate"},
		{"foreground scale above one", "foreground:\n  scale: 3\n", "foreground.scale is 3"},
		{"foreground threshold above 255", "foreground:\n  threshold: 300\n", "foreground.threshold is 300"},
		{"negative foreground size", "foreground:\n  min_size_m: -1\n", "foreground.min_size_m"},
		{"shadow alpha range inverted", "foreground:\n  shadow_alpha_min: 0.9\n  shadow_alpha_max: 0.5\n", "shadow_alpha_min (0.9) is above shadow_alpha_max (0.5)"},
		{"explicit zero shadow gate is fine but 2 is not", "foreground:\n  shadow_chroma_max: 2\n", "shadow_chroma_max is 2"},
		{"wrong YAML type", "tracking:\n  track_buffer: lots\n", "failed to parse config file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadYAML(t, tt.yaml)
			if err == nil {
				t.Fatalf("Load accepted %q, want an error containing %q", tt.yaml, tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestLoad_AcceptsUnsetAndBoundaryValues(t *testing.T) {
	tests := []struct{ name, yaml string }{
		{"empty file", ""},
		{"only comments", "# nothing\n"},
		{"zero means default", "robots:\n  - tag_id: 1\n    diameter: 0\npath_execution:\n  burst_frames: 0\nforeground:\n  shadow_alpha_min: 0\n"},
		{"upper boundaries", "tracking:\n  track_thresh: 1\nposition:\n  heading_smoothing_alpha: 1\nforeground:\n  scale: 1\n  threshold: 255\n"},
		{"known follower and family", "path_execution:\n  follower: waypoint\napril_tags:\n  family: tag16h5\n"},
		{"ip camera with url", "cameras:\n  - type: ip\n    url: http://x/video\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := loadYAML(t, tt.yaml); err != nil {
				t.Errorf("Load rejected a valid config: %v", err)
			}
		})
	}
}

// One restart should show every problem, not just the first.
func TestLoad_ReportsAllProblemsAtOnce(t *testing.T) {
	_, err := loadYAML(t, "path_execution:\n  follower: nope\ntracking:\n  track_thresh: 9\nrobots:\n  - tag_id: -2\n")
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"path_execution.follower", "tracking.track_thresh", "tag_id is -2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err, want)
		}
	}
}

// A misspelled or removed key must not take the service offline, but must be
// visible: it is returned as a warning with its line number.
func TestLoad_WarnsAboutUnknownKeys(t *testing.T) {
	cfg, err := loadYAML(t, "path_execution:\n  follwer: waypoint\n  burst_frames: 3\nmystery: 1\n")
	if err != nil {
		t.Fatalf("unknown keys must not be an error: %v", err)
	}
	if cfg.PathExecution.BurstFrames != 3 {
		t.Errorf("known keys must still load, burst_frames = %d", cfg.PathExecution.BurstFrames)
	}
	got := strings.Join(cfg.Warnings, "\n")
	for _, want := range []string{`line 2`, `"follwer"`, `line 4`, `"mystery"`} {
		if !strings.Contains(got, want) {
			t.Errorf("warnings %q are missing %q", got, want)
		}
	}
	if len(cfg.Warnings) != 2 {
		t.Errorf("got %d warnings, want 2: %v", len(cfg.Warnings), cfg.Warnings)
	}
}
