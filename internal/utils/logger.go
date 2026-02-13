package utils

import (
	"io"
	"log"
	"os"
)

var QuietMode bool
var DebugMode bool

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

func SetDebugMode(d bool) {
	DebugMode = d
}

func Debugf(format string, v ...interface{}) {
	if DebugMode {
		log.Printf("DEBUG "+format, v...)
	}
}
