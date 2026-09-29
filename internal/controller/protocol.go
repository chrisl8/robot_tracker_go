package controller

import "fmt"

type Command byte

const (
	CommandForward  Command = 'f'
	CommandBackward Command = 'b'
	CommandLeft     Command = 'l'
	CommandRight    Command = 'r'
	CommandWeapon   Command = 'w'
	CommandStop     Command = 's'
	CommandQuery    Command = '?'
	CommandDebug    Command = 'd'
)

const BaudRate = 9600
const CommandIntervalMs = 50
const HeartbeatTimeoutMs = 500

// encodeCommand is the wire format for a single robot command: the command
// byte followed by CRLF. The firmware has no robot address, so one serial line
// drives exactly one robot.
func encodeCommand(cmd Command) []byte {
	return []byte{byte(cmd), '\r', '\n'}
}

var ErrNotConnected = fmt.Errorf("not connected")
var ErrSendFailed = fmt.Errorf("failed to send command")
