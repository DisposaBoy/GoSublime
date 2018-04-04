package mg

import (
	"io"
	"log"
)

type Logger struct {
	*log.Logger
	Dbg *log.Logger
}

func NewLogger(w io.Writer) *Logger {
	return &Logger{
		Logger: log.New(w, "", log.Lshortfile),
		Dbg:    log.New(w, "DBG: ", log.Lshortfile),
	}
}
