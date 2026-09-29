package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

var QuietMode bool

// debugEnabled gates Debugf. Debug lines are emitted per frame (steering, paths,
// track positions), so leaving them on in normal runs floods the log file and
// costs time in the frame loop. It is off unless requested with --debug or the
// ROBOT_TRACKER_DEBUG environment variable (so a LaunchAgent can enable it).
var debugEnabled atomic.Bool

func init() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.SetPrefix("")
	debugEnabled.Store(debugFromEnv(os.Getenv("ROBOT_TRACKER_DEBUG")))
}

// debugFromEnv interprets ROBOT_TRACKER_DEBUG: any non-empty value other than
// "0" or "false" turns debug output on.
func debugFromEnv(v string) bool {
	return v != "" && v != "0" && !strings.EqualFold(v, "false")
}

// SetDebug turns Debugf output on or off.
func SetDebug(on bool) { debugEnabled.Store(on) }

// DebugEnabled reports whether Debugf output is on.
func DebugEnabled() bool { return debugEnabled.Load() }

func SetQuietMode(q bool) {
	QuietMode = q
	if QuietMode {
		log.SetOutput(io.Discard)
	} else {
		log.SetOutput(os.Stderr)
	}
}

func Log(v ...interface{}) {
	log.Print(v...)
}

func Logf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// Debugf logs a "DEBUG"-prefixed line, but only when debug output is enabled
// (see SetDebug).
func Debugf(format string, v ...interface{}) {
	if !debugEnabled.Load() {
		return
	}
	log.Printf("DEBUG "+format, v...)
}

// RotateLogFiles shifts numbered backups (.1 → .2 → .3, etc.) and deletes
// the oldest beyond maxBackups. The current log file is renamed to .1.
func RotateLogFiles(logPath string, maxBackups int) error {
	// Delete the oldest backup if it exists (best-effort, file may not exist)
	oldest := fmt.Sprintf("%s.%d", logPath, maxBackups)
	_ = os.Remove(oldest)

	// Shift .N-1 → .N, .N-2 → .N-1, etc. (best-effort, files may not exist)
	for i := maxBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", logPath, i)
		dst := fmt.Sprintf("%s.%d", logPath, i+1)
		_ = os.Rename(src, dst)
	}

	// Rename current log to .1
	if _, err := os.Stat(logPath); err == nil {
		if err := os.Rename(logPath, logPath+".1"); err != nil {
			return fmt.Errorf("rotating log file: %w", err)
		}
	}

	return nil
}

// SetupLogFile rotates existing logs and creates a fresh log file. It sets
// log.SetOutput, os.Stdout, and os.Stderr to point at the new file so that
// all output (including panics and stray fmt.Println calls) goes to the log.
func SetupLogFile(logPath string, maxBackups int) (*os.File, error) {
	// Ensure parent directory exists
	// #nosec G301
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}

	if err := RotateLogFiles(logPath, maxBackups); err != nil {
		return nil, err
	}

	// #nosec G304
	// #nosec G302
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("creating log file: %w", err)
	}

	log.SetOutput(f)
	os.Stdout = f
	os.Stderr = f

	return f, nil
}
