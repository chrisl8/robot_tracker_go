package controller

import (
	"errors"
	"sync"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

type CommandQueue struct {
	controller       *ArduinoController
	commandCh        chan Command
	stopCh           chan struct{}
	running          bool
	interval         time.Duration // min spacing between writes of an unchanged command; see Enqueue
	heartbeatTimeout time.Duration
	lastCommand      Command
	lastSentTime     time.Time
	activeCommand    Command // currently desired command (re-sent each tick)
	hasActiveCommand bool    // whether activeCommand is set
	errorCount       int
	mu               sync.Mutex
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
	}
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
	q.mu.Unlock()

	go q.runLoop(commandCh, stopCh)
}

// halt marks the queue stopped and shuts down the run loop, reporting whether
// it was running. Once halted, sendCommand refuses anything but Stop, so a
// command still buffered in commandCh can't reach the robot after the final
// Stop.
func (q *CommandQueue) halt() bool {
	q.mu.Lock()
	wasRunning := q.running
	q.running = false
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
	if cmd == CommandStop {
		q.hasActiveCommand = false
	} else {
		q.activeCommand = cmd
		q.hasActiveCommand = true
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
			if q.controller.IsConnected() && time.Since(q.lastSentTime) > q.heartbeatTimeout {
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

func (q *CommandQueue) sendCommand(cmd Command) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.running && cmd != CommandStop {
		return
	}

	if err := q.controller.SendCommand(cmd); err != nil {
		q.errorCount++
		if q.errorCount == 1 || q.errorCount%100 == 0 {
			utils.Logf("Arduino send error (count=%d): %v", q.errorCount, err)
		}
	} else {
		if q.errorCount > 0 {
			utils.Logf("Arduino send recovered after %d errors", q.errorCount)
			q.errorCount = 0
		}
		q.lastCommand = cmd
		q.lastSentTime = time.Now()
	}
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
