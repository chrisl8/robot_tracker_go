package controller

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

type ArduinoController struct {
	port      string
	baudrate  int
	timeout   time.Duration
	serial    serial.Port
	connected bool
	mu        sync.Mutex // guards serial and connected
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
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

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

	sp, err := serial.Open(port, mode)
	if err != nil {
		return fmt.Errorf("failed to connect to Arduino: %w", err)
	}
	_ = sp.SetReadTimeout(c.timeout)

	c.mu.Lock()
	c.serial = sp
	c.port = port
	c.connected = true
	c.mu.Unlock()

	time.Sleep(ArduinoStartupDelay)

	return nil
}

func (c *ArduinoController) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *ArduinoController) GetPort() string {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	patterns := []string{"/dev/cu.*", "/dev/tty*"}
	var allMatches []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		allMatches = append(allMatches, matches...)
	}

	var arduinoPorts []string
	for _, port := range allMatches {
		lower := strings.ToLower(port)
		if strings.Contains(lower, "usb") || strings.Contains(lower, "acm") || strings.Contains(lower, "ama") || strings.Contains(lower, "modem") {
			arduinoPorts = append(arduinoPorts, port)
		}
	}
	if len(arduinoPorts) == 0 {
		arduinoPorts = allMatches
	}
	return arduinoPorts, nil
}

func (c *ArduinoController) ListPorts() []string {
	ports, _ := c.detectPorts()
	return ports
}

const ArduinoStartupDelay = 2 * time.Second
