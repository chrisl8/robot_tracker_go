//go:build gocv

package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

func GetCalibrationFilename(cameraName string) string {
	sanitized := sanitizeCameraName(cameraName)
	return fmt.Sprintf("config/calibration_%s.yaml", sanitized)
}

// GetForegroundStateFilename returns where the foreground detector's learned
// background is saved for cameraName, so a restart can restore it instead of
// re-learning.
func GetForegroundStateFilename(cameraName string) string {
	sanitized := sanitizeCameraName(cameraName)
	return fmt.Sprintf("config/foreground_bg_%s.bin", sanitized)
}

// GetObstaclesFilename returns the default per-camera obstacles file path
// (config/obstacles_<sanitized-camera-name>.yaml), or the camera-agnostic
// config/obstacles.yaml when cameraName is empty. Mirrors GetCalibrationFilename.
func GetObstaclesFilename(cameraName string) string {
	if cameraName == "" {
		return "config/obstacles.yaml"
	}
	return fmt.Sprintf("config/obstacles_%s.yaml", sanitizeCameraName(cameraName))
}

// ResolveObstaclesPath is the single source of truth for where the obstacles
// file lives, used identically by startup loading (RobotSystem.
// loadStaticObstacles), the position estimator's own obstacle copy
// (RobotSystem.initPositionEstimator), and the web UI's save/clear handlers
// (via GetObstaclesPath), so they can no longer drift out of sync (see
// docs/archived/code-review-2026-09-27.md).
//
// An explicitly configured path (ObstaclesConfig.File/Path, via GetPath())
// is honored only if it points to a file that actually exists -- this lets
// the checked-in tracking_config.yaml default (config/obstacles.yaml, which
// nothing has ever saved to) fall through to the real per-camera file
// instead of silently never loading/saving anything. Otherwise, the
// per-camera default from GetObstaclesFilename is used, so a fresh setup's
// first save lands exactly where later loads will look for it.
func ResolveObstaclesPath(configuredPath, cameraName string) string {
	if configuredPath != "" && utils.FileExists(configuredPath) {
		return configuredPath
	}
	return GetObstaclesFilename(cameraName)
}

func sanitizeCameraName(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := reg.ReplaceAllString(name, "_")
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		sanitized = "unknown"
	}
	return sanitized
}
