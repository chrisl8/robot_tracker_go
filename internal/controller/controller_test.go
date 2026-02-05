package controller

import (
	"testing"
)

func TestSerialProtocol_EncodeCommand(t *testing.T) {
	p := NewSerialProtocol()

	tests := []struct {
		name     string
		cmd      Command
		expected []byte
	}{
		{"Forward", CommandForward, []byte{'F', '\r', '\n'}},
		{"Backward", CommandBackward, []byte{'B', '\r', '\n'}},
		{"Left", CommandLeft, []byte{'L', '\r', '\n'}},
		{"Right", CommandRight, []byte{'R', '\r', '\n'}},
		{"Weapon", CommandWeapon, []byte{'W', '\r', '\n'}},
		{"Stop", CommandStop, []byte{'S', '\r', '\n'}},
		{"Debug", CommandDebug, []byte{'d', '\r', '\n'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.EncodeCommand(tt.cmd)
			if len(result) != 3 {
				t.Errorf("EncodeCommand() returned %d bytes, want 3", len(result))
			}
			if result[0] != byte(tt.cmd) {
				t.Errorf("EncodeCommand()[0] = %c, want %c", result[0], tt.cmd)
			}
			if result[1] != '\r' || result[2] != '\n' {
				t.Errorf("EncodeCommand() should end with \\r\\n")
			}
		})
	}
}

func TestSerialProtocol_EncodeMode(t *testing.T) {
	p := NewSerialProtocol()

	tests := []struct {
		name     string
		mode     Mode
		expected byte
	}{
		{"Walk", ModeWalk, 'W'},
		{"Dance", ModeDance, 'D'},
		{"Fight", ModeFight, 'F'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.EncodeMode(tt.mode)
			if len(result) != 3 {
				t.Errorf("EncodeMode() returned %d bytes, want 3", len(result))
			}
			if result[0] != byte(tt.mode) {
				t.Errorf("EncodeMode() = %c, want %c", result[0], tt.mode)
			}
		})
	}
}

func TestSerialProtocol_EncodeSubmode(t *testing.T) {
	p := NewSerialProtocol()

	tests := []struct {
		name     string
		submode  Submode
		expected byte
	}{
		{"One", SubmodeOne, '1'},
		{"Two", SubmodeTwo, '2'},
		{"Three", SubmodeThree, '3'},
		{"Four", SubmodeFour, '4'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.EncodeSubmode(tt.submode)
			if len(result) != 3 {
				t.Errorf("EncodeSubmode() returned %d bytes, want 3", len(result))
			}
			if result[0] != byte(tt.submode) {
				t.Errorf("EncodeSubmode() = %c, want %c", result[0], tt.submode)
			}
		})
	}
}

func TestSerialProtocol_EncodeDebugToggle(t *testing.T) {
	p := NewSerialProtocol()
	result := p.EncodeDebugToggle()

	if len(result) != 3 {
		t.Errorf("EncodeDebugToggle() returned %d bytes, want 3", len(result))
	}
	if result[0] != byte(CommandDebug) {
		t.Errorf("EncodeDebugToggle() = %c, want %c", result[0], CommandDebug)
	}
}

func TestPathExecutor_VelocityToCommand(t *testing.T) {
	executor := NewPathExecutor(0.15, 1.0)

	tests := []struct {
		name string
		vx   float64
		vy   float64
		want Command
	}{
		{"Forward", 0.15, 0, CommandForward},
		{"Backward", -0.15, 0, CommandBackward},
		{"Left", 0, 1.0, CommandRight},
		{"Right", 0, -1.0, CommandLeft},
		{"Stop - Zero", 0, 0, CommandStop},
		{"Stop - Small X", 0.01, 0, CommandStop},
		{"Stop - Small Y", 0, 0.01, CommandStop},
		{"Forward - Diagonal", 0.1, 0.1, CommandForward},
		{"Backward - Diagonal", -0.1, -0.1, CommandBackward},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.VelocityToCommand(tt.vx, tt.vy)
			if got != tt.want {
				t.Errorf("VelocityToCommand(%f, %f) = %c, want %c", tt.vx, tt.vy, got, tt.want)
			}
		})
	}
}

func TestPathExecutor_CommandToVelocity(t *testing.T) {
	executor := NewPathExecutor(0.15, 1.0)

	tests := []struct {
		name string
		cmd  Command
		want Velocity
	}{
		{"Forward", CommandForward, Velocity{0.15, 0}},
		{"Backward", CommandBackward, Velocity{-0.15, 0}},
		{"Left", CommandLeft, Velocity{0, 1.0}},
		{"Right", CommandRight, Velocity{0, -1.0}},
		{"Stop", CommandStop, Velocity{0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.CommandToVelocity(tt.cmd)
			if got.VX != tt.want.VX || got.VY != tt.want.VY {
				t.Errorf("CommandToVelocity(%c) = (%f, %f), want (%f, %f)",
					tt.cmd, got.VX, got.VY, tt.want.VX, tt.want.VY)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	if BaudRate != 9600 {
		t.Errorf("BaudRate = %d, want 9600", BaudRate)
	}
	if CommandIntervalMs != 50 {
		t.Errorf("CommandIntervalMs = %d, want 50", CommandIntervalMs)
	}
	if HeartbeatTimeoutMs != 500 {
		t.Errorf("HeartbeatTimeoutMs = %d, want 500", HeartbeatTimeoutMs)
	}
}
