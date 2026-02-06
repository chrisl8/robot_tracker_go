//go:build gocv

package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"robot_tracker_go/internal/camera"
	"robot_tracker_go/internal/config"
	"robot_tracker_go/internal/controller"
	"robot_tracker_go/internal/detection"
	"robot_tracker_go/internal/planning"
	"robot_tracker_go/internal/position"
	"robot_tracker_go/internal/tracking"
	"robot_tracker_go/internal/ui"
)

type RobotSystem struct {
	cfg           *config.Config
	cam           camera.Camera
	detectionPipe *detection.DetectionPipeline
	tracker       tracking.Tracker
	planner       *planning.Planner
	positionEst   *position.PositionEstimator
	arduino       *controller.ArduinoController
	commandQueue  *controller.CommandQueue
	webServer     *ui.WebServer
	cameraRunning bool
	frameNum      int
}

func NewRobotSystem(cfg *config.Config) *RobotSystem {
	return &RobotSystem{
		cfg:           cfg,
		cameraRunning: false,
		frameNum:      0,
	}
}

func (rs *RobotSystem) Initialize() error {
	tagConfig := detection.AprilTagConfig{
		Family:       rs.cfg.AprilTags.Family,
		QuadDecimate: rs.cfg.AprilTags.QuadDecimate,
	}

	yoloConfig := &detection.YOLOConfig{
		ModelPath: rs.cfg.YOLO.ModelPath,
		InputSize: rs.cfg.YOLO.InputSize,
		ConfThres: rs.cfg.YOLO.ConfThres,
		IOUThres:  rs.cfg.YOLO.IOUThres,
		Device:    rs.cfg.YOLO.Device,
	}

	rs.detectionPipe = detection.NewDetectionPipeline(yoloConfig, tagConfig)
	log.Printf("Detection pipeline initialized, YOLO enabled: %v", rs.detectionPipe.IsYOLOEnabled())

	trackConfig := &tracking.ByteTrackConfig{
		TrackThresh: rs.cfg.Tracking.TrackThresh,
		TrackBuffer: rs.cfg.Tracking.TrackBuffer,
		MatchThresh: rs.cfg.Tracking.MatchThresh,
		FrameRate:   rs.cfg.Tracking.FrameRate,
		MinBoxArea:  rs.cfg.Tracking.MinBoxArea,
		MOT20:       rs.cfg.Tracking.MOT20,
	}
	rs.tracker = tracking.NewByteTrack(trackConfig)
	log.Printf("ByteTrack initialized")

	plannerConfig := &planning.PlannerConfig{
		AStarConfig:            nil,
		VelocityObstacleConfig: nil,
		CollisionMargin:        0.05,
	}
	rs.planner = planning.NewPlanner(plannerConfig)
	log.Printf("Planner initialized")

	if primaryCam := rs.cfg.GetPrimaryCamera(); primaryCam != nil {
		camConfig := camera.CameraConfig{
			Type:     primaryCam.Type,
			Name:     primaryCam.Name,
			CameraID: primaryCam.CameraID,
			URL:      primaryCam.URL,
			Width:    primaryCam.Width,
			Height:   primaryCam.Height,
			FPS:      primaryCam.FPS,
		}
		cam, err := camera.NewCamera(camConfig)
		if err != nil {
			log.Printf("Warning: Could not initialize camera: %v", err)
			rs.cam = nil
		} else {
			rs.cam = cam
			log.Printf("Camera initialized: %s", rs.cam.GetName())
		}
	} else {
		log.Printf("No camera configured, using demo mode")
	}

	calibrationPath := "config/calibration_default.yaml"
	if rs.cam != nil {
		calibrationPath = ui.GetCalibrationFilename(rs.cam.GetName())
		log.Printf("Using calibration file: %s", calibrationPath)
	}
	obstaclesPath := ""
	if rs.cfg.Obstacles.Path != "" {
		obstaclesPath = rs.cfg.Obstacles.Path
	}
	posEst, err := position.NewPositionEstimator(calibrationPath, obstaclesPath, rs.cfg.Position.Smoothing, rs.cfg.Position.SmoothingAlpha)
	if err != nil {
		log.Printf("Warning: Position estimator initialization failed: %v", err)
		rs.positionEst = nil
	} else {
		rs.positionEst = posEst
		log.Printf("Position estimator initialized")
	}

	rs.arduino = controller.NewArduinoController("auto", controller.BaudRate)
	if err := rs.arduino.Connect(); err != nil {
		log.Printf("Warning: Could not connect to Arduino: %v", err)
	} else {
		log.Printf("Connected to Arduino on %s", rs.arduino.GetPort())
	}

	rs.commandQueue = controller.NewCommandQueue(rs.arduino, controller.CommandIntervalMs)
	rs.commandQueue.Start()
	log.Printf("Command queue started")

	rs.webServer = ui.NewWebServer(":8080")
	if rs.cam != nil {
		rs.webServer.SetCameraName(rs.cam.GetName())
	}
	if rs.positionEst != nil && rs.positionEst.IsCalibrated() {
		rs.webServer.SetCalibrationState("calibrated", "Calibration loaded", calibrationPath, 0.15)
		log.Printf("Calibration loaded from %s", calibrationPath)
	}
	rs.webServer.Start()
	log.Printf("Web UI started at http://localhost:8080")

	return nil
}

func (rs *RobotSystem) StartCamera() error {
	log.Printf("Starting camera...")
	if rs.cam == nil {
		log.Printf("No camera available")
		rs.cameraRunning = false
		return nil
	}
	if err := rs.cam.Start(); err != nil {
		log.Printf("Failed to start camera: %v", err)
		rs.cameraRunning = false
		return err
	}
	rs.cameraRunning = true
	log.Printf("Camera started: %s", rs.cam.GetName())
	return nil
}

func (rs *RobotSystem) Stop() {
	log.Printf("Stopping system...")
	rs.cameraRunning = false
	if rs.cam != nil {
		rs.cam.Stop()
	}
	rs.commandQueue.Stop()
	if rs.arduino != nil {
		rs.arduino.Disconnect()
	}
	rs.webServer.Stop()
	log.Printf("System stopped")
}

func (rs *RobotSystem) convertFusedToTrackingDetections(fused []detection.FusedDetection) []tracking.Detection {
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

func (rs *RobotSystem) ProcessFrame(img image.Image, frameData []byte) {
	if img == nil {
		return
	}

	rs.frameNum++
	timestamp := float64(time.Now().UnixNano()) / 1e9

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	rgbaImg, ok := img.(*image.RGBA)
	if !ok {
		rgbaImg = image.NewRGBA(bounds)
		draw.Draw(rgbaImg, bounds, img, bounds.Min, draw.Src)
	}

	detectionResult := rs.detectionPipe.Detect(frameData, width, height, timestamp, rs.frameNum)

	trackingDetections := rs.convertFusedToTrackingDetections(detectionResult.FusedDetections)
	trackingResult := rs.tracker.Update(trackingDetections, timestamp, rs.frameNum)

	for _, track := range trackingResult.Tracks {
		if track.State == tracking.TrackStateConfirmed && track.TagID != nil {
			if rs.positionEst != nil {
				px, py := track.Bbox[0]+track.Bbox[2]/2, track.Bbox[1]+track.Bbox[3]/2
				worldPos := rs.positionEst.PixelToWorld(px, py)
				rs.positionEst.UpdatePosition(track.TrackID, worldPos.X, worldPos.Y)
			}
		}
	}

	overlay := rs.detectionPipe.DrawResults(frameData, width, height, detectionResult)
	if len(overlay) > 0 && len(overlay) < width*height*3 {
		rs.webServer.PushRawJPEG(overlay)
	} else if img != nil {
		rs.webServer.PushFrame(img)
	}

	rs.webServer.UpdateStats(len(detectionResult.Tags), len(detectionResult.YOLODetections))

	detectedTags := make([]ui.DetectedTagInfo, 0, len(detectionResult.Tags))
	for _, tag := range detectionResult.Tags {
		detectedTags = append(detectedTags, ui.DetectedTagInfo{
			ID:      tag.TagID,
			Center:  [2]float64{tag.CenterX, tag.CenterY},
			Corners: tag.Corners,
		})
	}
	rs.webServer.UpdateDetectedTags(detectedTags)
}

func decodeToImage(data []byte, width, height int) image.Image {
	if len(data) == 0 {
		return nil
	}
	expectedLen := width * height * 3
	if len(data) != expectedLen {
		return nil
	}
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < width*height; i++ {
		b := data[i*3]
		g := data[i*3+1]
		r := data[i*3+2]
		rgba.Pix[i*4] = r
		rgba.Pix[i*4+1] = g
		rgba.Pix[i*4+2] = b
		rgba.Pix[i*4+3] = 255
	}
	return rgba
}

func cameraFrameToImage(frame *camera.Frame) image.Image {
	if frame == nil || len(frame.Data) == 0 {
		return nil
	}
	if frame.Width <= 0 || frame.Height <= 0 {
		return nil
	}

	// Handle based on channel count
	switch frame.Channels {
	case 3:
		width := frame.Width
		height := frame.Height
		rgba := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				srcIdx := y*frame.Width*3 + x*3
				dstIdx := (y*width + x) * 4
				rgba.Pix[dstIdx+0] = frame.Data[srcIdx+2] // R (from BGR)
				rgba.Pix[dstIdx+1] = frame.Data[srcIdx+1] // G
				rgba.Pix[dstIdx+2] = frame.Data[srcIdx+0] // B
				rgba.Pix[dstIdx+3] = 255                  // A
			}
		}
		return rgba
	case 4:
		rgba := &image.RGBA{
			Pix:    frame.Data,
			Stride: frame.Width * frame.Channels,
			Rect:   image.Rect(0, 0, frame.Width, frame.Height),
		}
		return rgba
	case 1:
		gray := &image.Gray{
			Pix:    frame.Data,
			Stride: frame.Width,
			Rect:   image.Rect(0, 0, frame.Width, frame.Height),
		}
		return gray
	default:
		return nil
	}
}

func generateTestPattern(width, height int, frameNum int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bgColor := color.RGBA{20, 20, 40, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	gridColor := color.RGBA{50, 50, 70, 255}
	for x := 0; x < width; x += 50 {
		for y := 0; y < height; y++ {
			img.Set(x, y, gridColor)
		}
	}
	for y := 0; y < height; y += 50 {
		for x := 0; x < width; x++ {
			img.Set(x, y, gridColor)
		}
	}

	numRobots := 3
	for i := 0; i < numRobots; i++ {
		_ = float64(frameNum+i*100) * 0.02
		radius := 100.0
		cx := float64(width)/2 + float64(i-1)*80
		cy := float64(height) / 2
		x := int(cx + radius*float64(i)*0.3*float64(frameNum)*0.01)
		y := int(cy + radius*float64(i)*0.5*float64(frameNum)*0.01)

		robotColor := color.RGBA{uint8(78 + i*50), uint8(204 - i*30), 163, 255}
		for dy := -20; dy <= 20; dy++ {
			for dx := -20; dx <= 20; dx++ {
				if dx*dx+dy*dy <= 400 {
					img.Set(x+dx, y+dy, robotColor)
				}
			}
		}

		for tx := x - 25; tx <= x+25; tx++ {
			if tx >= 0 && tx < width && y-35 >= 0 && y-35 < height {
				img.Set(tx, y-35, color.RGBA{0, 0, 0, 200})
			}
		}
	}

	timeStr := time.Now().Format("15:04:05")
	for x := 0; x < 7*10; x++ {
		for y := 20; y < 35; y++ {
			if x < len(timeStr)*10 {
				img.Set(width-100+x, y, color.RGBA{0, 0, 0, 200})
			}
		}
	}
	for x := 0; x < len(timeStr)*10; x++ {
		for y := 20; y < 35; y++ {
			img.Set(width-100+x, y+1, color.White)
		}
	}

	return img
}

func generateDemoTags(width, height int, frameNum int) []detection.AprilTag {
	tags := []detection.AprilTag{}

	tagCount := 3
	for i := 0; i < tagCount; i++ {
		angle := float64(frameNum+i*100) * 0.01
		radius := 100.0 + float64(i)*30
		cx := float64(width)/2 + radius*float64(i-1)*0.2*float64(frameNum)*0.01
		cy := float64(height)/2 + radius*float64(i)*0.3*float64(frameNum)*0.01

		size := 60.0
		corners := [4][2]float64{
			{cx - size, cy - size},
			{cx + size, cy - size},
			{cx + size, cy + size},
			{cx - size, cy + size},
		}

		tags = append(tags, detection.AprilTag{
			TagID:    i + 1,
			Family:   "tag36h11",
			Corners:  corners,
			CenterX:  cx,
			CenterY:  cy,
			Size:     size * 2,
			Rotation: angle,
		})
	}

	return tags
}

func drawDemoTagsOnImage(img *image.RGBA, tags []detection.AprilTag) *image.RGBA {
	borderColor := color.RGBA{0, 255, 0, 255}
	bgColor := color.RGBA{0, 255, 0, 200}
	centerColor := color.RGBA{255, 255, 255, 255}

	for _, tag := range tags {
		points := make([]image.Point, 4)
		for j := 0; j < 4; j++ {
			points[j] = image.Point{
				X: int(tag.Corners[j][0]),
				Y: int(tag.Corners[j][1]),
			}
		}

		lineWidth := 3
		for j := 0; j < 4; j++ {
			drawLineOnRGBA(img, points[j], points[(j+1)%4], borderColor, lineWidth)
		}

		cx := int(tag.CenterX)
		cy := int(tag.CenterY) - 25

		fontSize := 20
		boxWidth := fontSize
		boxHeight := fontSize

		boxRect := image.Rect(cx-boxWidth/2, cy, cx+boxWidth/2, cy+boxHeight)
		draw.Draw(img, boxRect, &image.Uniform{bgColor}, image.Point{}, draw.Src)

		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				if cx+dx >= 0 && cx+dx < img.Rect.Max.X && cy+dy >= 0 && cy+dy < img.Rect.Max.Y {
					img.Set(cx+dx, cy+dy, centerColor)
				}
			}
		}
	}

	return img
}

func drawLineOnRGBA(img *image.RGBA, p1, p2 image.Point, c color.RGBA, width int) {
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y

	if abs(dx) > abs(dy) {
		if p1.X > p2.X {
			p1, p2 = p2, p1
		}
		for x := p1.X; x <= p2.X; x++ {
			y := p1.Y + dy*(x-p1.X)/dx
			drawCircleOnRGBA(img, x, y, width/2, c)
		}
	} else {
		if p1.Y > p2.Y {
			p1, p2 = p2, p1
		}
		for y := p1.Y; y <= p2.Y; y++ {
			x := p1.X + dx*(y-p1.Y)/dy
			drawCircleOnRGBA(img, x, y, width/2, c)
		}
	}
}

func drawCircleOnRGBA(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				x := cx + dx
				y := cy + dy
				if x >= 0 && x < img.Rect.Max.X && y >= 0 && y < img.Rect.Max.Y {
					img.Set(x, y, c)
				}
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (rs *RobotSystem) ProcessDemoFrame(img *image.RGBA, frameNum int, demoTags []detection.AprilTag) {
	if img == nil {
		return
	}

	rs.frameNum++
	timestamp := float64(time.Now().UnixNano()) / 1e9

	width := img.Rect.Max.X
	height := img.Rect.Max.Y

	result := &detection.DetectionResult{
		Tags:            demoTags,
		YOLODetections:  []detection.YOLODetection{},
		FusedDetections: []detection.FusedDetection{},
		Timestamp:       timestamp,
		FrameIdx:        rs.frameNum,
	}

	for _, tag := range demoTags {
		bbox := detection.BoundingBox{
			X1: int(tag.Corners[0][0]),
			Y1: int(tag.Corners[0][1]),
			X2: int(tag.Corners[2][0]),
			Y2: int(tag.Corners[2][1]),
		}
		tagID := tag.TagID
		result.FusedDetections = append(result.FusedDetections, detection.FusedDetection{
			DetectionType: detection.DetectionTypeAprilTag,
			Bbox:          &bbox,
			TagID:         &tagID,
			Confidence:    1.0,
			Corners:       tag.Corners,
			Source:        "april_tag",
		})
	}

	trackingDetections := rs.convertFusedToTrackingDetections(result.FusedDetections)
	trackingResult := rs.tracker.Update(trackingDetections, timestamp, rs.frameNum)

	for _, track := range trackingResult.Tracks {
		if track.State == tracking.TrackStateConfirmed && track.TagID != nil {
			if rs.positionEst != nil {
				px, py := track.Bbox[0]+track.Bbox[2]/2, track.Bbox[1]+track.Bbox[3]/2
				worldPos := rs.positionEst.PixelToWorld(px, py)
				rs.positionEst.UpdatePosition(track.TrackID, worldPos.X, worldPos.Y)
			}
		}
	}

	overlay := rs.detectionPipe.DrawResults(img.Pix, width, height, result)
	if overlay != nil {
		overlayImg := decodeToImage(overlay, width, height)
		if overlayImg != nil {
			rs.webServer.PushFrame(overlayImg)
		} else {
			rs.webServer.PushFrame(img)
		}
	} else {
		rs.webServer.PushFrame(img)
	}

	rs.webServer.UpdateStats(len(demoTags), 0)

	detectedTags := make([]ui.DetectedTagInfo, 0, len(demoTags))
	for _, tag := range demoTags {
		detectedTags = append(detectedTags, ui.DetectedTagInfo{
			ID:      tag.TagID,
			Center:  [2]float64{tag.CenterX, tag.CenterY},
			Corners: tag.Corners,
		})
	}
	rs.webServer.UpdateDetectedTags(detectedTags)
}

func main() {
	configPath := flag.String("config", "config/tracking_config.yaml", "Path to configuration file")
	listPorts := flag.Bool("list-ports", false, "List available serial ports")
	flag.String("web-port", ":8080", "Web server port")
	demoMode := flag.Bool("demo", false, "Run demo mode with test pattern")
	flag.Parse()

	if *listPorts {
		arduino := controller.NewArduinoController("auto", controller.BaudRate)
		ports := arduino.ListPorts()
		fmt.Println("Available serial ports:")
		for _, p := range ports {
			fmt.Printf("  - %s\n", p)
		}
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Warning: Could not load config: %v", err)
		log.Printf("Running with demo mode only")
		*demoMode = true
	}

	rs := NewRobotSystem(cfg)
	if cfg != nil {
		rs.Initialize()
	}
	defer rs.Stop()

	fmt.Println("Press Ctrl+C to exit.")

	if rs.cam != nil && !*demoMode {
		fmt.Println("Starting real camera capture...")
		if err := rs.StartCamera(); err != nil {
			log.Printf("Failed to start camera: %v, falling back to demo mode", err)
			*demoMode = true
		} else {
			log.Printf("Starting real camera capture...")
			frameNum := 0
			for rs.cameraRunning {
				startTime := time.Now()
				frame, err := rs.cam.GetFrame()
				if err != nil {
					log.Printf("Failed to get frame: %v", err)
					time.Sleep(100 * time.Millisecond)
					continue
				}
				if frame == nil || len(frame.Data) == 0 {
					log.Printf("Empty frame received")
					time.Sleep(100 * time.Millisecond)
					continue
				}
				log.Printf("Frame %d: %dx%d, %d bytes, channels=%d (capture time: %v)",
					frameNum, frame.Width, frame.Height, len(frame.Data), frame.Channels, time.Since(startTime))
				img := cameraFrameToImage(frame)
				if img == nil {
					log.Printf("Failed to convert frame to image")
					time.Sleep(100 * time.Millisecond)
					continue
				}
				rs.ProcessFrame(img, frame.Data)
				frameNum++
				elapsed := time.Since(startTime)
				if elapsed < 33*time.Millisecond {
					time.Sleep(33*time.Millisecond - elapsed)
				}
			}
		}
	}

	for *demoMode {
		fmt.Println("Demo mode: Generating test pattern with AprilTag visualization...")
		rs.StartCamera()
		frameNum := 0
		for {
			frame := generateTestPattern(640, 480, frameNum)
			demoTags := generateDemoTags(640, 480, frameNum)
			rgbaImg, ok := frame.(*image.RGBA)
			if !ok {
				rgbaImg = image.NewRGBA(frame.Bounds())
				draw.Draw(rgbaImg, frame.Bounds(), frame, frame.Bounds().Min, draw.Src)
			}
			rgbaWithTags := drawDemoTagsOnImage(rgbaImg, demoTags)
			rs.ProcessDemoFrame(rgbaWithTags, frameNum, demoTags)
			frameNum++
			time.Sleep(33 * time.Millisecond)
		}
	}

	if cfg != nil {
		executor := controller.NewPathExecutor(0.15, 1.0)
		testCommands := []controller.Command{
			controller.CommandForward,
			controller.CommandLeft,
			controller.CommandRight,
			controller.CommandStop,
		}

		for _, cmd := range testCommands {
			fmt.Printf("Sending command: %c\n", cmd)
			rs.commandQueue.Enqueue(cmd)
			vel := executor.CommandToVelocity(cmd)
			fmt.Printf("  Velocity: (%.2f, %.2f)\n", vel.VX, vel.VY)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\nShutting down...")
}
