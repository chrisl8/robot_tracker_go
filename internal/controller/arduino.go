package controller

import (
	"fmt"
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

	c.serial.SetReadTimeout(c.timeout)
	c.port = port
	c.connected = true

	time.Sleep(ArduinoStartupDelay)

	return nil
}

func (c *ArduinoController) Disconnect() error {
	if c.serial != nil && c.connected {
		c.serial.Close()
		c.serial = nil
		c.connected = false
	}
	return nil
}

func (c *ArduinoController) SendCommand(cmd Command) error {
	if !c.connected || c.serial == nil {
		return ErrNotConnected
	}

	data := []byte{byte(cmd), '\r', '\n'}
	_, err := c.serial.Write(data)
	if err != nil {
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
	_, err := c.serial.Write(data)
	if err != nil {
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
	_, err := c.serial.Write(data)
	if err != nil {
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
	_, err := c.serial.Write(data)
	if err != nil {
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
	return "", fmt.Errorf("port auto-detect not available on this platform")
}

func (c *ArduinoController) ListPorts() []string {
	return []string{}
}

const ArduinoStartupDelay = 2 * time.Second
