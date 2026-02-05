package controller

import "fmt"

type Command byte

const (
	CommandForward  Command = 'F'
	CommandBackward Command = 'B'
	CommandLeft     Command = 'L'
	CommandRight    Command = 'R'
	CommandWeapon   Command = 'W'
	CommandStop     Command = 'S'
	CommandQuery    Command = '?'
	CommandDebug    Command = 'd'
)

type Mode byte

const (
	ModeWalk  Mode = 'W'
	ModeDance Mode = 'D'
	ModeFight Mode = 'F'
)

type Submode byte

const (
	SubmodeOne   Submode = '1'
	SubmodeTwo   Submode = '2'
	SubmodeThree Submode = '3'
	SubmodeFour  Submode = '4'
)

const BaudRate = 9600
const CommandIntervalMs = 50
const HeartbeatTimeoutMs = 500

type SerialProtocol struct{}

func NewSerialProtocol() *SerialProtocol {
	return &SerialProtocol{}
}

func (p *SerialProtocol) EncodeCommand(cmd Command) []byte {
	return []byte{byte(cmd), '\r', '\n'}
}

func (p *SerialProtocol) EncodeMode(mode Mode) []byte {
	return []byte{byte(mode), '\r', '\n'}
}

func (p *SerialProtocol) EncodeSubmode(submode Submode) []byte {
	return []byte{byte(submode), '\r', '\n'}
}

func (p *SerialProtocol) EncodeDebugToggle() []byte {
	return []byte{byte(CommandDebug), '\r', '\n'}
}

func (p *SerialProtocol) Decode(data []byte) (Command, error) {
	if len(data) < 1 {
		return 0, ErrInvalidData
	}
	return Command(data[0]), nil
}

var ErrInvalidData = fmt.Errorf("invalid serial data")
var ErrNotConnected = fmt.Errorf("not connected")
var ErrSendFailed = fmt.Errorf("failed to send command")
