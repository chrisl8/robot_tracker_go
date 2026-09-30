package controller

import (
	"bytes"
	"strconv"
	"strings"
	"time"

	"go.bug.st/serial"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// RobotLink says whether the robot itself is answering, as opposed to the USB
// link to the gamepad Arduino (see IsConnected). The gamepad firmware polls the
// robot about once a second and prints each valid reply as a "#R=..." line;
// silence with the serial port open means the robot or its radio is down.
type RobotLink string

const (
	// RobotLinkUnknown: not connected, or connected for less than
	// RobotSilentAfter without having heard a reply yet.
	RobotLinkUnknown RobotLink = "unknown"
	RobotLinkAlive   RobotLink = "alive"
	RobotLinkSilent  RobotLink = "silent"
)

// RobotSilentAfter is how long without a reply (or, after connecting, without a
// first reply) before the robot is reported silent: three missed polls.
const RobotSilentAfter = 3 * time.Second

const (
	replyPrefix    = "#R="
	replyBadPrefix = "#RBAD"
	maxReplyLine   = 256 // longer lines are garbage; drop them
)

// parseReplyLine parses a "#R=a,b,c,d" line into its integer fields. Other
// lines (the boot banner, "#RBAD:..." reports, robot chatter) are not replies.
func parseReplyLine(line string) ([]int, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(line), replyPrefix)
	if !ok || rest == "" {
		return nil, false
	}
	parts := strings.Split(rest, ",")
	values := make([]int, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		values = append(values, v)
	}
	return values, true
}

// noteConnected starts a fresh link-watch period. Callers hold c.mu.
func (c *ArduinoController) noteConnected() {
	c.linkMu.Lock()
	c.connectedAt = time.Now()
	c.lastReply = time.Time{}
	c.linkMu.Unlock()
}

// noteDisconnected forgets the link state. Callers hold c.mu.
func (c *ArduinoController) noteDisconnected() {
	c.linkMu.Lock()
	c.connectedAt = time.Time{}
	c.lastReply = time.Time{}
	c.linkMu.Unlock()
}

func (c *ArduinoController) noteLine(line string) {
	if _, ok := parseReplyLine(line); ok {
		c.linkMu.Lock()
		c.lastReply = time.Now()
		c.linkMu.Unlock()
		return
	}
	if strings.HasPrefix(line, replyBadPrefix) {
		c.linkMu.Lock()
		c.badReplies++
		c.linkMu.Unlock()
	}
}

// RobotLink reports the robot's link state at now. It takes only the link
// lock, not the serial lock, so a stalled serial write can't freeze the caller.
func (c *ArduinoController) RobotLink(now time.Time) RobotLink {
	c.linkMu.Lock()
	defer c.linkMu.Unlock()
	if c.connectedAt.IsZero() {
		return RobotLinkUnknown
	}
	if c.lastReply.IsZero() {
		if now.Sub(c.connectedAt) < RobotSilentAfter {
			return RobotLinkUnknown
		}
		return RobotLinkSilent
	}
	if now.Sub(c.lastReply) <= RobotSilentAfter {
		return RobotLinkAlive
	}
	return RobotLinkSilent
}

// BadReplies is how many replies the gamepad reported as corrupt (#RBAD) since
// the controller was created.
func (c *ArduinoController) BadReplies() int {
	c.linkMu.Lock()
	defer c.linkMu.Unlock()
	return c.badReplies
}

// readLoop consumes the gamepad's output for one open port until the port
// closes. Reading also notices an unplugged cable while idle, which a
// write-only controller only finds out about on its next command.
func (c *ArduinoController) readLoop(sp serial.Port) {
	buf := make([]byte, 256)
	var line []byte
	for {
		n, err := sp.Read(buf)
		if err != nil {
			c.mu.Lock()
			// Only tear down our own port: Disconnect or a failed write may
			// already have closed it and a reconnect replaced it.
			if c.serial == sp {
				utils.Logf("Arduino read failed, dropping connection: %v", err)
				c.dropConnectionLocked()
			}
			c.mu.Unlock()
			return
		}
		for _, b := range buf[:n] {
			if b != '\n' {
				if len(line) < maxReplyLine {
					line = append(line, b)
				}
				continue
			}
			c.noteLine(string(bytes.TrimRight(line, "\r")))
			line = line[:0]
		}
	}
}
