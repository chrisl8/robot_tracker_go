//go:build gocv

package camera

func NewCamera(config CameraConfig) (Camera, error) {
	if config.URL != "" {
		return NewIPCamera(config.URL, config.Width, config.Height, config.FPS), nil
	}
	return NewGoCVCamera(config)
}
