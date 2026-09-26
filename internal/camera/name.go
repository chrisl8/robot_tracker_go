package camera

import "fmt"

// DisplayName is the name a GoCV camera reports for a stream URL or device
// ID. It is derived from configuration alone so callers (such as the
// calibration file lookup) get the same name whether or not the camera has
// finished opening yet; a camera still waiting on OS permission must not
// change which calibration file is used.
func DisplayName(url string, cameraID int) string {
	if url != "" {
		return fmt.Sprintf("Video: %s", url)
	}
	return fmt.Sprintf("Camera %d", cameraID)
}
