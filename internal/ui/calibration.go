//go:build gocv

package ui

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

func (s *WebServer) SetCameraName(name string) {
	s.calibration.cameraName = name
}

func (s *WebServer) SetCalibrationState(state, message, filename string, tagSize float64) {
	s.calibration.mutex.Lock()
	s.calibration.state = state
	s.calibration.message = message
	s.calibration.filename = filename
	s.calibration.tagSize = tagSize
	s.calibration.mutex.Unlock()

	mismatch := false
	if pe := s.calibration.positionEstimator; pe != nil {
		mismatch = pe.ResolutionMismatch()
	}
	s.BroadcastOverlay(OverlayMessage{
		Type: "calibration",
		Calibration: &CalibrationStatusMessage{
			State:              state,
			Message:            message,
			Filename:           filename,
			TagSize:            tagSize,
			ResolutionMismatch: mismatch,
		},
	})
}

func (s *WebServer) SetPositionEstimator(pe *position.PositionEstimator) {
	s.calibration.positionEstimator = pe
}

func (s *WebServer) pixelCornersToWorld(pixelTL, pixelBR [2]int) ([2]float64, [2]float64) {
	if s.calibration.positionEstimator != nil && s.calibration.positionEstimator.IsCalibrated() {
		wTL := s.calibration.positionEstimator.PixelToWorld(pixelTL[0], pixelTL[1])
		wBR := s.calibration.positionEstimator.PixelToWorld(pixelBR[0], pixelBR[1])
		worldTL := [2]float64{wTL.X, wTL.Y}
		worldBR := [2]float64{wBR.X, wBR.Y}
		// Normalize so TopLeft has min coords and BottomRight has max coords
		if worldTL[0] > worldBR[0] {
			worldTL[0], worldBR[0] = worldBR[0], worldTL[0]
		}
		if worldTL[1] > worldBR[1] {
			worldTL[1], worldBR[1] = worldBR[1], worldTL[1]
		}
		return worldTL, worldBR
	}
	// Fallback when not calibrated: use pixel coords directly
	return [2]float64{float64(pixelTL[0]), float64(pixelTL[1])},
		[2]float64{float64(pixelBR[0]), float64(pixelBR[1])}
}

type CalibrationStatusMessage struct {
	State              string  `json:"state"`
	Message            string  `json:"message"`
	Filename           string  `json:"filename"`
	TagSize            float64 `json:"tagSize"`
	ResolutionMismatch bool    `json:"resolutionMismatch"`
}

func (s *WebServer) handleCalibrationStatus(c *gin.Context) {
	s.calibration.mutex.RLock()
	state := s.calibration.state
	message := s.calibration.message
	filename := s.calibration.filename
	tagSize := s.calibration.tagSize
	s.calibration.mutex.RUnlock()

	resp := gin.H{
		"state":              state,
		"message":            message,
		"filename":           filename,
		"tagSize":            tagSize,
		"resolutionMismatch": false,
	}
	if pe := s.calibration.positionEstimator; pe != nil {
		resp["resolutionMismatch"] = pe.ResolutionMismatch()
		if w, h, ok := pe.CalibratedResolution(); ok {
			resp["calibratedResolution"] = [2]int{w, h}
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (s *WebServer) handleCalibrationStart(c *gin.Context) {
	s.SetCalibrationState("detecting", "Looking for the calibration tags...", "", position.TargetTagSize)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "state": "detecting"})
}

type DetectedTagInfo struct {
	ID      int           `json:"id"`
	Center  [2]float64    `json:"center"`
	Corners [4][2]float64 `json:"corners"`
}

// CalibrationTargetSpec describes the printable calibration target so the UI
// does not have to hardcode tag IDs, labels or sizes.
type CalibrationTargetSpec struct {
	TagSize float64              `json:"tagSize"`
	Tags    []position.TargetTag `json:"tags"`
}

func calibrationTargetSpec() CalibrationTargetSpec {
	return CalibrationTargetSpec{
		TagSize: position.TargetTagSize,
		Tags:    position.TargetTags(),
	}
}

type CalibrationDetectedTagsResponse struct {
	Tags        []DetectedTagInfo     `json:"tags"`
	Count       int                   `json:"count"`
	FrameWidth  int                   `json:"frameWidth"`
	FrameHeight int                   `json:"frameHeight"`
	Target      CalibrationTargetSpec `json:"target"`
}

func (s *WebServer) UpdateDetectedTags(tags []DetectedTagInfo, frameWidth, frameHeight int) {
	s.calibration.detectedTagsMut.Lock()
	s.calibration.detectedTags = tags
	s.calibration.detectedFrameW = frameWidth
	s.calibration.detectedFrameH = frameHeight
	s.calibration.lastTagUpdate = time.Now()
	s.calibration.detectedTagsMut.Unlock()
}

// calibrationPollWindow is how recently the calibration wizard must have
// polled for tags for the stream to count as "in calibration view".
const calibrationPollWindow = 3 * time.Second

// CalibrationViewActive reports whether the calibration wizard is open and
// polling for tags. While it is, the video stream is sent without the
// detection overlay so the user sees only the clean camera view. Deriving
// this from polling means it clears itself if the browser goes away.
func (s *WebServer) CalibrationViewActive() bool {
	s.calibration.detectedTagsMut.RLock()
	defer s.calibration.detectedTagsMut.RUnlock()
	return !s.calibration.lastTagPoll.IsZero() && time.Since(s.calibration.lastTagPoll) < calibrationPollWindow
}

func (s *WebServer) handleCalibrationDetectedTags(c *gin.Context) {
	s.calibration.detectedTagsMut.Lock()
	s.calibration.lastTagPoll = time.Now()
	s.calibration.detectedTagsMut.Unlock()

	s.calibration.detectedTagsMut.RLock()
	tags := s.calibration.detectedTags
	if time.Since(s.calibration.lastTagUpdate) > 2*time.Second {
		tags = nil
	}
	width, height := s.calibration.detectedFrameW, s.calibration.detectedFrameH
	s.calibration.detectedTagsMut.RUnlock()

	if tags == nil {
		tags = []DetectedTagInfo{}
	}
	c.JSON(http.StatusOK, CalibrationDetectedTagsResponse{
		Tags:        tags,
		Count:       len(tags),
		FrameWidth:  width,
		FrameHeight: height,
		Target:      calibrationTargetSpec(),
	})
}

type CalibrationTagCapture struct {
	ID      int           `json:"id"`
	Corners [4][2]float64 `json:"corners"`
}

type CalibrationComputeRequest struct {
	Tags []CalibrationTagCapture `json:"tags"`
}

type CalibrationComputeResponse struct {
	State      string                   `json:"state"`
	RMSCm      float64                  `json:"rmsCm"`
	MaxCm      float64                  `json:"maxCm"`
	QualityCm  float64                  `json:"qualityCm"`
	Rating     string                   `json:"rating,omitempty"`
	WorstTagID int                      `json:"worstTagId,omitempty"`
	PerTag     []position.TagFit        `json:"perTag,omitempty"`
	Checks     []position.DistanceCheck `json:"checks,omitempty"`
	Message    string                   `json:"message,omitempty"`
	Error      string                   `json:"error,omitempty"`
	Filename   string                   `json:"filename,omitempty"`
}

func worstTagLabel(fit *position.FitResult) string {
	for _, t := range fit.PerTag {
		if t.ID == fit.WorstTagID {
			return t.Label
		}
	}
	return ""
}

func (s *WebServer) handleCalibrationCompute(c *gin.Context) {
	var req CalibrationComputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CalibrationComputeResponse{State: "error", Error: err.Error()})
		return
	}

	if len(req.Tags) > maxCalibrationTags {
		c.JSON(http.StatusBadRequest, CalibrationComputeResponse{State: "error", Error: fmt.Sprintf("too many tags (%d, max %d)", len(req.Tags), maxCalibrationTags)})
		return
	}

	captures := make([]position.TargetCapture, 0, len(req.Tags))
	for _, t := range req.Tags {
		capture := position.TargetCapture{ID: t.ID}
		for i, corner := range t.Corners {
			capture.Corners[i] = position.Point2D{X: corner[0], Y: corner[1]}
		}
		captures = append(captures, capture)
	}
	utils.Logf("Calibration compute: %d tags", len(captures))

	fit, err := position.FitTarget(captures)
	if err != nil {
		resp := CalibrationComputeResponse{State: "error", Error: err.Error()}
		if fit != nil {
			resp.RMSCm, resp.MaxCm, resp.QualityCm = fit.RMSCm, fit.MaxCm, fit.QualityCm
			resp.Rating, resp.WorstTagID, resp.PerTag, resp.Checks = fit.Rating, fit.WorstTagID, fit.PerTag, fit.Checks
			if errors.Is(err, position.ErrPoorFit) {
				resp.Error = fmt.Sprintf("The tags do not fit together as flat 15 cm squares (%.1f cm off). %s fits worst. Make sure every tag lies flat, is not curled, and was printed at exactly 15 cm, then try again. Nothing was saved.",
					fit.QualityCm, worstTagLabel(fit))
			}
		}
		utils.Logf("Calibration rejected: %v", err)
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	s.calibration.detectedTagsMut.RLock()
	frameW, frameH := s.calibration.detectedFrameW, s.calibration.detectedFrameH
	s.calibration.detectedTagsMut.RUnlock()
	if frameW <= 0 || frameH <= 0 {
		c.JSON(http.StatusBadRequest, CalibrationComputeResponse{State: "error", Error: "no camera frame has been seen yet"})
		return
	}

	utils.Logf("Homography fit: rms=%.2fcm max=%.2fcm quality=%.2fcm rating=%s H=%v",
		fit.RMSCm, fit.MaxCm, fit.QualityCm, fit.Rating, fit.Homography.H)

	cameraFile := GetCalibrationFilename(s.calibration.cameraName)
	cfg := position.NewCalibrationConfig(s.calibration.cameraName, frameW, frameH, fit, time.Now())
	if err := position.SaveCalibration(cameraFile, cfg); err != nil {
		utils.Logf("Failed to save calibration: %v", err)
		c.JSON(http.StatusInternalServerError, CalibrationComputeResponse{State: "error", Error: err.Error()})
		return
	}

	if s.Callbacks.OnCalibrationComplete != nil {
		s.Callbacks.OnCalibrationComplete(cameraFile)
	}

	message := fmt.Sprintf("Calibration saved: %.1f cm average error (%s).", fit.RMSCm, fit.Rating)
	if fit.Rating != position.RatingGood {
		message += fmt.Sprintf(" %s fits worst; make sure it lies flat and printed at 15 cm if accuracy matters.", worstTagLabel(fit))
	}
	s.SetCalibrationState("calibrated", message, cameraFile, position.TargetTagSize)

	c.JSON(http.StatusOK, CalibrationComputeResponse{
		State:      "calibrated",
		RMSCm:      fit.RMSCm,
		MaxCm:      fit.MaxCm,
		QualityCm:  fit.QualityCm,
		Rating:     fit.Rating,
		WorstTagID: fit.WorstTagID,
		PerTag:     fit.PerTag,
		Checks:     fit.Checks,
		Message:    message,
		Filename:   cameraFile,
	})
}

func (s *WebServer) handleCalibrationCancel(c *gin.Context) {
	pe := s.calibration.positionEstimator
	calibrated := pe != nil && pe.IsCalibrated()
	state, message, filename, tagSize := calibrationStateAfterCancel(calibrated, GetCalibrationFilename(s.calibration.cameraName))
	s.SetCalibrationState(state, message, filename, tagSize)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "state": "cancelled"})
}
