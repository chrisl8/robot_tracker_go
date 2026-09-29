package controller

import (
	"testing"
	"time"

	"go.bug.st/serial"
)

// stubSerialOpen makes Connect() hand out the given ports in order (and skip
// the Arduino reset delay), restoring the real opener afterwards.
func stubSerialOpen(t *testing.T, ports ...*recordingPort) {
	t.Helper()
	origOpen, origDelay := openSerial, arduinoStartupDelay
	next := 0
	openSerial = func(string, *serial.Mode) (serial.Port, error) {
		if next >= len(ports) {
			return nil, ErrNotConnected
		}
		p := ports[next]
		next++
		return p, nil
	}
	arduinoStartupDelay = 0
	t.Cleanup(func() { openSerial, arduinoStartupDelay = origOpen, origDelay })
}

// A failed write used to only flip a flag, leaving the port handle open (so a
// reconnect to the same device would fail as busy) and control dead until the
// process restarted.
func TestArduino_WriteFailureClosesPortAndMarksDisconnected(t *testing.T) {
	port := &recordingPort{}
	port.failWrites.Store(true)
	ctrl := NewArduinoController("test", 0)
	ctrl.serial = port
	ctrl.connected = true

	err := ctrl.SendCommand(CommandForward)

	if err == nil {
		t.Fatal("expected an error from a failing write")
	}
	if ctrl.IsConnected() {
		t.Error("controller should be marked disconnected after a failed write")
	}
	if !port.closed.Load() {
		t.Error("the failed port must be closed, not leaked")
	}
}

func TestCommandQueue_ReconnectsAfterWriteFailure(t *testing.T) {
	q, first := newRecordingQueue()
	second := &recordingPort{}
	stubSerialOpen(t, second)
	q.reconnectInterval = 10 * time.Millisecond

	q.Start()
	defer q.Stop()

	first.failWrites.Store(true)
	q.Enqueue(CommandForward) // this write fails and drops the connection

	// The queue reopens the port on its own and starts by telling the robot to stop.
	waitFor(t, func() bool { return len(second.commands()) > 0 })
	cmds := second.commands()
	if len(cmds) == 0 || cmds[0] != CommandStop {
		t.Errorf("commands after reconnect = %q, want stop first", cmds)
	}
	if q.HasActiveCommand() {
		t.Error("a pre-disconnect movement command must not resume after reconnect")
	}
	if !first.closed.Load() {
		t.Error("the original failed port should have been closed")
	}

	// And control works again.
	q.Enqueue(CommandLeft)
	waitFor(t, func() bool { return second.lastCommand() == CommandLeft })
}

// The Arduino may simply not be plugged in at startup: the queue should keep
// retrying and connect when it appears.
func TestCommandQueue_ConnectsWhenArduinoAppearsLater(t *testing.T) {
	port := &recordingPort{}
	stubSerialOpen(t, port)
	ctrl := NewArduinoController("test", 0) // never connected
	q := NewCommandQueue(ctrl, 50, 0)
	q.reconnectInterval = 10 * time.Millisecond

	q.Start()
	defer q.Stop()

	waitFor(t, ctrl.IsConnected)
	waitFor(t, func() bool { return port.lastCommand() == CommandStop })
}

// A serial write that blocks must not freeze the callers of Enqueue,
// ClearActiveCommand, IsRunning etc. (the frame loop and HTTP handlers). The
// queue lock used to be held across the write.
func TestCommandQueue_BlockedWriteDoesNotFreezeCallers(t *testing.T) {
	q, port := newRecordingQueue()
	port.block = make(chan struct{})
	q.Start()
	defer func() {
		close(port.block)
		q.Stop()
	}()

	q.Enqueue(CommandForward) // run loop picks it up and blocks inside Write
	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		q.Enqueue(CommandLeft)
		q.ClearActiveCommand()
		_ = q.IsRunning()
		_ = q.HasActiveCommand()
		_ = q.GetLastCommand()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("callers blocked behind a stalled serial write")
	}
}
