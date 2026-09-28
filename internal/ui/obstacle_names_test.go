package ui

import (
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/planning"
)

func named(names ...string) []planning.Obstacle {
	out := make([]planning.Obstacle, len(names))
	for i, n := range names {
		out[i] = planning.Obstacle{Name: n}
	}
	return out
}

func TestUniqueObstacleName(t *testing.T) {
	tests := []struct {
		name      string
		existing  []planning.Obstacle
		requested string
		want      string
	}{
		{"empty list, no name", nil, "", "obstacle_1"},
		{"free requested name kept", named("a"), "wall", "wall"},
		{"default follows count", named("x", "y"), "", "obstacle_3"},
		// Delete obstacle_1 of 3, then add: count+1 = 3 collides with obstacle_3.
		{"gap after delete", named("obstacle_2", "obstacle_3"), "obstacle_3", "obstacle_3_3"},
		{"gap after delete, no name", named("obstacle_2", "obstacle_3"), "", "obstacle_4"},
		{"taken name gets suffix", named("wall"), "wall", "wall_2"},
		{"suffix skips taken", named("wall", "wall_2"), "wall", "wall_3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uniqueObstacleName(tt.existing, tt.requested)
			if got != tt.want {
				t.Errorf("uniqueObstacleName(%v, %q) = %q, want %q", tt.existing, tt.requested, got, tt.want)
			}
			for _, o := range tt.existing {
				if o.Name == got {
					t.Errorf("returned name %q collides with an existing obstacle", got)
				}
			}
		})
	}
}
