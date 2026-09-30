package controller

import (
	"reflect"
	"testing"
	"time"
)

func TestParseReplyLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []int
		ok   bool
	}{
		{"valid reply", "#R=783,577,441,1000", []int{783, 577, 441, 1000}, true},
		{"trailing CR", "#R=1,2,3,4\r", []int{1, 2, 3, 4}, true},
		{"different field count", "#R=5", []int{5}, true},
		{"bad checksum report", "#RBAD:sum", nil, false},
		{"boot banner", "#GV3r1c-Chris10-0.02", nil, false},
		{"empty", "", nil, false},
		{"empty payload", "#R=", nil, false},
		{"non-numeric field", "#R=1,x,3", nil, false},
		{"corrupted prefix", "#r=1,2,3", nil, false},
		{"embedded garbage", "\x92\x9a#R=1,2", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseReplyLine(tt.line)
			if ok != tt.ok || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseReplyLine(%q) = %v, %v; want %v, %v", tt.line, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestRobotLink_StateMachine(t *testing.T) {
	base := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		connectedAt time.Time
		lastReply   time.Time
		now         time.Time
		want        RobotLink
	}{
		{"never connected", time.Time{}, time.Time{}, base, RobotLinkUnknown},
		{"just connected, no reply yet", base, time.Time{}, base.Add(RobotSilentAfter - time.Millisecond), RobotLinkUnknown},
		{"connected, no reply for too long", base, time.Time{}, base.Add(RobotSilentAfter), RobotLinkSilent},
		{"fresh reply", base, base.Add(time.Second), base.Add(2 * time.Second), RobotLinkAlive},
		{"reply exactly at the limit", base, base.Add(time.Second), base.Add(time.Second + RobotSilentAfter), RobotLinkAlive},
		{"reply gone stale", base, base.Add(time.Second), base.Add(time.Second + RobotSilentAfter + time.Millisecond), RobotLinkSilent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewArduinoController("test", 0)
			c.connectedAt, c.lastReply = tt.connectedAt, tt.lastReply
			if got := c.RobotLink(tt.now); got != tt.want {
				t.Errorf("RobotLink = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadLoop_RepliesMakeLinkAliveAndBadOnesAreCounted(t *testing.T) {
	port := &recordingPort{}
	stubSerialOpen(t, port)
	ctrl := NewArduinoController("test", 0)
	if err := ctrl.Connect(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ctrl.Disconnect() }()

	if got := ctrl.RobotLink(time.Now()); got != RobotLinkUnknown {
		t.Fatalf("before any reply RobotLink = %q, want unknown", got)
	}

	// Lines can be split across reads, banner text is ignored, and #RBAD is counted.
	port.feed("#GV3r1c-Chris10-0.02\r\n#RBAD:sum\n#R=7")
	port.feed("83,577,441,1000\n")

	waitFor(t, func() bool { return ctrl.RobotLink(time.Now()) == RobotLinkAlive })
	if got := ctrl.BadReplies(); got != 1 {
		t.Errorf("BadReplies = %d, want 1", got)
	}
}

func TestReadLoop_SilenceBecomesSilentAndReplyRecovers(t *testing.T) {
	port := &recordingPort{}
	stubSerialOpen(t, port)
	ctrl := NewArduinoController("test", 0)
	if err := ctrl.Connect(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ctrl.Disconnect() }()

	port.feed("#R=1,2,3,4\n")
	waitFor(t, func() bool { return ctrl.RobotLink(time.Now()) == RobotLinkAlive })

	later := time.Now().Add(RobotSilentAfter + time.Second)
	if got := ctrl.RobotLink(later); got != RobotLinkSilent {
		t.Errorf("after %v of silence RobotLink = %q, want silent", RobotSilentAfter+time.Second, got)
	}

	port.feed("#R=1,2,3,4\n")
	waitFor(t, func() bool { return ctrl.RobotLink(time.Now()) == RobotLinkAlive })
}

// An unplugged cable while idle used to go unnoticed until the next command.
func TestReadLoop_ReadFailureDropsConnection(t *testing.T) {
	port := &recordingPort{}
	stubSerialOpen(t, port)
	ctrl := NewArduinoController("test", 0)
	if err := ctrl.Connect(); err != nil {
		t.Fatal(err)
	}

	port.failReads.Store(true)

	waitFor(t, func() bool { return !ctrl.IsConnected() })
	if !port.closed.Load() {
		t.Error("the failed port must be closed")
	}
	if got := ctrl.RobotLink(time.Now()); got != RobotLinkUnknown {
		t.Errorf("RobotLink after a drop = %q, want unknown", got)
	}
}

// Disconnect closes the port, which makes the reader's Read fail; that must not
// be mistaken for a fault on a newer connection.
func TestReadLoop_StaleReaderDoesNotDropReplacementConnection(t *testing.T) {
	first, second := &recordingPort{}, &recordingPort{}
	stubSerialOpen(t, first, second)
	ctrl := NewArduinoController("test", 0)
	if err := ctrl.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := ctrl.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if err := ctrl.Connect(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ctrl.Disconnect() }()

	time.Sleep(50 * time.Millisecond) // give the first reader time to notice its port closed
	if !ctrl.IsConnected() {
		t.Error("closing the old port must not drop the new connection")
	}
}

func TestIsReboot(t *testing.T) {
	tests := []struct {
		name      string
		prev, cur int
		want      bool
	}{
		{"uptime advances", 10, 11, false},
		{"same second twice", 10, 10, false},
		{"uptime goes backwards", 500, 3, true},
		{"restart right after a long run", 40000, 0, true},
		{"16-bit counter wraps", 65535, 0, false},
		{"wrap with a small step past zero", 65534, 4, false},
		{"just below the wrap floor is a reboot", 64999, 3, true},
		{"high uptime dropping to a mid value is a reboot", 65500, 2000, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isReboot(tt.prev, tt.cur); got != tt.want {
				t.Errorf("isReboot(%d, %d) = %v, want %v", tt.prev, tt.cur, got, tt.want)
			}
		})
	}
}

func TestNoteReply_HeartbeatFieldsAndReboots(t *testing.T) {
	c := NewArduinoController("test", 0)
	c.connectedAt = time.Now()

	// First heartbeat after connecting is never a reboot, however small the uptime.
	if c.noteReply([]int{5, 0, 'W'}) {
		t.Fatal("first heartbeat reported a reboot")
	}
	if c.noteReply([]int{6, 1, 'D'}) {
		t.Fatal("advancing uptime reported a reboot")
	}
	info := c.RobotInfo(time.Now())
	if info.Link != RobotLinkAlive || info.Servos != "asleep" || info.Mode != "D" || info.Reboots != 0 {
		t.Errorf("info = %+v, want alive/asleep/D/0 reboots", info)
	}

	if !c.noteReply([]int{1, 0, 'W'}) {
		t.Fatal("uptime going backwards was not reported as a reboot")
	}
	info = c.RobotInfo(time.Now())
	if info.Reboots != 1 || info.Servos != "awake" || info.Mode != "W" {
		t.Errorf("after reboot info = %+v, want 1 reboot/awake/W", info)
	}
}

// A reply that isn't a well-formed heartbeat still proves the robot answered,
// but must not disturb the recorded state or fake a reboot.
func TestNoteReply_MalformedHeartbeatOnlyCountsAsAlive(t *testing.T) {
	for _, values := range [][]int{
		{1, 2},          // wrong field count
		{1, 2, 3, 4},    // the old sensor format
		{-1, 0, 'W'},    // uptime out of range
		{70000, 0, 'W'}, // uptime out of range
		{10, 2, 'W'},    // detached flag is not 0/1
		{10, 0, 300},    // mode is not a byte
	} {
		c := NewArduinoController("test", 0)
		c.connectedAt = time.Now()
		c.noteReply([]int{500, 0, 'W'})

		if c.noteReply(values) {
			t.Errorf("%v reported a reboot", values)
		}
		info := c.RobotInfo(time.Now())
		if info.Link != RobotLinkAlive || info.Reboots != 0 || info.Servos != "awake" {
			t.Errorf("%v disturbed the state: %+v", values, info)
		}
		if c.noteReply([]int{501, 0, 'W'}) {
			t.Errorf("%v: a later valid heartbeat was taken for a reboot", values)
		}
	}
}

func TestRobotInfo_DetailsHiddenUnlessAlive(t *testing.T) {
	c := NewArduinoController("test", 0)
	c.connectedAt = time.Now()
	c.noteReply([]int{9, 1, 'W'})

	info := c.RobotInfo(time.Now().Add(RobotSilentAfter + time.Second))
	if info.Link != RobotLinkSilent || info.Servos != "" || info.Mode != "" {
		t.Errorf("a silent robot should report no servo/mode details, got %+v", info)
	}
}

func TestReadLoop_HeartbeatLinesUpdateInfoAndCountReboots(t *testing.T) {
	port := &recordingPort{}
	stubSerialOpen(t, port)
	ctrl := NewArduinoController("test", 0)
	if err := ctrl.Connect(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ctrl.Disconnect() }()

	port.feed("#R=100,1,87\n#R=101,1,87\n#R=2,0,87\n") // 87 is 'W'

	waitFor(t, func() bool { return ctrl.RobotInfo(time.Now()).Reboots == 1 })
	info := ctrl.RobotInfo(time.Now())
	if info.Servos != "awake" || info.Mode != "W" {
		t.Errorf("info = %+v, want awake/W after the last heartbeat", info)
	}
}

func TestDisconnect_ForgetsHeartbeatButKeepsRebootCount(t *testing.T) {
	c := NewArduinoController("test", 0)
	c.connectedAt = time.Now()
	c.noteReply([]int{50, 0, 'W'})
	c.noteReply([]int{1, 0, 'W'}) // a reboot

	c.mu.Lock()
	c.noteDisconnected()
	c.mu.Unlock()
	c.noteConnected()

	// After reconnecting, the next heartbeat is a fresh baseline, not a reboot.
	if c.noteReply([]int{0, 0, 'W'}) { // lower than the last value seen before the disconnect
		t.Error("the first heartbeat after reconnecting was reported as a reboot")
	}
	if got := c.RobotInfo(time.Now()).Reboots; got != 1 {
		t.Errorf("reboot count = %d, want it kept at 1 across the reconnect", got)
	}
}
