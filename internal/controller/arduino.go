package controller

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

// Seams for tests: the real serial port opener and the post-open delay that
// lets the Arduino finish resetting (opening the port resets most boards).
var (
	openSerial          = serial.Open
	arduinoStartupDelay = ArduinoStartupDelay
)

type ArduinoController struct {
	port      string
	baudrate  int
	timeout   time.Duration
	serial    serial.Port
	connected bool
	// connecting is set while Connect opens the port and waits out the
	// board's reset, so a second Connect can't open the port twice.
	connecting bool
	mu         sync.Mutex // guards serial, connected and connecting
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

// Connect opens the serial port. The controller only reports connected once
// the board has had ArduinoStartupDelay to finish the reset that opening the
// port triggers: marking it connected earlier let writes (including an
// emergency Stop) land while the board was still resetting and be lost.
func (c *ArduinoController) Connect() error {
	c.mu.Lock()
	if c.connected || c.connecting {
		c.mu.Unlock()
		return nil
	}
	c.connecting = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.connecting = false
		c.mu.Unlock()
	}()

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

	sp, err := openSerial(port, mode)
	if err != nil {
		return fmt.Errorf("failed to connect to Arduino: %w", err)
	}
	_ = sp.SetReadTimeout(c.timeout)

	time.Sleep(arduinoStartupDelay)

	c.mu.Lock()
	c.serial = sp
	c.port = port
	c.connected = true
	c.mu.Unlock()

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

	if err := c.writeAll(encodeCommand(cmd)); err != nil {
		c.dropConnectionLocked()
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	return nil
}

// dropConnectionLocked closes and forgets the serial port after a failed
// write. Merely flagging it disconnected left the handle open, so a
// reconnect to the same port would fail as "busy". Callers hold c.mu.
func (c *ArduinoController) dropConnectionLocked() {
	if c.serial != nil {
		_ = c.serial.Close()
		c.serial = nil
	}
	c.connected = false
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
	ports := filterArduinoPorts(globSerialPorts())
	if len(ports) == 0 {
		return "", fmt.Errorf("no Arduino ports found")
	}
	return ports[0], nil
}

// globSerialPorts lists every serial device node the OS exposes.
func globSerialPorts() []string {
	var all []string
	for _, pattern := range []string{"/dev/cu.*", "/dev/tty*"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		all = append(all, matches...)
	}
	return all
}

// filterArduinoPorts keeps the ports that look like a USB serial adapter or
// board. There is deliberately no fallback to "every port that exists": with no
// board attached that would pick something like /dev/cu.Bluetooth-Incoming-Port,
// report the Arduino as connected, and write drive commands into it. An adapter
// with an unusual name can be selected with controller.serial.port.
func filterArduinoPorts(all []string) []string {
	var out []string
	for _, port := range all {
		lower := strings.ToLower(port)
		if strings.Contains(lower, "usb") || strings.Contains(lower, "acm") || strings.Contains(lower, "ama") || strings.Contains(lower, "modem") {
			out = append(out, port)
		}
	}
	return out
}

// ListPorts returns every serial device, for --list-ports; auto-detection only
// considers the ones that look like an Arduino (see filterArduinoPorts).
func (c *ArduinoController) ListPorts() []string {
	return globSerialPorts()
}

const ArduinoStartupDelay = 2 * time.Second
