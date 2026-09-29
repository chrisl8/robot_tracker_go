package utils

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

// captureLog redirects the standard logger to a buffer for one test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	prevDebug := DebugEnabled()
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		SetDebug(prevDebug)
	})
	return &buf
}

func TestDebugf_SilentByDefaultAndWhenDisabled(t *testing.T) {
	buf := captureLog(t)
	SetDebug(false)

	Debugf("steering %d", 42)

	if buf.Len() != 0 {
		t.Errorf("Debugf wrote %q while debug is off, want nothing", buf.String())
	}
}

func TestDebugf_LogsWithPrefixWhenEnabled(t *testing.T) {
	buf := captureLog(t)
	SetDebug(true)

	Debugf("steering %d", 42)

	if got := buf.String(); !strings.Contains(got, "DEBUG steering 42") {
		t.Errorf("Debugf wrote %q, want it to contain %q", got, "DEBUG steering 42")
	}
}

// Normal logging must never depend on the debug switch.
func TestLogf_UnaffectedByDebugSwitch(t *testing.T) {
	buf := captureLog(t)
	SetDebug(false)

	Logf("hello %s", "world")

	if !strings.Contains(buf.String(), "hello world") {
		t.Errorf("Logf output %q missing message", buf.String())
	}
}

func TestSetDebug_RoundTrips(t *testing.T) {
	captureLog(t)
	SetDebug(true)
	if !DebugEnabled() {
		t.Error("DebugEnabled() = false after SetDebug(true)")
	}
	SetDebug(false)
	if DebugEnabled() {
		t.Error("DebugEnabled() = true after SetDebug(false)")
	}
}

func TestDebugFromEnv(t *testing.T) {
	for v, want := range map[string]bool{
		"": false, "0": false, "false": false, "FALSE": false,
		"1": true, "true": true, "yes": true, "debug": true,
	} {
		if got := debugFromEnv(v); got != want {
			t.Errorf("debugFromEnv(%q) = %v, want %v", v, got, want)
		}
	}
}
