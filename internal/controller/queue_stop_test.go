package controller

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
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

	incoming   []byte        // bytes the next Read hands out (what the gamepad "sent"); guarded by mu
	failReads  atomic.Bool   // every Read returns an error while set
	failWrites atomic.Bool   // every Write returns an error while set
	block      chan struct{} // if non-nil, Write blocks until it is closed
	closed     atomic.Bool
}

func (p *recordingPort) Write(b []byte) (int, error) {
	if p.failWrites.Load() {
		return 0, errors.New("simulated write failure")
	}
	if p.block != nil {
		<-p.block
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buf = append(p.buf, b...)
	return len(b), nil
}

// Read hands out queued incoming bytes, or after a short pause reports a read
// timeout (0, nil) like a real port; it fails once closed or failReads is set.
func (p *recordingPort) Read(b []byte) (int, error) {
	if p.closed.Load() || p.failReads.Load() {
		return 0, errors.New("simulated read failure")
	}
	p.mu.Lock()
	n := copy(b, p.incoming)
	p.incoming = p.incoming[n:]
	p.mu.Unlock()
	if n == 0 {
		time.Sleep(2 * time.Millisecond)
	}
	return n, nil
}

// feed queues bytes for the reader, as if the gamepad had printed them.
func (p *recordingPort) feed(s string) {
	p.mu.Lock()
	p.incoming = append(p.incoming, s...)
	p.mu.Unlock()
}

func (p *recordingPort) Close() error {
	p.closed.Store(true)
	return nil
}

func (p *recordingPort) SetReadTimeout(time.Duration) error { return nil }

// commands returns the command bytes written so far (each is "<cmd>\r\n").
func (p *recordingPort) commands() []Command {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Command, 0, len(p.buf)/3)
	for i := 0; i < len(p.buf); i += 3 {
		out = append(out, Command(p.buf[i]))
	}
	return out
}

// lastCommand returns the most recent command written, or 0 if none.
func (p *recordingPort) lastCommand() Command {
	cmds := p.commands()
	if len(cmds) == 0 {
		return 0
	}
	return cmds[len(cmds)-1]
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

		if got := port.lastCommand(); got != CommandStop {
			t.Fatalf("iteration %d: last command %q, want stop", i, got)
		}
	}
}

func TestCommandQueue_CanRestartAfterStop(t *testing.T) {
	q, port := newRecordingQueue()
	q.Start()
	q.Stop()
	q.Start()
	q.Enqueue(CommandForward)
	waitFor(t, func() bool { return port.lastCommand() == CommandForward })
	q.Stop()
}

func TestCommandQueue_HaltMotion_StopsRobotAndKeepsQueueRunning(t *testing.T) {
	q, port := newRecordingQueue()
	q.Start()
	defer q.Stop()
	q.Enqueue(CommandForward)
	waitFor(t, func() bool { return len(port.commands()) == 1 })

	q.HaltMotion()

	if got := port.lastCommand(); got != CommandStop {
		t.Errorf("last command %q, want stop", got)
	}
	if !q.IsRunning() {
		t.Error("queue should still be running after HaltMotion")
	}
	// The heartbeat must not resurrect the old Forward.
	q.mu.Lock()
	active := q.hasActiveCommand
	q.mu.Unlock()
	if active {
		t.Error("active command should be cleared")
	}

	// And it can be driven again afterwards.
	q.Enqueue(CommandLeft)
	waitFor(t, func() bool { return port.lastCommand() == CommandLeft })
}

func TestCommandQueue_Deadman_StopsRobotWhenCommandNotRefreshed(t *testing.T) {
	q, port := newRecordingQueue()
	q.SetCommandTTL(100 * time.Millisecond)
	q.Start()
	defer q.Stop()

	q.Enqueue(CommandForward)
	waitFor(t, func() bool { return port.lastCommand() == CommandForward })

	// Nobody refreshes it: the queue must stop the robot on its own.
	waitFor(t, func() bool { return port.lastCommand() == CommandStop })
}

func TestCommandQueue_Deadman_RefreshKeepsRobotMoving(t *testing.T) {
	q, port := newRecordingQueue()
	q.SetCommandTTL(150 * time.Millisecond)
	q.Start()
	defer q.Stop()

	// Refresh every 40 ms for well over the TTL, like a held key or a frame loop.
	end := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(end) {
		q.Enqueue(CommandForward)
		time.Sleep(40 * time.Millisecond)
		for _, c := range port.commands() {
			if c == CommandStop {
				t.Fatal("robot was stopped even though the command was being refreshed")
			}
		}
	}
}

func TestCommandQueue_Deadman_DisabledWithZeroTTL(t *testing.T) {
	q, port := newRecordingQueue()
	q.SetCommandTTL(0)
	q.Start()
	defer q.Stop()

	q.Enqueue(CommandForward)
	time.Sleep(300 * time.Millisecond)

	if got := port.lastCommand(); got != CommandForward {
		t.Errorf("last command %q, want forward (deadman disabled)", got)
	}
}

func TestNewCommandQueue_DeadmanOnByDefault(t *testing.T) {
	q := NewCommandQueue(nil, 0, 0)
	if q.commandTTL != DefaultCommandTTL || DefaultCommandTTL != 2*time.Second {
		t.Errorf("commandTTL = %v, want the 2s default", q.commandTTL)
	}
}

// A command that races an e-stop (a frame or handler that checked the e-stop
// just before it landed) must not be resumed by the heartbeat when control is
// restored.
func TestCommandQueue_LateEnqueueAfterEStop_NotResumedOnRestart(t *testing.T) {
	q, port := newRecordingQueue()
	q.Start()
	q.EmergencyStop()
	q.Enqueue(CommandForward) // late arrival on the halted queue

	q.Start() // ClearEmergencyStop restarts the queue
	defer q.Stop()
	time.Sleep(700 * time.Millisecond) // longer than the heartbeat

	for _, c := range port.commands() {
		if c == CommandForward {
			t.Fatalf("Forward reached the robot after the e-stop was cleared: %q", port.commands())
		}
	}
}

func TestCommandQueue_AutoReconnectOff_NeverOpensThePort(t *testing.T) {
	origOpen := openSerial
	opened := make(chan struct{}, 8)
	openSerial = func(string, *serial.Mode) (serial.Port, error) {
		opened <- struct{}{}
		return &recordingPort{}, nil
	}
	t.Cleanup(func() { openSerial = origOpen })

	ctrl := NewArduinoController("test", 0) // never connected
	q := NewCommandQueue(ctrl, 10, 0)
	q.reconnectInterval = 10 * time.Millisecond
	q.SetAutoReconnect(false)
	q.Start()
	defer q.Stop()
	time.Sleep(200 * time.Millisecond)

	select {
	case <-opened:
		t.Fatal("queue opened the serial port although auto-reconnect is off")
	default:
	}
}

func TestFilterArduinoPorts_NoFallbackToUnrelatedPorts(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"bluetooth and debug console only", []string{"/dev/cu.Bluetooth-Incoming-Port", "/dev/cu.debug-console", "/dev/tty"}, nil},
		{"usb serial", []string{"/dev/cu.Bluetooth-Incoming-Port", "/dev/cu.usbserial-BG01OQ2N"}, []string{"/dev/cu.usbserial-BG01OQ2N"}},
		{"linux acm", []string{"/dev/ttyS0", "/dev/ttyACM0"}, []string{"/dev/ttyACM0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterArduinoPorts(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// Writes while the board is still resetting after the port opens are lost, so
// the controller must not report connected until the startup delay has passed.
func TestArduinoController_NotConnectedDuringStartupDelay(t *testing.T) {
	origOpen, origDelay := openSerial, arduinoStartupDelay
	openSerial = func(string, *serial.Mode) (serial.Port, error) { return &recordingPort{}, nil }
	arduinoStartupDelay = 150 * time.Millisecond
	t.Cleanup(func() { openSerial, arduinoStartupDelay = origOpen, origDelay })

	ctrl := NewArduinoController("test", 0)
	done := make(chan error, 1)
	go func() { done <- ctrl.Connect() }()

	time.Sleep(50 * time.Millisecond)
	if ctrl.IsConnected() {
		t.Error("reported connected before the board finished resetting")
	}
	if err := ctrl.SendCommand(CommandStop); !errors.Is(err, ErrNotConnected) {
		t.Errorf("SendCommand during reset = %v, want ErrNotConnected", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !ctrl.IsConnected() {
		t.Error("not connected after Connect returned")
	}
}

func TestPathExecutor_Reset_ForgetsStaleBurst(t *testing.T) {
	e := NewPathExecutor()
	e.BurstFrames, e.MaxWaitFrames = 3, 3
	// 60° off: start a turn burst.
	e.BearingToCommand(0, 60*math.Pi/180, 0)
	if e.phase != phaseBursting {
		t.Fatalf("setup: phase = %v, want bursting", e.phase)
	}
	e.Reset()

	// Facing the target and stable: a fresh run drives forward at once instead
	// of finishing the old turn burst.
	if got := e.BearingToCommand(0, 0, 0); got != CommandForward {
		t.Errorf("first command after Reset = %q, want forward", got)
	}
}

// Autonomy drops a robot's path when an obstacle blocks the goal or a replan
// fails. ClearActiveCommand only stops the heartbeat re-sending the drive
// command: the robot keeps running on it until the next heartbeat writes Stop
// (measured ~380 ms), so the caller needs a stop that goes out at once.
func TestCommandQueue_StopIfActive_StopsRobotWithoutWaitingForHeartbeat(t *testing.T) {
	q, port := newRecordingQueue()
	q.Start()
	defer q.Stop()

	q.Enqueue(CommandForward)
	waitFor(t, func() bool { return len(port.commands()) >= 1 })
	time.Sleep(120 * time.Millisecond) // driving for a few frames
	before := len(port.commands())

	q.StopIfActive()
	time.Sleep(100 * time.Millisecond)

	got := port.commands()
	if len(got) <= before || got[len(got)-1] != CommandStop {
		t.Fatalf("Stop had not reached the robot 100 ms after the path was dropped: %q", got)
	}
	if q.HasActiveCommand() {
		t.Error("the drive command is still active after the stop")
	}
}

func TestCommandQueue_StopIfActive_IdleQueueWritesNothing(t *testing.T) {
	q, port := newRecordingQueue()
	q.Start()
	defer q.Stop()

	time.Sleep(120 * time.Millisecond) // let the heartbeat's first idle Stop go out
	before := len(port.commands())

	for i := 0; i < 20; i++ { // autonomy calls this every frame while idle
		q.StopIfActive()
		time.Sleep(5 * time.Millisecond)
	}
	if got := port.commands(); len(got) != before {
		t.Errorf("an idle queue wrote %q; StopIfActive must not spam Stop", got[before:])
	}
}

func TestCommandQueue_StopIfActive_HaltedQueueStaysHalted(t *testing.T) {
	q, port := newRecordingQueue()
	q.Start()
	q.EmergencyStop()
	n := len(port.commands())

	q.StopIfActive()
	time.Sleep(60 * time.Millisecond)
	if got := port.commands(); len(got) != n {
		t.Errorf("wrote after the e-stop: %q", got)
	}
}
