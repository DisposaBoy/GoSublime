package mg

import (
	"log"
	"os"
)

var (
	Log = log.New(os.Stderr, "", log.Lshortfile)
	Dbg = log.New(os.Stderr, "DBG: ", log.Lshortfile)
)
