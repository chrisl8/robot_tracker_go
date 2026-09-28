//go:build gocv

package detection

import "testing"

// TestNewAprilTagDetector_FamilyDefault verifies that an explicitly configured
// Family is preserved, and only an unset Family falls back to the default
// "tag36h11". A prior regression inverted this condition, forcing every
// configured family to "tag36h11" and only leaving an actually-unset Family
// as "".
func TestNewAprilTagDetector_FamilyDefault(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		want       string
	}{
		{"unset family defaults to tag36h11", "", "tag36h11"},
		{"explicitly configured family is preserved", "tag16h5", "tag16h5"},
		{"explicit tag36h11 stays tag36h11", "tag36h11", "tag36h11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewAprilTagDetector(AprilTagConfig{Family: tt.configured})
			if err != nil {
				t.Fatalf("NewAprilTagDetector() error = %v", err)
			}
			if d.family != tt.want {
				t.Errorf("family = %q, want %q", d.family, tt.want)
			}
		})
	}
}
