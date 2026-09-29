package controller

import (
	"math"
	"testing"
	"time"
)

// The wire format is the command byte followed by CRLF. SendCommand writes
// exactly this, so this pins what the Arduino firmware receives.
func TestEncodeCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  Command
		want string
	}{
		{"forward", CommandForward, "f\r\n"},
		{"backward", CommandBackward, "b\r\n"},
		{"left", CommandLeft, "l\r\n"},
		{"right", CommandRight, "r\r\n"},
		{"stop", CommandStop, "s\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(encodeCommand(tt.cmd)); got != tt.want {
				t.Errorf("encodeCommand(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestPathExecutor_BearingToCommand(t *testing.T) {
	deg := math.Pi / 180

	tests := []struct {
		name         string
		heading      float64
		bearing      float64
		headingDelta float64
		want         Command
	}{
		// Rule 1: aligned and stable → Forward
		{"Aligned stable", 0, 10 * deg, 0, CommandForward},
		{"Aligned stable negative", 0, -15 * deg, 0, CommandForward},
		// Rule 2: spinning toward target → Forward (aligned enough to continue)
		{"Spinning toward close", 51 * deg, 52 * deg, 35 * deg, CommandForward},
		{"Spinning toward within brake zone", 30 * deg, 51 * deg, 15 * deg, CommandForward},
		// Rule 3: facing away → Backward
		{"Rear facing positive", 0, 170 * deg, 0, CommandBackward},
		{"Rear facing negative", 0, -170 * deg, 0, CommandBackward},
		{"Rear facing spinning", 0, 175 * deg, 20 * deg, CommandBackward},
		{"Rear facing exactly behind", 0, 180 * deg, 0, CommandBackward},
		// Forward at moderate angle (within 60° exit threshold, below 30° nudge threshold)
		{"Forward at moderate angle", 0, 25 * deg, 0, CommandForward},
		// Rule 4: normal turn
		{"Turn right", 0, 60 * deg, 0, CommandRight},
		{"Turn left", 0, -60 * deg, 0, CommandLeft},
		// Aligned but spinning → spin suppressed in forward mode (isTurning=false) → Forward
		{"Aligned but spinning fast", 0, 5 * deg, 15 * deg, CommandForward},
		// Zero delta backward compat: same as old behavior for small angles
		{"Zero delta forward", 45 * deg, 50 * deg, 0, CommandForward},
		{"Zero delta turn", 0, 90 * deg, 0, CommandRight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewPathExecutor()
			got := executor.BearingToCommand(tt.heading, tt.bearing, tt.headingDelta)
			if got != tt.want {
				t.Errorf("BearingToCommand(heading=%.1f°, bearing=%.1f°, delta=%.1f°) = %c, want %c",
					tt.heading/deg, tt.bearing/deg, tt.headingDelta/deg, got, tt.want)
			}
		})
	}
}

func TestBearingToCommand_SpinInForwardMode(t *testing.T) {
	deg := math.Pi / 180

	t.Run("spin suppressed while driving forward", func(t *testing.T) {
		executor := NewPathExecutor()

		// First call: small bearing → enters forward mode (isTurning=false)
		cmd := executor.BearingToCommand(0, 10*deg, 0)
		if cmd != CommandForward {
			t.Fatalf("setup: got %c, want %c (Forward)", cmd, CommandForward)
		}

		// Second call: large heading delta (25°) simulating AprilTag noise spike
		// Should stay Forward because spin detection is suppressed in forward mode
		cmd = executor.BearingToCommand(0, 10*deg, 25*deg)
		if cmd != CommandForward {
			t.Fatalf("spin in forward mode: got %c, want %c (Forward)", cmd, CommandForward)
		}
	})

	t.Run("spin active during turning", func(t *testing.T) {
		executor := NewPathExecutor()

		// Enter turn mode: large angle → burst
		cmd := executor.BearingToCommand(0, 80*deg, 0)
		if cmd != CommandRight {
			t.Fatalf("setup: got %c, want %c (Right)", cmd, CommandRight)
		}

		// Exhaust burst and wait to re-evaluate
		for i := 0; i < 10; i++ {
			executor.BearingToCommand(0, 80*deg, 0)
		}

		// Now in turn mode with large angle + spin → should NOT be Forward
		cmd = executor.BearingToCommand(0, 80*deg, 25*deg)
		if cmd == CommandForward {
			t.Fatalf("spin during turning: got Forward, expected turn or stop")
		}
	})
}

func TestBearingToCommand_TurnPulse(t *testing.T) {
	deg := math.Pi / 180

	t.Run("burst then wait then re-evaluate", func(t *testing.T) {
		executor := NewPathExecutor()

		// Frames 1-3: burst of 3 turn commands
		for i := 1; i <= 3; i++ {
			cmd := executor.BearingToCommand(0, 80*deg, 0)
			if cmd != CommandRight {
				t.Fatalf("frame %d (burst): got %c, want %c (Right)", i, cmd, CommandRight)
			}
		}

		// Frame 4: burst done, heading barely changed → drive forward while waiting
		// (80° < 90° threshold, so Forward instead of Stop)
		cmd := executor.BearingToCommand(0.5*deg, 80*deg, 0)
		if cmd != CommandForward {
			t.Fatalf("frame 4: got %c, want %c (Forward/waiting)", cmd, CommandForward)
		}

		// Frame 5: heading jumped >2° from wait heading → should re-evaluate and start new burst
		cmd = executor.BearingToCommand(3*deg, 80*deg, 0)
		if cmd != CommandRight {
			t.Fatalf("frame 5: got %c, want %c (Right after heading update)", cmd, CommandRight)
		}
	})

	t.Run("timeout exits wait mode", func(t *testing.T) {
		executor := NewPathExecutor()

		// Frames 1-3: burst of 3 turn commands
		for i := 1; i <= 3; i++ {
			cmd := executor.BearingToCommand(0, 80*deg, 0)
			if cmd != CommandRight {
				t.Fatalf("frame %d (burst): got %c, want %c (Right)", i, cmd, CommandRight)
			}
		}

		// Frames 4-6: heading stuck, drive forward while waiting for 3 frames then timeout
		// (80° < 90° threshold, so Forward instead of Stop)
		for i := 4; i <= 6; i++ {
			cmd := executor.BearingToCommand(0, 80*deg, 0)
			if cmd != CommandForward {
				t.Fatalf("frame %d: got %c, want %c (Forward/waiting)", i, cmd, CommandForward)
			}
		}

		// Frame 7: timeout (>3 wait frames) → should re-evaluate and start new burst
		cmd := executor.BearingToCommand(0, 80*deg, 0)
		if cmd != CommandRight {
			t.Fatalf("frame 7 (after timeout): got %c, want %c (Right)", cmd, CommandRight)
		}
	})

	t.Run("burst continuation sends same command", func(t *testing.T) {
		executor := NewPathExecutor()

		// Frame 1: start burst turning left
		cmd := executor.BearingToCommand(0, -80*deg, 0)
		if cmd != CommandLeft {
			t.Fatalf("frame 1: got %c, want %c (Left)", cmd, CommandLeft)
		}

		// Frames 2-3: burst continues with same command regardless of heading changes
		for i := 2; i <= 3; i++ {
			cmd = executor.BearingToCommand(float64(i)*deg, -80*deg, 0)
			if cmd != CommandLeft {
				t.Fatalf("frame %d (burst): got %c, want %c (Left)", i, cmd, CommandLeft)
			}
		}
	})

	t.Run("no wait for non-turn commands", func(t *testing.T) {
		executor := NewPathExecutor()

		// Forward command should not enter wait mode
		cmd := executor.BearingToCommand(0, 10*deg, 0)
		if cmd != CommandForward {
			t.Fatalf("frame 1: got %c, want %c (Forward)", cmd, CommandForward)
		}

		// Next frame: still forward, no waiting
		cmd = executor.BearingToCommand(0, 10*deg, 0)
		if cmd != CommandForward {
			t.Fatalf("frame 2: got %c, want %c (Forward)", cmd, CommandForward)
		}
	})
}

func TestBearingToCommand_Hysteresis(t *testing.T) {
	deg := math.Pi / 180

	t.Run("stays forward despite noise crossing entry threshold", func(t *testing.T) {
		executor := NewPathExecutor()
		executor.ForwardThresholdDeg = 25.0 // enter=25°, exit=35° (25+10)

		// Start aligned → forward (enters forward mode, isTurning=false)
		cmd := executor.BearingToCommand(0, 10*deg, 0)
		if cmd != CommandForward {
			t.Fatalf("frame 1: got %c, want Forward", cmd)
		}

		// Heading drifts to 20° — nudge correction (above 17.5° nudge threshold),
		// but stays in forward mode (isTurning=false, hysteresis still active).
		cmd = executor.BearingToCommand(0, 20*deg, 0)
		if cmd != CommandRight {
			t.Fatalf("frame 2 (20° diff, nudge correction): got %c, want Right (nudge)", cmd)
		}

		// Heading drifts to 30° — forward (nudge cooldown active, hysteresis keeps forward mode)
		cmd = executor.BearingToCommand(0, 30*deg, 0)
		if cmd != CommandForward {
			t.Fatalf("frame 3 (30° diff, nudge cooldown): got %c, want Forward", cmd)
		}

		// Heading drifts to 40° — exceeds exit threshold (35°), should start turning
		cmd = executor.BearingToCommand(0, 40*deg, 0)
		if cmd == CommandForward {
			t.Fatalf("frame 4 (40° diff, beyond exit threshold): got Forward, want turn")
		}
	})

	t.Run("must align tightly before re-entering forward", func(t *testing.T) {
		executor := NewPathExecutor()
		executor.ForwardThresholdDeg = 25.0 // enter=25°, exit=35° (25+10)

		// Start with large angle → turn (enters turning mode)
		cmd := executor.BearingToCommand(0, 60*deg, 0)
		if cmd != CommandRight {
			t.Fatalf("frame 1: got %c, want Right", cmd)
		}

		// Skip burst frames (default 3)
		for range 2 {
			executor.BearingToCommand(0, 60*deg, 0) // burst
		}
		for range 4 {
			executor.BearingToCommand(0, 60*deg, 0) // wait + timeout
		}

		// Now at 30° diff — below exit threshold (35°) but above entry (25°).
		// Should NOT go forward yet (still in turning mode, needs <25° to enter forward).
		cmd = executor.BearingToCommand(0, 30*deg, 0)
		if cmd == CommandForward {
			t.Fatalf("30° diff after turning: got Forward, want turn (entry threshold is 25°)")
		}

		// Skip burst + wait from the 30° turn, feeding 20° so the re-evaluation
		// after wait timeout sees 20° (below 25° entry) and enters forward.
		for range 2 {
			executor.BearingToCommand(0, 20*deg, 0) // burst
		}
		for range 4 {
			executor.BearingToCommand(0, 20*deg, 0) // wait + timeout → re-evaluates at 20° → Forward
		}

		// Confirm we're now in forward mode at 20°
		cmd = executor.BearingToCommand(0, 20*deg, 0)
		if cmd != CommandForward {
			t.Fatalf("20° diff after turning: got %c, want Forward (below 25° entry)", cmd)
		}
	})
}

func TestBearingToCommand_ContinuousTurn(t *testing.T) {
	deg := math.Pi / 180

	t.Run("large angle sends continuous turn without burst/wait", func(t *testing.T) {
		executor := NewPathExecutor()
		executor.ForwardThresholdDeg = 25.0 // exit=35°, continuous=52.5°

		// 80° off target — above 52.5° continuous threshold, below 135° rear threshold
		// Every frame should return a turn command (no stops for burst/wait)
		for i := 1; i <= 5; i++ {
			cmd := executor.BearingToCommand(0, 80*deg, 0)
			if cmd != CommandRight {
				t.Fatalf("frame %d: got %c, want Right (continuous turn)", i, cmd)
			}
		}
	})

	t.Run("continuous turn left", func(t *testing.T) {
		executor := NewPathExecutor()
		executor.ForwardThresholdDeg = 25.0

		for i := 1; i <= 5; i++ {
			cmd := executor.BearingToCommand(0, -80*deg, 0)
			if cmd != CommandLeft {
				t.Fatalf("frame %d: got %c, want Left (continuous turn)", i, cmd)
			}
		}
	})

	t.Run("transitions from continuous to burst/wait as angle decreases", func(t *testing.T) {
		executor := NewPathExecutor()
		executor.ForwardThresholdDeg = 25.0 // exit=40°, continuous=60°

		// Start with large angle — continuous turn
		cmd := executor.BearingToCommand(0, 80*deg, 0)
		if cmd != CommandRight {
			t.Fatalf("frame 1 (80°): got %c, want Right", cmd)
		}

		// Angle drops to 50° — below 60° continuous threshold, enters burst/wait
		cmd = executor.BearingToCommand(0, 50*deg, 0)
		if cmd != CommandRight {
			t.Fatalf("frame 2 (50°): got %c, want Right (first frame of burst)", cmd)
		}

		// With default BurstFrames=3, two more burst frames remain
		cmd = executor.BearingToCommand(0, 50*deg, 0)
		if cmd != CommandRight {
			t.Fatalf("frame 3 (50°, burst): got %c, want Right", cmd)
		}
		cmd = executor.BearingToCommand(0, 50*deg, 0)
		if cmd != CommandRight {
			t.Fatalf("frame 4 (50°, burst): got %c, want Right", cmd)
		}

		// Now in wait phase — drives forward (50° < 90° threshold)
		cmd = executor.BearingToCommand(0, 50*deg, 0)
		if cmd != CommandForward {
			t.Fatalf("frame 5 (50°, wait): got %c, want Forward", cmd)
		}
	})

	t.Run("spin detection blocks continuous turn but allows burst turn", func(t *testing.T) {
		executor := NewPathExecutor()
		executor.ForwardThresholdDeg = 25.0

		// First put executor into turning mode with a large angle (continuous turn)
		cmd1 := executor.BearingToCommand(0, 80*deg, 0)
		if cmd1 != CommandRight {
			t.Fatalf("frame 1: got %c, want Right (continuous turn)", cmd1)
		}

		// With high delta while already turning, continuous turn is blocked by
		// spin detection, but the default case still issues a burst turn command
		cmd2 := executor.BearingToCommand(0, 80*deg, 25*deg)
		if cmd2 != CommandRight {
			t.Fatalf("frame 2: got %c, want Right (burst turn despite spinning)", cmd2)
		}
	})
}

func TestBearingToCommand_Nudge(t *testing.T) {
	deg := math.Pi / 180

	t.Run("nudge right when drifting past threshold", func(t *testing.T) {
		executor := NewPathExecutor()
		executor.ForwardThresholdDeg = 25.0 // exit=40°, nudge=20°

		// Start aligned → forward (enters forward mode)
		cmd := executor.BearingToCommand(0, 10*deg, 0)
		if cmd != CommandForward {
			t.Fatalf("frame 1 (10°): got %c, want Forward", cmd)
		}

		// Drift to 22° → should nudge right (above 20° nudge threshold)
		cmd = executor.BearingToCommand(0, 22*deg, 0)
		if cmd != CommandRight {
			t.Fatalf("frame 2 (22°): got %c, want Right (nudge)", cmd)
		}

		// Next frame: cooldown active → forward
		cmd = executor.BearingToCommand(0, 22*deg, 0)
		if cmd != CommandForward {
			t.Fatalf("frame 3 (22°, cooldown): got %c, want Forward", cmd)
		}
	})

	t.Run("nudge left when drifting negative", func(t *testing.T) {
		executor := NewPathExecutor()
		executor.ForwardThresholdDeg = 25.0

		// Start forward
		executor.BearingToCommand(0, 5*deg, 0)

		// Drift to -22° → nudge left
		cmd := executor.BearingToCommand(0, -22*deg, 0)
		if cmd != CommandLeft {
			t.Fatalf("got %c, want Left (nudge)", cmd)
		}
	})

	t.Run("cooldown prevents rapid nudging", func(t *testing.T) {
		executor := NewPathExecutor()
		executor.ForwardThresholdDeg = 25.0

		// Enter forward mode
		executor.BearingToCommand(0, 5*deg, 0)

		// Trigger nudge (22° > nudge threshold of 17.5°)
		cmd := executor.BearingToCommand(0, 22*deg, 0)
		if cmd != CommandRight {
			t.Fatalf("nudge frame: got %c, want Right", cmd)
		}

		// Next 3 frames should be Forward (cooldown=3)
		for i := 1; i <= 3; i++ {
			cmd = executor.BearingToCommand(0, 22*deg, 0)
			if cmd != CommandForward {
				t.Fatalf("cooldown frame %d: got %c, want Forward", i, cmd)
			}
		}

		// Frame 5: cooldown expired → nudge again
		cmd = executor.BearingToCommand(0, 22*deg, 0)
		if cmd != CommandRight {
			t.Fatalf("after cooldown: got %c, want Right (nudge)", cmd)
		}
	})

	t.Run("no nudge below threshold", func(t *testing.T) {
		executor := NewPathExecutor()
		executor.ForwardThresholdDeg = 25.0 // nudge=17.5°

		// Enter forward mode
		executor.BearingToCommand(0, 5*deg, 0)

		// 15° is below nudge threshold (17.5°) → always Forward
		for i := range 10 {
			cmd := executor.BearingToCommand(0, 15*deg, 0)
			if cmd != CommandForward {
				t.Fatalf("frame %d (15°): got %c, want Forward", i, cmd)
			}
		}
	})

	t.Run("stays in forward mode within hysteresis band", func(t *testing.T) {
		executor := NewPathExecutor()
		executor.ForwardThresholdDeg = 25.0

		// Enter forward mode with well-aligned heading
		executor.BearingToCommand(0, 5*deg, 0)

		// Still in forward mode at 15° (below nudge threshold of 17.5° and exit threshold of 35°)
		cmd := executor.BearingToCommand(0, 15*deg, 0)
		if cmd != CommandForward {
			t.Fatalf("got %c, want Forward (within hysteresis band)", cmd)
		}
	})
}

func TestNewCommandQueue_HeartbeatTimeoutDefault(t *testing.T) {
	q := NewCommandQueue(nil, 0, 0)
	want := time.Duration(HeartbeatTimeoutMs) * time.Millisecond
	if q.heartbeatTimeout != want {
		t.Errorf("heartbeatTimeout = %v, want default %v", q.heartbeatTimeout, want)
	}
}

func TestNewCommandQueue_HeartbeatTimeoutOverride(t *testing.T) {
	q := NewCommandQueue(nil, 0, 250)
	want := 250 * time.Millisecond
	if q.heartbeatTimeout != want {
		t.Errorf("heartbeatTimeout = %v, want configured %v", q.heartbeatTimeout, want)
	}
}

// drainCommandCh reports how many commands are currently buffered in q's
// command channel, without starting the queue's runLoop (so sendCommand
// never runs and q.lastCommand/lastSentTime are whatever the test set).
func drainCommandCh(q *CommandQueue) int {
	n := 0
	for {
		select {
		case <-q.commandCh:
			n++
		default:
			return n
		}
	}
}

func TestCommandQueue_Enqueue_RateLimitsDuplicates(t *testing.T) {
	q := NewCommandQueue(nil, 50, 0)
	// Simulate that CommandForward was just written to the controller.
	q.lastCommand = CommandForward
	q.lastSentTime = time.Now()

	q.Enqueue(CommandForward)

	if n := drainCommandCh(q); n != 0 {
		t.Errorf("Enqueue of an unchanged command within interval wrote %d times, want 0", n)
	}
}

func TestCommandQueue_Enqueue_AllowsDifferentCommandsImmediately(t *testing.T) {
	q := NewCommandQueue(nil, 50, 0)
	q.lastCommand = CommandForward
	q.lastSentTime = time.Now()

	q.Enqueue(CommandLeft)

	if n := drainCommandCh(q); n != 1 {
		t.Errorf("Enqueue of a different command wrote %d times, want 1", n)
	}
}

func TestCommandQueue_Enqueue_ResendsAfterIntervalElapses(t *testing.T) {
	q := NewCommandQueue(nil, 50, 0)
	q.lastCommand = CommandForward
	q.lastSentTime = time.Now().Add(-100 * time.Millisecond) // well past the 50ms interval

	q.Enqueue(CommandForward)

	if n := drainCommandCh(q); n != 1 {
		t.Errorf("Enqueue of an unchanged command after interval elapsed wrote %d times, want 1", n)
	}
}

func TestCommandQueue_Enqueue_FirstCommandAlwaysSent(t *testing.T) {
	q := NewCommandQueue(nil, 50, 0)
	// q.lastCommand/lastSentTime are still zero-valued.

	q.Enqueue(CommandForward)

	if n := drainCommandCh(q); n != 1 {
		t.Errorf("first Enqueue wrote %d times, want 1", n)
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
