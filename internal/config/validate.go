package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/chrisl8/robot_tracker_go/internal/position"
)

// knownAprilTagFamilies are the families the detector understands; any other
// name silently becomes tag36h11 inside the detector, so reject it here.
var knownAprilTagFamilies = map[string]bool{
	"tag16h5": true, "tag25h9": true, "tag36h10": true, "tag36h11": true,
}

// Validate reports every invalid value in the configuration at once, so one
// edit-and-restart cycle fixes them all. An unset (zero) value is never an
// error: it means "use the default". Only values that are wrong whatever the
// default is (negative, out of range, unknown names, duplicates) are rejected.
// Unknown YAML keys are not errors; Load reports them in Warnings.
func (c *Config) Validate() error {
	v := &validator{}
	c.validateRobots(v)
	c.validateCameras(v)
	c.validateDetection(v)
	c.validateMotion(v)
	c.validateForeground(v)
	return v.err()
}

type validator struct{ problems []error }

func (v *validator) add(format string, args ...any) {
	v.problems = append(v.problems, fmt.Errorf(format, args...))
}

func (v *validator) err() error {
	if len(v.problems) == 0 {
		return nil
	}
	return fmt.Errorf("invalid config: %w", errors.Join(v.problems...))
}

// nonNegative flags a negative value; zero stays legal (unset).
func (v *validator) nonNegative(field string, value float64) {
	if value < 0 {
		v.add("%s is %v, must not be negative", field, value)
	}
}

// fraction flags a value outside 0..1.
func (v *validator) fraction(field string, value float64) {
	if value < 0 || value > 1 {
		v.add("%s is %v, must be between 0 and 1", field, value)
	}
}

func (c *Config) validateRobots(v *validator) {
	tags := map[int]int{}
	names := map[string]int{}
	for i, r := range c.Robots {
		where := fmt.Sprintf("robots[%d]", i)
		if r.Name != "" {
			where = fmt.Sprintf("robots[%d] (%q)", i, r.Name)
		}
		if r.TagID < 0 {
			v.add("%s: tag_id is %d, must not be negative", where, r.TagID)
		}
		if position.IsTargetTagID(r.TagID) {
			v.add("%s: tag_id %d is reserved for the calibration target (%d-%d)",
				where, r.TagID, position.TargetIDMin, position.TargetIDMax)
		}
		if j, dup := tags[r.TagID]; dup {
			v.add("%s: tag_id %d is already used by robots[%d]", where, r.TagID, j)
		}
		tags[r.TagID] = i
		if r.Name != "" {
			if j, dup := names[r.Name]; dup {
				v.add("%s: name is already used by robots[%d]", where, j)
			}
			names[r.Name] = i
		}
		v.nonNegative(where+": diameter", r.Diameter)
	}
}

func (c *Config) validateCameras(v *validator) {
	for i, cam := range c.Cameras {
		where := fmt.Sprintf("cameras[%d]", i)
		switch cam.Type {
		case "", "usb", "ip":
		default:
			v.add("%s: type is %q, must be \"usb\" or \"ip\"", where, cam.Type)
		}
		if cam.Type == "ip" && cam.URL == "" {
			v.add("%s: an ip camera needs a url", where)
		}
		if cam.CameraID < 0 {
			v.add("%s: camera_id is %d, must not be negative", where, cam.CameraID)
		}
		v.nonNegative(where+": width", float64(cam.Width))
		v.nonNegative(where+": height", float64(cam.Height))
		v.nonNegative(where+": fps", float64(cam.FPS))
	}
}

func (c *Config) validateDetection(v *validator) {
	if f := c.AprilTags.Family; f != "" && !knownAprilTagFamilies[f] {
		v.add("april_tags.family is %q, must be one of tag16h5, tag25h9, tag36h10, tag36h11", f)
	}
	v.nonNegative("april_tags.quad_decimate", c.AprilTags.QuadDecimate)

	t := c.Tracking
	v.fraction("tracking.track_thresh", t.TrackThresh)
	v.fraction("tracking.match_thresh", t.MatchThresh)
	v.nonNegative("tracking.track_buffer", float64(t.TrackBuffer))
	v.nonNegative("tracking.min_box_area", float64(t.MinBoxArea))
}

func (c *Config) validateMotion(v *validator) {
	p := c.PathExecution
	switch p.Follower {
	case "", "carrot", "waypoint":
	default:
		v.add("path_execution.follower is %q, must be \"carrot\" or \"waypoint\"", p.Follower)
	}
	v.nonNegative("path_execution.waypoint_threshold", p.WaypointThreshold)
	v.nonNegative("path_execution.spin_threshold_deg", p.SpinThresholdDeg)
	v.nonNegative("path_execution.burst_frames", float64(p.BurstFrames))
	v.nonNegative("path_execution.max_wait_frames", float64(p.MaxWaitFrames))
	v.nonNegative("path_execution.forward_threshold_deg", p.ForwardThresholdDeg)
	v.nonNegative("path_execution.tracking_lost_timeout_s", p.TrackingLostTimeoutS)

	v.fraction("position.heading_smoothing_alpha", c.Position.HeadingSmoothingAlpha)
	v.nonNegative("position.heading_max_rate_deg", c.Position.HeadingMaxRateDeg)

	v.nonNegative("controller.serial.baudrate", float64(c.Controller.Serial.BaudRate))
	v.nonNegative("controller.command_interval", c.Controller.CommandInterval)
	v.nonNegative("controller.heartbeat_timeout", c.Controller.HeartbeatTimeout)
}

func (c *Config) validateForeground(v *validator) {
	f := c.Foreground
	v.fraction("foreground.scale", f.Scale)
	v.fraction("foreground.guard_fraction", f.GuardFraction)
	if f.Threshold < 0 || f.Threshold > 255 {
		v.add("foreground.threshold is %v, must be between 0 and 255", f.Threshold)
	}
	for field, value := range map[string]float64{
		"dark_factor": f.DarkFactor, "min_size_m": f.MinSizeM, "tau_sec": f.TauSec,
		"warmup_sec": f.WarmupSec, "robot_margin_m": f.RobotMarginM, "pad_m": f.PadM,
		"absorb_after_sec": f.AbsorbAfterSec, "persist_interval_sec": f.PersistIntervalSec,
		"appear_ms": float64(f.AppearMs), "vanish_ms": float64(f.VanishMs),
		"static_margin_px": float64(f.StaticMarginPx), "max_blobs": float64(f.MaxBlobs),
	} {
		v.nonNegative("foreground."+field, value)
	}
	for field, p := range map[string]*float64{
		"shadow_alpha_min": f.ShadowAlphaMin, "shadow_alpha_max": f.ShadowAlphaMax,
		"shadow_chroma_max": f.ShadowChromaMax,
	} {
		if p != nil {
			v.fraction("foreground."+field, *p)
		}
	}
	if f.ShadowAlphaMin != nil && f.ShadowAlphaMax != nil && *f.ShadowAlphaMin > *f.ShadowAlphaMax {
		v.add("foreground.shadow_alpha_min (%v) is above shadow_alpha_max (%v)", *f.ShadowAlphaMin, *f.ShadowAlphaMax)
	}
}

// unknownKeyWarning turns one yaml.v3 strict-decode message such as
// "line 12: field quad_sigma not found in type config.AprilTagConfig" into a
// warning, or reports false when the message is some other kind of error.
func unknownKeyWarning(msg string) (string, bool) {
	line, rest, ok := strings.Cut(msg, ": field ")
	if !ok || !strings.HasPrefix(line, "line ") {
		return "", false
	}
	key, _, ok := strings.Cut(rest, " not found in type ")
	if !ok {
		return "", false
	}
	return fmt.Sprintf("config %s: unknown key %q (ignored; check the spelling)", line, key), true
}
