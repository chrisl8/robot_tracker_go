//go:build gocv

package camera

// NewCamera opens the camera a CameraConfig describes: an IP stream when config.URL is set, otherwise the local device config.CameraID.
func NewCamera(config CameraConfig) (Camera, error) {
	if config.URL != "" {
		return NewGoCVIPCamera(config.URL)
	}
	return NewGoCVCamera(config)
}
