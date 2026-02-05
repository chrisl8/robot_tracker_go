package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"robot_tracker_go/internal/config"
	"robot_tracker_go/internal/controller"
)

func main() {
	configPath := flag.String("config", "config/tracking_config.yaml", "Path to configuration file")
	port := flag.String("port", "auto", "Serial port (auto-detect if not specified)")
	listPorts := flag.Bool("list-ports", false, "List available serial ports")
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

	_, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Warning: Could not load config: %v", err)
	}

	arduino := controller.NewArduinoController(*port, controller.BaudRate)
	if err := arduino.Connect(); err != nil {
		log.Fatalf("Failed to connect to Arduino: %v", err)
	}
	defer arduino.Disconnect()

	fmt.Printf("Connected to Arduino on %s\n", arduino.GetPort())

	queue := controller.NewCommandQueue(arduino, controller.CommandIntervalMs)
	queue.Start()
	defer queue.Stop()

	fmt.Println("Command queue started. Press Ctrl+C to exit.")

	executor := controller.NewPathExecutor(0.15, 1.0)

	testCommands := []controller.Command{
		controller.CommandForward,
		controller.CommandLeft,
		controller.CommandRight,
		controller.CommandStop,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for _, cmd := range testCommands {
		fmt.Printf("Sending command: %c\n", cmd)
		queue.Enqueue(cmd)
		vel := executor.CommandToVelocity(cmd)
		fmt.Printf("  Velocity: (%.2f, %.2f)\n", vel.VX, vel.VY)
	}

	<-sigCh
	fmt.Println("\nShutting down...")
}
