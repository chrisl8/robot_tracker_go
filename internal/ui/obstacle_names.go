package ui

import (
	"fmt"

	"github.com/chrisl8/robot_tracker_go/internal/planning"
)

// uniqueObstacleName returns a name not used by any obstacle in existing.
// Obstacle names double as IDs (delete and update match on them), so a
// duplicate would make one action hit several obstacles. A requested name is
// kept if free; otherwise (or if empty) a numeric suffix is added.
func uniqueObstacleName(existing []planning.Obstacle, requested string) string {
	used := make(map[string]bool, len(existing))
	for _, o := range existing {
		used[o.Name] = true
	}

	if requested != "" && !used[requested] {
		return requested
	}

	base := requested
	if base == "" {
		base = "obstacle"
	}
	for n := len(existing) + 1; ; n++ {
		if candidate := fmt.Sprintf("%s_%d", base, n); !used[candidate] {
			return candidate
		}
	}
}
