package controller

import (
	"sync"
	"time"
)

type CommandQueue struct {
	controller   *ArduinoController
	commandCh    chan Command
	stopCh       chan struct{}
	running      bool
	interval     time.Duration
	lastCommand  Command
	lastSentTime time.Time
	mu           sync.Mutex
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

	q.running = true
	go q.runLoop()
}

func (q *CommandQueue) Stop() {
	if !q.running {
		return
	}

	close(q.stopCh)
	q.running = false
}

func (q *CommandQueue) Enqueue(cmd Command) {
	select {
	case q.commandCh <- cmd:
	default:
	}
}

func (q *CommandQueue) EmergencyStop() {
	q.Enqueue(CommandStop)
	q.Stop()
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
				q.sendCommand(CommandStop)
			}

		case <-q.stopCh:
			return
		}
	}
}

func (q *CommandQueue) sendCommand(cmd Command) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if cmd == q.lastCommand && time.Since(q.lastSentTime) < q.interval {
		return
	}

	if err := q.controller.SendCommand(cmd); err == nil {
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
