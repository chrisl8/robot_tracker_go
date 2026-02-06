//go:build gocv

package camera

func NewCamera(config CameraConfig) (Camera, error) {
	if config.URL != "" {
		return NewGoCVIPCamera(config.URL, config.Width, config.Height, config.FPS)
	}
	return NewGoCVCamera(config)
}
