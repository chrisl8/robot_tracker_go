package controller

import "fmt"

// Command is a single-byte robot command; its value is the ASCII letter sent over the serial line.
type Command byte

const (
	// CommandForward walks the robot forward.
	CommandForward Command = 'f'
	// CommandBackward walks the robot backward.
	CommandBackward Command = 'b'
	// CommandLeft turns the robot left.
	CommandLeft Command = 'l'
	// CommandRight turns the robot right.
	CommandRight Command = 'r'
	// CommandWeapon is part of the wire protocol's vocabulary but is never sent by this program.
	CommandWeapon Command = 'w'
	// CommandStop stops the robot. It is the only command a halted queue still sends.
	CommandStop Command = 's'
	// CommandQuery is part of the wire protocol's vocabulary but is never sent by this program.
	CommandQuery Command = '?'
	// CommandDebug is part of the wire protocol's vocabulary but is never sent by this program.
	CommandDebug Command = 'd'
)

// BaudRate is the default serial speed. The gamepad and the robot's Bluetooth radios both run at 9600.
const BaudRate = 9600

// CommandIntervalMs is the default minimum spacing between commands sent to the Arduino, in milliseconds.
const CommandIntervalMs = 50

// HeartbeatTimeoutMs is the default time without a write before the queue re-sends the active command (or Stop), in milliseconds.
const HeartbeatTimeoutMs = 500

// encodeCommand is the wire format for a single robot command: the command
// byte followed by CRLF. The firmware has no robot address, so one serial line
// drives exactly one robot.
func encodeCommand(cmd Command) []byte {
	return []byte{byte(cmd), '\r', '\n'}
}

// ErrNotConnected is returned when a command is sent without an open serial port.
var ErrNotConnected = fmt.Errorf("not connected")

// ErrSendFailed is returned when a command could not be written to the serial port.
var ErrSendFailed = fmt.Errorf("failed to send command")
