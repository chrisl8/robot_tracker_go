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
