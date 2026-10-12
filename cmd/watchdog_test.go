//go:build gocv

package main

import (
	"errors"
	"testing"
	"time"
)

func TestRestartIfCameraDead(t *testing.T) {
	tests := []struct {
		name       string
		hadFrames  bool
		age        time.Duration
		restartErr error
		want       bool
	}{
		{"fresh frames", true, time.Second, nil, false},
		{"stalled but within the reopen window", true, cameraDeadRestartAfter - time.Second, nil, false},
		{"dead after delivering frames", true, cameraDeadRestartAfter + time.Second, nil, true},
		{"never delivered a frame (permission): no restart loop", false, time.Hour, nil, false},
		{"restart command fails (not under launchd)", true, cameraDeadRestartAfter + time.Second, errors.New("no such service"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := newDemoAutonomyRig(t)
			if tt.hadFrames {
				rs.stats.lastFrameNanos.Store(time.Now().UnixNano())
			} else {
				rs.stats.lastFrameNanos.Store(0)
			}
			calls := 0
			old := restartService
			restartService = func() error { calls++; return tt.restartErr }
			defer func() { restartService = old }()

			got := rs.restartIfCameraDead(tt.age)

			if got != tt.want {
				t.Errorf("restartIfCameraDead(%v) = %v, want %v", tt.age, got, tt.want)
			}
			if (calls == 1) != tt.want {
				t.Errorf("restartService called %d times, want called=%v", calls, tt.want)
			}
		})
	}
}
