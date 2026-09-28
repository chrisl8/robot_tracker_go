package controller

import (
	"sync"
	"testing"
	"time"

	"go.bug.st/serial"
)

// recordingPort captures every byte written; all other serial.Port methods
// are left nil (a call would panic, which is what we want in a test).
type recordingPort struct {
	serial.Port
	mu  sync.Mutex
	buf []byte
}

func (p *recordingPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buf = append(p.buf, b...)
	return len(b), nil
}

func (p *recordingPort) Close() error { return nil }

// commands returns the command bytes written so far (each is "<cmd>\r\n").
func (p *recordingPort) commands() []Command {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []Command
	for i := 0; i < len(p.buf); i += 3 {
		out = append(out, Command(p.buf[i]))
	}
	return out
}

func newRecordingQueue() (*CommandQueue, *recordingPort) {
	port := &recordingPort{}
	ctrl := NewArduinoController("test", 0)
	ctrl.serial = port
	ctrl.connected = true
	return NewCommandQueue(ctrl, 50, 0), port
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within 2s")
}

func TestCommandQueue_Stop_SendsStopToRobot(t *testing.T) {
	q, port := newRecordingQueue()
	q.Start()
	q.Enqueue(CommandForward)
	waitFor(t, func() bool { return len(port.commands()) == 1 })

	q.Stop()

	cmds := port.commands()
	if len(cmds) != 2 || cmds[0] != CommandForward || cmds[1] != CommandStop {
		t.Errorf("wrote %q, want forward then stop", cmds)
	}
}

func TestCommandQueue_Stop_NotRunningSendsNothing(t *testing.T) {
	q, port := newRecordingQueue()

	q.Stop()

	if cmds := port.commands(); len(cmds) != 0 {
		t.Errorf("Stop on a never-started queue wrote %q, want nothing", cmds)
	}
}

func TestCommandQueue_EmergencyStop_AlwaysSendsStop(t *testing.T) {
	q, port := newRecordingQueue() // not started

	q.EmergencyStop()

	cmds := port.commands()
	if len(cmds) != 1 || cmds[0] != CommandStop {
		t.Errorf("wrote %q, want a single stop", cmds)
	}
}

// A command sitting in commandCh when EmergencyStop runs must never reach the
// wire: the last byte the robot sees has to be Stop.
func TestCommandQueue_EmergencyStop_DropsBufferedCommands(t *testing.T) {
	for i := 0; i < 200; i++ {
		q, port := newRecordingQueue()
		q.Start()
		// Buffer several commands, then e-stop immediately.
		for j := 0; j < 5; j++ {
			q.Enqueue(CommandForward)
			q.Enqueue(CommandLeft)
		}
		q.EmergencyStop()
		time.Sleep(2 * time.Millisecond) // give the run loop a chance to misbehave

		cmds := port.commands()
		if len(cmds) == 0 || cmds[len(cmds)-1] != CommandStop {
			t.Fatalf("iteration %d: last command %q, want stop", i, cmds)
		}
	}
}

func TestCommandQueue_CanRestartAfterStop(t *testing.T) {
	q, port := newRecordingQueue()
	q.Start()
	q.Stop()
	q.Start()
	q.Enqueue(CommandForward)
	waitFor(t, func() bool {
		c := port.commands()
		return len(c) > 0 && c[len(c)-1] == CommandForward
	})
	q.Stop()
}
