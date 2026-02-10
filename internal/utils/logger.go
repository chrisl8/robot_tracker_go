package utils

import (
	"io"
	"log"
	"os"
)

var QuietMode bool

func init() {
	log.SetFlags(0)
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
