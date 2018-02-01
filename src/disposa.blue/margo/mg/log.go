package mg

import (
	"log"
	"os"
)

var (
	Log = log.New(os.Stderr, "margo@", log.Lshortfile|log.Ltime)
	Dbg = log.New(os.Stderr, "margo/dbg@", log.Lshortfile|log.Ltime)
)
