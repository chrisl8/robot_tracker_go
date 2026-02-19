package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

var QuietMode bool

func init() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.SetPrefix("")
}

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

func Debugf(format string, v ...interface{}) {
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
