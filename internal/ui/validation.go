package ui

import "fmt"

// maxCalibrationTags bounds a calibration compute request. The target has a few
// dozen tags at most; anything far beyond that is malformed or hostile.
const maxCalibrationTags = 64

// isValidCommand reports whether cmd is one of the drive commands the UI sends.
func isValidCommand(cmd string) bool {
	switch cmd {
	case "F", "B", "L", "R", "S":
		return true
	}
	return false
}

// destinationProblem explains why a destination request is invalid, or returns
// "" if it is acceptable. frameW/frameH are the current video frame size (0 if
// not yet known, in which case the range check is skipped).
func destinationProblem(robotID, x, y, frameW, frameH int) string {
	if robotID < 0 {
		return fmt.Sprintf("invalid robot_id %d", robotID)
	}
	if x < 0 || y < 0 {
		return "destination is outside the video frame"
	}
	if frameW > 0 && frameH > 0 && (x >= frameW || y >= frameH) {
		return "destination is outside the video frame"
	}
	return ""
}

// pixelBoxProblem explains why an obstacle's pixel box is invalid, or returns
// "" if it is acceptable: it must have positive area and lie inside the frame
// (0 frame size means unknown, which skips the upper bound).
func pixelBoxProblem(topLeft, bottomRight [2]int, frameW, frameH int) string {
	if topLeft[0] < 0 || topLeft[1] < 0 {
		return "obstacle is outside the video frame"
	}
	if bottomRight[0] <= topLeft[0] || bottomRight[1] <= topLeft[1] {
		return "obstacle has no area"
	}
	if frameW > 0 && frameH > 0 && (bottomRight[0] > frameW || bottomRight[1] > frameH) {
		return "obstacle is outside the video frame"
	}
	return ""
}

// calibrationStateAfterCancel is the calibration status to show when the
// operator cancels the wizard. Cancelling must not make a working calibration
// look lost.
func calibrationStateAfterCancel(calibrated bool, calibrationFile string) (state, message, filename string, tagSize float64) {
	if calibrated {
		return "calibrated", "Calibration loaded", calibrationFile, 0.15
	}
	return "not_calibrated", "Click Settings to calibrate", "", 0
}
