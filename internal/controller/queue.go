package controller

import (
	"errors"
	"sync"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

type CommandQueue struct {
	controller *ArduinoController
	commandCh  chan Command
	stopCh     chan struct{}
	running    bool
	// halted is set by halt() and cleared by Start(). While set, Enqueue drops
	// movement commands: a late caller (a frame or HTTP handler that checked
	// the e-stop just before it landed) must not leave a stale active command
	// that the heartbeat would re-send the moment control is restored.
	halted           bool
	interval         time.Duration // min spacing between writes of an unchanged command; see Enqueue
	heartbeatTimeout time.Duration
	lastCommand      Command
	lastSentTime     time.Time
	activeCommand    Command // currently desired command (re-sent each tick)
	hasActiveCommand bool    // whether activeCommand is set
	activeSince      time.Time
	commandTTL       time.Duration // 0 disables; see SetCommandTTL
	errorCount       int
	mu               sync.Mutex

	// sendMu serializes "check running, then write to the controller" against
	// halt(), so a write already in flight can never land after the final
	// Stop. It is separate from mu so that a serial write that blocks does not
	// also freeze Enqueue, ClearActiveCommand and the callers that use them.
	sendMu sync.Mutex

	reconnectInterval    time.Duration
	autoReconnect        bool      // guarded by mu; see SetAutoReconnect
	lastReconnectAttempt time.Time // run-loop goroutine only
	reconnectFailures    int       // run-loop goroutine only
}

// NewCommandQueue builds a CommandQueue that re-sends the active (or stop)
// command at least every heartbeatTimeoutMs, so the Arduino firmware's own
// watchdog never sees silence while a track is being followed. intervalMs
// and heartbeatTimeoutMs of 0 (or negative) fall back to CommandIntervalMs
// and HeartbeatTimeoutMs respectively.
func NewCommandQueue(controller *ArduinoController, intervalMs int, heartbeatTimeoutMs int) *CommandQueue {
	interval := time.Duration(intervalMs) * time.Millisecond
	if interval == 0 {
		interval = CommandIntervalMs * time.Millisecond
	}
	heartbeatTimeout := time.Duration(heartbeatTimeoutMs) * time.Millisecond
	if heartbeatTimeout <= 0 {
		heartbeatTimeout = HeartbeatTimeoutMs * time.Millisecond
	}
	return &CommandQueue{
		controller:       controller,
		commandCh:        make(chan Command, 10),
		stopCh:           make(chan struct{}),
		interval:         interval,
		heartbeatTimeout: heartbeatTimeout,
		commandTTL:       DefaultCommandTTL,

		reconnectInterval: DefaultReconnectInterval,
		autoReconnect:     true,
	}
}

// SetAutoReconnect controls whether the run loop reopens the serial port while
// the controller is disconnected. It is on by default; the app turns it off
// when the controller is disabled in config, so a disabled controller is never
// connected behind the operator's back.
func (q *CommandQueue) SetAutoReconnect(on bool) {
	q.mu.Lock()
	q.autoReconnect = on
	q.mu.Unlock()
}

// DefaultReconnectInterval is how often the run loop retries opening the
// serial port while the Arduino is disconnected (unplugged, or a write failed).
const DefaultReconnectInterval = 2 * time.Second

// DefaultCommandTTL is how long a movement command stays active without being
// re-Enqueued. The heartbeat re-sends the active command to the robot on its
// own, so without this a caller that disappears (browser closed, network
// dropped, camera frozen) would leave the robot driving indefinitely. Real
// callers refresh well inside it: the UI re-sends a held key every 250 ms and
// the autonomous loop enqueues every frame.
const DefaultCommandTTL = 2 * time.Second

// SetCommandTTL overrides the active-command deadman; 0 disables it.
func (q *CommandQueue) SetCommandTTL(d time.Duration) {
	q.mu.Lock()
	q.commandTTL = d
	q.mu.Unlock()
}

func (q *CommandQueue) Start() {
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return
	}

	// Recreate channels so queue can restart after Stop
	commandCh := make(chan Command, 10)
	stopCh := make(chan struct{})
	q.commandCh = commandCh
	q.stopCh = stopCh
	q.running = true
	q.halted = false
	// Anything still marked active was left by a caller that raced the halt;
	// the robot's state is unknown, so callers must re-issue movement.
	q.hasActiveCommand = false
	q.mu.Unlock()

	go q.runLoop(commandCh, stopCh)
}

// halt marks the queue stopped and shuts down the run loop, reporting whether
// it was running. Once halted, sendCommand refuses anything but Stop, so a
// command still buffered in commandCh can't reach the robot after the final
// Stop.
func (q *CommandQueue) halt() bool {
	// Held for the whole method: this waits out a write already in flight, and
	// keeps sendCommand from starting another until running is false, so the
	// caller's final Stop can't be overtaken by a movement command.
	q.sendMu.Lock()
	defer q.sendMu.Unlock()

	q.mu.Lock()
	wasRunning := q.running
	q.running = false
	q.halted = true
	q.hasActiveCommand = false
	stopCh := q.stopCh
	q.mu.Unlock()

	if wasRunning {
		close(stopCh)
	}
	return wasRunning
}

// sendStopDirect writes Stop straight to the controller, bypassing the queue.
func (q *CommandQueue) sendStopDirect() {
	if q.controller == nil {
		return
	}
	if err := q.controller.SendCommand(CommandStop); err != nil && !errors.Is(err, ErrNotConnected) {
		utils.Logf("Arduino stop send error: %v", err)
	}
}

// Stop shuts the queue down and tells the robot to stop, so shutting down
// mid-drive doesn't leave it running on its last command.
func (q *CommandQueue) Stop() {
	if q.halt() {
		q.sendStopDirect()
	}
}

// Enqueue asks the queue to send cmd. The active/heartbeat bookkeeping is
// updated unconditionally so the heartbeat ticker keeps re-sending the
// latest desired command, but the actual write to the controller is
// rate-limited: a repeat of the same command arriving less than `interval`
// after the last write is dropped rather than written immediately, so a
// fast caller (e.g. a per-frame autonomous steering loop) can't burst
// duplicate writes to the serial line faster than CommandIntervalMs. A
// different command, or enough elapsed time, is always sent right away —
// only redundant repeats of an unchanged command are throttled.
func (q *CommandQueue) Enqueue(cmd Command) {
	q.mu.Lock()
	if q.halted && cmd != CommandStop {
		q.mu.Unlock()
		return
	}
	if cmd == CommandStop {
		q.hasActiveCommand = false
	} else {
		q.activeCommand = cmd
		q.hasActiveCommand = true
		q.activeSince = time.Now() // refreshed even when the write below is rate-limited
	}
	if cmd == q.lastCommand && time.Since(q.lastSentTime) < q.interval {
		q.mu.Unlock()
		return
	}
	commandCh := q.commandCh
	q.mu.Unlock()

	select {
	case commandCh <- cmd:
	default:
	}
}

// EmergencyStop halts the queue and sends Stop whether or not the queue was
// running.
func (q *CommandQueue) EmergencyStop() {
	q.halt()
	q.sendStopDirect()
}

// HaltMotion clears the active command and sends Stop immediately, but leaves
// the queue running so control can resume afterwards. Used when the robot must
// not keep moving on its last command, e.g. the camera has stalled.
func (q *CommandQueue) HaltMotion() {
	q.ClearActiveCommand()
	q.sendStopDirect()
}

// ClearActiveCommand clears the active command without stopping the queue.
// Used on mode transitions to stop re-sending movement commands.
func (q *CommandQueue) ClearActiveCommand() {
	q.mu.Lock()
	q.hasActiveCommand = false
	q.mu.Unlock()
}

func (q *CommandQueue) runLoop(commandCh chan Command, stopCh chan struct{}) {
	ticker := time.NewTicker(q.interval)
	defer ticker.Stop()

	for {
		select {
		case cmd := <-commandCh:
			q.sendCommand(cmd)

		case <-ticker.C:
			if q.expireStaleCommand() {
				q.sendCommand(CommandStop)
				continue
			}
			if !q.controller.IsConnected() {
				if q.reconnectEnabled() {
					q.tryReconnect()
				}
				continue
			}
			if time.Since(q.lastSentTime) > q.heartbeatTimeout {
				q.mu.Lock()
				active := q.hasActiveCommand
				cmd := q.activeCommand
				q.mu.Unlock()
				if active {
					q.sendCommand(cmd)
				} else {
					q.sendCommand(CommandStop)
				}
			}

		case <-stopCh:
			return
		}
	}
}

func (q *CommandQueue) reconnectEnabled() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.autoReconnect
}

// expireStaleCommand drops the active command if nobody has refreshed it
// within commandTTL, reporting whether it did (the caller then sends Stop).
func (q *CommandQueue) expireStaleCommand() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.hasActiveCommand || q.commandTTL <= 0 || time.Since(q.activeSince) <= q.commandTTL {
		return false
	}
	q.hasActiveCommand = false
	utils.Logf("Command %q not refreshed for %v: stopping robot", q.activeCommand, q.commandTTL)
	return true
}

func (q *CommandQueue) sendCommand(cmd Command) {
	q.sendMu.Lock()
	defer q.sendMu.Unlock()

	q.mu.Lock()
	allowed := q.running || cmd == CommandStop
	q.mu.Unlock()
	if !allowed {
		return
	}

	err := q.controller.SendCommand(cmd) // may block; q.mu is not held

	q.mu.Lock()
	defer q.mu.Unlock()
	if err != nil {
		q.errorCount++
		if q.errorCount == 1 || q.errorCount%100 == 0 {
			utils.Logf("Arduino send error (count=%d): %v", q.errorCount, err)
		}
		return
	}
	if q.errorCount > 0 {
		utils.Logf("Arduino send recovered after %d errors", q.errorCount)
		q.errorCount = 0
	}
	q.lastCommand = cmd
	q.lastSentTime = time.Now()
}

// tryReconnect reopens the serial port after a failed write, or if the Arduino
// was never connected, at most once per reconnectInterval. The robot's state
// is unknown after a reconnect (opening the port resets most Arduinos), so it
// is told to Stop and any stale active command is dropped rather than
// resumed; callers re-issue movement.
func (q *CommandQueue) tryReconnect() {
	if time.Since(q.lastReconnectAttempt) < q.reconnectInterval {
		return
	}
	q.lastReconnectAttempt = time.Now()

	if err := q.controller.Connect(); err != nil {
		q.reconnectFailures++
		if q.reconnectFailures == 1 || q.reconnectFailures%15 == 0 {
			utils.Logf("Arduino reconnect failed (attempt %d): %v", q.reconnectFailures, err)
		}
		return
	}
	utils.Logf("Arduino connected (after %d failed attempts)", q.reconnectFailures)
	q.reconnectFailures = 0

	q.ClearActiveCommand()
	q.sendCommand(CommandStop)
}

// HasActiveCommand reports whether a movement command is currently being
// held (and re-sent by the heartbeat).
func (q *CommandQueue) HasActiveCommand() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.hasActiveCommand
}

func (q *CommandQueue) GetLastCommand() Command {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.lastCommand
}

func (q *CommandQueue) IsRunning() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.running
}
