package controller

import (
	"sync"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

type CommandQueue struct {
	controller       *ArduinoController
	commandCh        chan Command
	stopCh           chan struct{}
	running          bool
	interval         time.Duration
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

func (q *CommandQueue) Stop() {
	q.mu.Lock()
	if !q.running {
		q.mu.Unlock()
		return
	}
	q.running = false
	q.hasActiveCommand = false
	stopCh := q.stopCh
	q.mu.Unlock()

	close(stopCh)
}

func (q *CommandQueue) Enqueue(cmd Command) {
	q.mu.Lock()
	if cmd == CommandStop {
		q.hasActiveCommand = false
	} else {
		q.activeCommand = cmd
		q.hasActiveCommand = true
	}
	commandCh := q.commandCh
	q.mu.Unlock()

	select {
	case commandCh <- cmd:
	default:
	}
}

func (q *CommandQueue) EmergencyStop() {
	q.mu.Lock()
	q.hasActiveCommand = false
	q.mu.Unlock()

	// Send stop directly to controller, bypassing the queue to avoid
	// racing with channel close
	_ = q.controller.SendCommand(CommandStop)
	q.Stop()
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
