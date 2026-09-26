package camera

import "testing"

func TestDisplayName(t *testing.T) {
	tests := []struct {
		name string
		url  string
		id   int
		want string
	}{
		{"local camera", "", 0, "Camera 0"},
		{"second local camera", "", 2, "Camera 2"},
		{"stream url", "http://192.168.1.5:4747/video", 0, "Video: http://192.168.1.5:4747/video"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DisplayName(tt.url, tt.id); got != tt.want {
				t.Errorf("DisplayName(%q, %d) = %q, want %q", tt.url, tt.id, got, tt.want)
			}
		})
	}
}
