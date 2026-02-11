package controller

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.bug.st/serial"
)

type ArduinoController struct {
	port      string
	baudrate  int
	timeout   time.Duration
	serial    serial.Port
	connected bool
}

func NewArduinoController(port string, baudrate int) *ArduinoController {
	if baudrate == 0 {
		baudrate = BaudRate
	}
	return &ArduinoController{
		port:     port,
		baudrate: baudrate,
		timeout:  100 * time.Millisecond,
	}
}

func (c *ArduinoController) Connect() error {
	if c.connected {
		return nil
	}

	port := c.port
	if port == "auto" {
		var err error
		port, err = c.autoDetectPort()
		if err != nil {
			return fmt.Errorf("no Arduino port found: %w", err)
		}
	}

	mode := &serial.Mode{
		BaudRate: c.baudrate,
	}

	var err error
	c.serial, err = serial.Open(port, mode)
	if err != nil {
		return fmt.Errorf("failed to connect to Arduino: %w", err)
	}

	_ = c.serial.SetReadTimeout(c.timeout)
	c.port = port
	c.connected = true

	time.Sleep(ArduinoStartupDelay)

	return nil
}

func (c *ArduinoController) Disconnect() error {
	if c.serial != nil && c.connected {
		_ = c.serial.Close()
		c.serial = nil
		c.connected = false
	}
	return nil
}

func (c *ArduinoController) writeAll(data []byte) error {
	totalWritten := 0
	for totalWritten < len(data) {
		n, err := c.serial.Write(data[totalWritten:])
		if err != nil {
			return err
		}
		totalWritten += n
	}
	return nil
}

func (c *ArduinoController) SendCommand(cmd Command) error {
	if !c.connected || c.serial == nil {
		return ErrNotConnected
	}

	data := []byte{byte(cmd), '\r', '\n'}
	if err := c.writeAll(data); err != nil {
		c.connected = false
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	return nil
}

func (c *ArduinoController) SendMode(mode Mode) error {
	if !c.connected || c.serial == nil {
		return ErrNotConnected
	}

	data := []byte{byte(mode), '\r', '\n'}
	if err := c.writeAll(data); err != nil {
		c.connected = false
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	return nil
}

func (c *ArduinoController) SendSubmode(submode Submode) error {
	if !c.connected || c.serial == nil {
		return ErrNotConnected
	}

	data := []byte{byte(submode), '\r', '\n'}
	if err := c.writeAll(data); err != nil {
		c.connected = false
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	return nil
}

func (c *ArduinoController) ToggleDebug() error {
	if !c.connected || c.serial == nil {
		return ErrNotConnected
	}

	data := []byte{byte(CommandDebug), '\r', '\n'}
	if err := c.writeAll(data); err != nil {
		c.connected = false
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	return nil
}

func (c *ArduinoController) IsConnected() bool {
	return c.connected
}

func (c *ArduinoController) GetPort() string {
	return c.port
}

func (c *ArduinoController) autoDetectPort() (string, error) {
	ports, err := c.detectPorts()
	if err != nil {
		return "", err
	}
	if len(ports) == 0 {
		return "", fmt.Errorf("no Arduino ports found")
	}
	return ports[0], nil
}

func (c *ArduinoController) detectPorts() ([]string, error) {
	pattern := "/dev/tty*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	var arduinoPorts []string
	for _, port := range matches {
		if strings.Contains(port, "USB") || strings.Contains(port, "ACM") || strings.Contains(port, "AMA") {
			arduinoPorts = append(arduinoPorts, port)
		}
	}
	if len(arduinoPorts) == 0 {
		arduinoPorts = matches
	}
	return arduinoPorts, nil
}

func (c *ArduinoController) ListPorts() []string {
	ports, _ := c.detectPorts()
	return ports
}

const ArduinoStartupDelay = 2 * time.Second
