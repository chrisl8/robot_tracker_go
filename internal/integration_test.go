package main

import (
	"testing"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/config"
	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/detection"
	"github.com/chrisl8/robot_tracker_go/internal/planning"
	"github.com/chrisl8/robot_tracker_go/internal/position"
	"github.com/chrisl8/robot_tracker_go/internal/tracking"
)

func TestDetectionToTrackingPipeline(t *testing.T) {
	tagConfig := detection.AprilTagConfig{
		Family:       "tag36h11",
		QuadDecimate: 2.0,
	}

	yoloConfig := &detection.YOLOConfig{
		ModelPath: "assets/yolov8n.onnx",
		InputSize: 640,
		ConfThres: 0.5,
		IOUThres:  0.45,
	}

	pipeline := detection.NewDetectionPipeline(yoloConfig, tagConfig)
	if pipeline == nil {
		t.Fatal("Failed to create detection pipeline")
	}

	trackConfig := &tracking.ByteTrackConfig{
		TrackThresh: 0.5,
		TrackBuffer: 30,
		MatchThresh: 0.8,
		FrameRate:   30,
		MinBoxArea:  100,
		MOT20:       false,
	}
	tracker := tracking.NewByteTrack(trackConfig)
	if tracker == nil {
		t.Fatal("Failed to create tracker")
	}

	timestamp := float64(time.Now().UnixNano()) / 1e9

	emptyImage := make([]byte, 640*480*3)

	result := pipeline.Detect(emptyImage, 640, 480, timestamp, 1)
	if result == nil {
		t.Fatal("Detection returned nil")
	}

	trackingDets := convertFusedToTracking(result.FusedDetections)
	trackingResult := tracker.Update(trackingDets, timestamp, 1)
	if trackingResult == nil {
		t.Fatal("Tracking returned nil")
	}

	t.Logf("Detection result: %d tags, %d YOLO detections, %d fused",
		len(result.Tags), len(result.YOLODetections), len(result.FusedDetections))
	t.Logf("Tracking result: %d tracks", len(trackingResult.Tracks))
}

func TestTrackingToPositionPipeline(t *testing.T) {
	estimator, err := position.NewPositionEstimator("", "", true, 0.3)
	if err != nil {
		t.Skip("Position estimator initialization failed (expected without calibration)")
	}
	if estimator == nil {
		t.Skip("Position estimator is nil")
	}

	timestamp := float64(time.Now().UnixNano()) / 1e9

	track := tracking.NewTrack(1, [4]int{100, 100, 200, 200}, timestamp, 0.9)
	if track == nil {
		t.Fatal("Failed to create track")
	}

	bbox := track.Bbox
	px, py := bbox[0]+(bbox[2]-bbox[0])/2, bbox[1]+(bbox[3]-bbox[1])/2
	worldPos := estimator.PixelToWorld(px, py)
	if worldPos == nil {
		t.Fatal("PixelToWorld returned nil")
	}

	estimator.UpdatePosition(track.TrackID, worldPos.X, worldPos.Y)
	retrievedPos := estimator.GetPosition(track.TrackID)
	if retrievedPos == nil {
		t.Fatal("GetPosition returned nil for known track")
	}

	t.Logf("Track ID: %d, World position: (%.2f, %.2f)", track.TrackID, retrievedPos.X, retrievedPos.Y)
}

func TestPlannerToControllerPipeline(t *testing.T) {
	plannerConfig := &planning.PlannerConfig{
		AStarConfig:            nil,
		VelocityObstacleConfig: nil,
		CollisionMargin:        0.05,
	}
	planner := planning.NewPlanner(plannerConfig)
	if planner == nil {
		t.Fatal("Failed to create planner")
	}

	executor := controller.NewPathExecutor(0.15, 1.0)
	if executor == nil {
		t.Fatal("Failed to create path executor")
	}

	planner.AddRobot(0, [2]float64{0, 0}, 0.18)
	planner.SetGoal(0, [2]float64{1.0, 0.0})

	commands := planner.ComputeAllCommands()
	if commands == nil {
		t.Fatal("ComputeAllCommands returned nil")
	}

	if vel, exists := commands[0]; exists {
		cmd := executor.VelocityToCommand(vel[0], vel[1])
		t.Logf("Robot 0 velocity: (%.2f, %.2f), command: %c", vel[0], vel[1], cmd)
	} else {
		t.Log("No command for robot 0 (expected if no goal set)")
	}
}

func TestFullPipelineIntegration(t *testing.T) {
	cfg := &config.Config{
		AprilTags: config.AprilTagConfig{
			Family:       "tag36h11",
			QuadDecimate: 2.0,
		},
		YOLO: config.YOLOConfig{
			ModelPath: "assets/yolov8n.onnx",
			InputSize: 640,
			ConfThres: 0.5,
			IOUThres:  0.45,
		},
		Tracking: config.TrackingConfig{
			TrackThresh: 0.5,
			TrackBuffer: 30,
			MatchThresh: 0.8,
			FrameRate:   30,
			MinBoxArea:  100,
			MOT20:       false,
		},
		Position: config.PositionConfig{
			Smoothing:        true,
			SmoothingAlpha:   0.3,
			OutlierThreshold: 0.1,
		},
		Planning: config.PlanningConfig{
			StepSize:       0.05,
			ReplanInterval: 0.5,
		},
	}

	tagConfig := detection.AprilTagConfig{
		Family:       cfg.AprilTags.Family,
		QuadDecimate: cfg.AprilTags.QuadDecimate,
	}
	yoloConfig := &detection.YOLOConfig{
		ModelPath: cfg.YOLO.ModelPath,
		InputSize: cfg.YOLO.InputSize,
		ConfThres: cfg.YOLO.ConfThres,
		IOUThres:  cfg.YOLO.IOUThres,
	}
	pipeline := detection.NewDetectionPipeline(yoloConfig, tagConfig)
	tracker := tracking.NewByteTrack(&tracking.ByteTrackConfig{
		TrackThresh: cfg.Tracking.TrackThresh,
		TrackBuffer: cfg.Tracking.TrackBuffer,
		MatchThresh: cfg.Tracking.MatchThresh,
		FrameRate:   cfg.Tracking.FrameRate,
		MinBoxArea:  cfg.Tracking.MinBoxArea,
		MOT20:       cfg.Tracking.MOT20,
	})
	planner := planning.NewPlanner(&planning.PlannerConfig{
		CollisionMargin: 0.05,
	})

	if pipeline == nil || tracker == nil || planner == nil {
		t.Fatal("Failed to create pipeline components")
	}

	timestamp := float64(time.Now().UnixNano()) / 1e9
	emptyImage := make([]byte, 640*480*3)

	detectionResult := pipeline.Detect(emptyImage, 640, 480, timestamp, 1)
	trackingDets := convertFusedToTracking(detectionResult.FusedDetections)
	trackingResult := tracker.Update(trackingDets, timestamp, 1)

	for _, track := range trackingResult.Tracks {
		if track.TagID != nil {
			planner.AddRobot(*track.TagID, [2]float64{0, 0}, 0.18)
		}
	}

	commands := planner.ComputeAllCommands()

	t.Logf("Pipeline test: detection=%d, tracking=%d, planning commands=%d",
		len(detectionResult.FusedDetections),
		len(trackingResult.Tracks),
		len(commands))
}

func convertFusedToTracking(fused []detection.FusedDetection) []tracking.Detection {
	detections := make([]tracking.Detection, 0, len(fused))
	for _, f := range fused {
		bbox := f.Bbox
		det := tracking.Detection{
			Bbox:       [4]int{bbox.X1, bbox.Y1, bbox.X2, bbox.Y2},
			Confidence: f.Confidence,
		}
		if f.TagID != nil {
			det.TagID = f.TagID
		}
		if f.ClassName != "" {
			classID := int(f.Confidence * 100)
			det.ClassID = classID
		}
		detections = append(detections, det)
	}
	return detections
}

func TestConfigLoadingIntegration(t *testing.T) {
	cfg, err := config.Load("../config/tracking_config.yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(cfg.Robots) == 0 {
		t.Log("No robots configured")
	}

	if cfg.AprilTags.Family == "" {
		t.Error("AprilTag family not configured")
	}

	// Temporary obstacles come from the foreground detector, which is on by
	// default; YOLO is intentionally off (empty model path).
	if !cfg.EffectiveForeground().Enabled {
		t.Error("foreground obstacle detection should be enabled in the shipped config")
	}

	if cfg.Tracking.TrackThresh == 0 {
		t.Error("Tracking threshold not configured")
	}

	t.Logf("Config: %d robots, AprilTag family=%s, foreground=%v, track_thresh=%.2f",
		len(cfg.Robots), cfg.AprilTags.Family, cfg.EffectiveForeground().Enabled, cfg.Tracking.TrackThresh)
}

func TestDetectionWithRealImage(t *testing.T) {
	tagConfig := detection.AprilTagConfig{
		Family:       "tag36h11",
		QuadDecimate: 2.0,
	}

	yoloConfig := &detection.YOLOConfig{
		ModelPath: "assets/yolov8n.onnx",
		InputSize: 640,
		ConfThres: 0.5,
		IOUThres:  0.45,
	}

	pipeline := detection.NewDetectionPipeline(yoloConfig, tagConfig)
	if pipeline == nil {
		t.Fatal("Failed to create pipeline")
	}

	timestamp := float64(time.Now().UnixNano()) / 1e9
	width, height := 640, 480

	emptyImage := make([]byte, width*height*3)

	result := pipeline.Detect(emptyImage, width, height, timestamp, 1)
	if result == nil {
		t.Fatal("Detection failed")
	}

	overlay := pipeline.DrawResults(emptyImage, width, height, result)
	if overlay == nil {
		t.Error("DrawResults returned nil")
	}

	t.Logf("Detection on empty image: tags=%d, yolo=%d, fused=%d",
		len(result.Tags), len(result.YOLODetections), len(result.FusedDetections))
}
