package controller

import (
	"sync"
	"time"

	"robot_tracker_go/internal/utils"
)

type CommandQueue struct {
	controller       *ArduinoController
	commandCh        chan Command
	stopCh           chan struct{}
	running          bool
	interval         time.Duration
	lastCommand      Command
	lastSentTime     time.Time
	activeCommand    Command // currently desired command (re-sent each tick)
	hasActiveCommand bool    // whether activeCommand is set
	errorCount       int
	mu               sync.Mutex
}

func NewCommandQueue(controller *ArduinoController, intervalMs int) *CommandQueue {
	interval := time.Duration(intervalMs) * time.Millisecond
	if interval == 0 {
		interval = CommandIntervalMs * time.Millisecond
	}
	return &CommandQueue{
		controller: controller,
		commandCh:  make(chan Command, 10),
		stopCh:     make(chan struct{}),
		interval:   interval,
	}
}

func (q *CommandQueue) Start() {
	if q.running {
		return
	}

	// Recreate channels so queue can restart after Stop
	q.commandCh = make(chan Command, 10)
	q.stopCh = make(chan struct{})
	q.running = true
	go q.runLoop()
}

func (q *CommandQueue) Stop() {
	if !q.running {
		return
	}

	q.mu.Lock()
	q.hasActiveCommand = false
	q.mu.Unlock()

	q.running = false
	close(q.stopCh)
}

func (q *CommandQueue) Enqueue(cmd Command) {
	q.mu.Lock()
	if cmd == CommandStop {
		q.hasActiveCommand = false
	} else {
		q.activeCommand = cmd
		q.hasActiveCommand = true
	}
	q.mu.Unlock()

	select {
	case q.commandCh <- cmd:
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

func (q *CommandQueue) runLoop() {
	ticker := time.NewTicker(q.interval)
	defer ticker.Stop()

	for {
		select {
		case cmd := <-q.commandCh:
			q.sendCommand(cmd)

		case <-ticker.C:
			if q.controller.IsConnected() && time.Since(q.lastSentTime) > HeartbeatTimeoutMs*time.Millisecond {
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

		case <-q.stopCh:
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
	return q.running
}
