package detection

import (
	"sync"
	"testing"
)

// SetObstacles is called from HTTP handler goroutines while DrawResults runs
// on the frame loop. Run with -race: an unguarded slice header write/read here
// is a data race (and a torn header can crash).
func TestDetectionPipeline_SetObstaclesConcurrentWithDrawResults(t *testing.T) {
	p := NewDetectionPipeline(AprilTagConfig{Family: "tag36h11"})
	img := make([]byte, 64*48*3)
	result := &DetectionResult{}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			p.SetObstacles([]Obstacle{{ID: "a", PixelTopLeft: [2]int{1, 1}, PixelBottomRight: [2]int{10, 10}}})
			p.SetObstacles(nil)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			p.DrawResults(img, 64, 48, result)
		}
	}()
	wg.Wait()
}
