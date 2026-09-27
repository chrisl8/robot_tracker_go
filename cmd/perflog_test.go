package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPerfWindow_SummarisesAndResets(t *testing.T) {
	var p perfWindow
	start := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	p.start = start

	p.Record(80*time.Millisecond, 60*time.Millisecond, 20*time.Millisecond)
	p.Record(120*time.Millisecond, 70*time.Millisecond, 50*time.Millisecond)
	p.RecordCameraFailure()

	s := p.TakeSummary(start.Add(2 * time.Second))
	if s.Frames != 2 || s.FPS != 1.0 {
		t.Errorf("frames=%d fps=%.2f, want 2 frames at 1.0 fps", s.Frames, s.FPS)
	}
	if s.TotalAvg != 100*time.Millisecond || s.TotalMax != 120*time.Millisecond {
		t.Errorf("total avg/max = %v/%v, want 100ms/120ms", s.TotalAvg, s.TotalMax)
	}
	if s.DetectMax != 70*time.Millisecond || s.PlanMax != 50*time.Millisecond || s.CameraFails != 1 {
		t.Errorf("unexpected detect/plan/camfail: %+v", s)
	}
	if line := s.Line(); !strings.Contains(line, "fps=1.0") || !strings.Contains(line, "frame=100/120ms") {
		t.Errorf("unexpected line %q", line)
	}

	// The next window starts empty, and a silent window reports 0 fps.
	empty := p.TakeSummary(start.Add(4 * time.Second))
	if empty.Frames != 0 || empty.FPS != 0 || empty.TotalAvg != 0 {
		t.Errorf("window should have reset, got %+v", empty)
	}
}

func TestParseTopCPU(t *testing.T) {
	output := `  PID  %CPU COMM
  740  99.7 /usr/libexec/searchpartyd
26605 200.3 /Users/me/robot_tracker_go/robot_tracker
 1232  16.4 /Applications/Jellyfin Media Player.app/Contents/MacOS/Jellyfin Media Player
50280  17.1 /System/Library/VTDecoderXPCService
garbage line
`
	tests := []struct {
		name string
		self int
		n    int
		want []string
	}{
		{"excludes our own process and sorts by cpu", 26605, 3, []string{"searchpartyd(100%)", "VTDecoderXPCService(17%)", "Jellyfin Media Player(16%)"}},
		{"limits to n", 26605, 1, []string{"searchpartyd(100%)"}},
		{"keeps our process when it is not self", 1, 1, []string{"robot_tracker(200%)"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTopCPU(output, tt.self, tt.n); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
