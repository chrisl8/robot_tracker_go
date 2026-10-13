//go:build !gocv

package camera

import (
	"fmt"
)

// NewCamera always fails in builds without the gocv tag, which have no camera support.
func NewCamera(config CameraConfig) (Camera, error) {
	return nil, fmt.Errorf("camera not available (gocv build tag required)")
}
