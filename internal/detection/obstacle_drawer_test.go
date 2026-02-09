package detection

import (
	"testing"
)

func TestObstacleDrawer_DrawObstacles(t *testing.T) {
	drawer := NewObstacleDrawer()

	t.Run("empty input returns same image", func(t *testing.T) {
		imgData := make([]byte, 640*480*3)
		result := drawer.DrawObstacles(imgData, 640, 480, nil)
		if len(result) != len(imgData) {
			t.Errorf("expected same length, got %d", len(result))
		}
	})

	t.Run("empty obstacles returns same image", func(t *testing.T) {
		imgData := make([]byte, 640*480*3)
		result := drawer.DrawObstacles(imgData, 640, 480, []Obstacle{})
		if len(result) != len(imgData) {
			t.Errorf("expected same length, got %d", len(result))
		}
	})

	t.Run("draws obstacle rectangle", func(t *testing.T) {
		imgData := make([]byte, 640*480*3)
		obstacles := []Obstacle{
			{
				ID:               "test1",
				PixelTopLeft:     [2]int{100, 100},
				PixelBottomRight: [2]int{200, 200},
			},
		}
		result := drawer.DrawObstacles(imgData, 640, 480, obstacles)
		if len(result) != len(imgData) {
			t.Errorf("expected same length, got %d", len(result))
		}
	})

	t.Run("handles out of bounds obstacle", func(t *testing.T) {
		imgData := make([]byte, 640*480*3)
		obstacles := []Obstacle{
			{
				ID:               "out_of_bounds",
				PixelTopLeft:     [2]int{-100, -100},
				PixelBottomRight: [2]int{-50, -50},
			},
		}
		result := drawer.DrawObstacles(imgData, 640, 480, obstacles)
		if len(result) != len(imgData) {
			t.Errorf("expected same length, got %d", len(result))
		}
	})

	t.Run("multiple obstacles", func(t *testing.T) {
		imgData := make([]byte, 640*480*3)
		obstacles := []Obstacle{
			{
				ID:               "obs1",
				PixelTopLeft:     [2]int{50, 50},
				PixelBottomRight: [2]int{100, 100},
			},
			{
				ID:               "obs2",
				PixelTopLeft:     [2]int{200, 200},
				PixelBottomRight: [2]int{300, 300},
			},
		}
		result := drawer.DrawObstacles(imgData, 640, 480, obstacles)
		if len(result) != len(imgData) {
			t.Errorf("expected same length, got %d", len(result))
		}
	})

	t.Run("returns early on wrong image size", func(t *testing.T) {
		// This test catches the issue where camera returns unexpected frame sizes
		imgData := make([]byte, 1280*720*3+1000) // Extra bytes like real camera
		obstacles := []Obstacle{
			{
				ID:               "test",
				PixelTopLeft:     [2]int{100, 100},
				PixelBottomRight: [2]int{200, 200},
			},
		}
		// Should return early without crashing when size doesn't match
		result := drawer.DrawObstacles(imgData, 1280, 720, obstacles)
		if len(result) != len(imgData) {
			t.Errorf("expected original length when size mismatch, got %d", len(result))
		}
	})

	t.Run("handles 1280x720 camera format", func(t *testing.T) {
		// Test with realistic camera resolution
		width, height := 1280, 720
		imgData := make([]byte, width*height*3)
		obstacles := []Obstacle{
			{
				ID:               "camera_test",
				PixelTopLeft:     [2]int{100, 100},
				PixelBottomRight: [2]int{400, 400},
			},
		}
		result := drawer.DrawObstacles(imgData, width, height, obstacles)
		if len(result) != len(imgData) {
			t.Errorf("expected same length, got %d", len(result))
		}
	})
}
