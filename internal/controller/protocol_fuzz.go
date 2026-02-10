//go:build go1.18

package controller

import (
	"testing"
)

func FuzzDecode(f *testing.F) {
	f.Add([]byte("F\r\n"))
	f.Add([]byte("B\r\n"))
	f.Add([]byte("L\r\n"))
	f.Add([]byte("R\r\n"))
	f.Add([]byte("W\r\n"))
	f.Add([]byte("S\r\n"))
	f.Add([]byte("?\r\n"))
	f.Add([]byte("d\r\n"))
	f.Add([]byte{})
	f.Add([]byte("X"))
	f.Add([]byte{0x00})
	f.Add([]byte{0xFF})
	f.Add([]byte{0x7F})
	f.Add([]byte("ABC"))
	f.Add([]byte("\r\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := NewSerialProtocol().Decode(data)

		if len(data) >= 1 {
			cmd := Command(data[0])
			switch cmd {
			case CommandForward, CommandBackward, CommandLeft, CommandRight,
				CommandWeapon, CommandStop, CommandQuery, CommandDebug:
				if err != nil {
					t.Errorf("Decode(%v) returned error %v for valid command %c", data, err, data[0])
				}
			default:
				if len(data) > 0 && err == nil {
					t.Errorf("Decode(%v) = nil, want error for invalid command byte 0x%02x", data, data[0])
				}
			}
		}

		if len(data) == 0 && err == nil {
			t.Error("Decode([]byte{}) should error for empty input")
		}

		if err != nil && err != ErrInvalidData {
			t.Errorf("Decode(%v) returned unexpected error %v", data, err)
		}
	})
}

func FuzzEncodeCommand(f *testing.F) {
	f.Add(CommandForward)
	f.Add(CommandBackward)
	f.Add(CommandLeft)
	f.Add(CommandRight)
	f.Add(CommandStop)
	f.Add(CommandWeapon)
	f.Add(CommandQuery)
	f.Add(CommandDebug)

	f.Fuzz(func(t *testing.T, cmd Command) {
		result := NewSerialProtocol().EncodeCommand(cmd)

		if len(result) != 3 {
			t.Errorf("EncodeCommand(%c) returned %d bytes, want 3", cmd, len(result))
		}

		if result[0] != byte(cmd) {
			t.Errorf("EncodeCommand(%c)[0] = %c, want %c", cmd, result[0], cmd)
		}

		if result[1] != '\r' || result[2] != '\n' {
			t.Errorf("EncodeCommand(%c) should end with \\r\\n, got %v", cmd, result)
		}
	})
}
