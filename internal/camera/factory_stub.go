//go:build !gocv

package camera

import (
	"fmt"
)

func NewCamera(config CameraConfig) (Camera, error) {
	return nil, fmt.Errorf("camera not available (gocv build tag required)")
}
