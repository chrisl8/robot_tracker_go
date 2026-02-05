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

	"robot_tracker_go/internal/config"
	"robot_tracker_go/internal/controller"
	"robot_tracker_go/internal/ui"
)

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

	borderColor := color.RGBA{78, 204, 163, 255}
	for x := 0; x < width; x++ {
		img.Set(x, 10, borderColor)
		img.Set(x, height-10, borderColor)
	}
	for y := 0; y < height; y++ {
		img.Set(10, y, borderColor)
		img.Set(width-10, y, borderColor)
	}

	numRobots := 3
	for i := 0; i < numRobots; i++ {
		angle := float64(frameNum+i*100) * 0.02
		_ = angle
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

		if x+2 >= 0 && x+2 < width && y-30 >= 0 && y-30 < height {
			draw.Draw(img, image.Rect(x-25, y-35, x+25, y-25), &image.Uniform{color.RGBA{0, 0, 0, 200}}, image.Point{}, draw.Src)
			img.Set(x+2, y-30, color.White)
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

func main() {
	configPath := flag.String("config", "config/tracking_config.yaml", "Path to configuration file")
	port := flag.String("port", "auto", "Serial port (auto-detect if not specified)")
	listPorts := flag.Bool("list-ports", false, "List available serial ports")
	webPort := flag.String("web-port", ":8080", "Web server port")
	demoMode := flag.Bool("demo", false, "Run demo mode with test pattern")
	flag.Parse()

	if *listPorts {
		arduino := controller.NewArduinoController(*port, 0)
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
	}

	arduino := controller.NewArduinoController(*port, controller.BaudRate)
	if err := arduino.Connect(); err != nil {
		log.Printf("Warning: Could not connect to Arduino: %v", err)
	} else {
		defer arduino.Disconnect()
		fmt.Printf("Connected to Arduino on %s\n", arduino.GetPort())
	}

	queue := controller.NewCommandQueue(arduino, controller.CommandIntervalMs)
	queue.Start()
	defer queue.Stop()

	webServer := ui.NewWebServer(*webPort)
	webServer.Start()
	defer webServer.Stop()

	fmt.Printf("Web UI started at http://localhost%s\n", *webPort)
	fmt.Println("Command queue started. Press Ctrl+C to exit.")

	if *demoMode {
		fmt.Println("Demo mode: Generating test pattern...")
		frameNum := 0
		for {
			frame := generateTestPattern(640, 480, frameNum)
			webServer.PushFrame(frame)
			frameNum++
			time.Sleep(33 * time.Millisecond)
		}
	}

	if cfg != nil {
		log.Printf("Config loaded: %+v", cfg)
	}

	executor := controller.NewPathExecutor(0.15, 1.0)
	testCommands := []controller.Command{
		controller.CommandForward,
		controller.CommandLeft,
		controller.CommandRight,
		controller.CommandStop,
	}

	for _, cmd := range testCommands {
		fmt.Printf("Sending command: %c\n", cmd)
		queue.Enqueue(cmd)
		vel := executor.CommandToVelocity(cmd)
		fmt.Printf("  Velocity: (%.2f, %.2f)\n", vel.VX, vel.VY)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	fmt.Println("\nShutting down...")
}
