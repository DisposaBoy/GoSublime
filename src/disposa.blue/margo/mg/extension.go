package mg

import (
	"log"
)

type Args struct {
	Store *Store
	Log   *log.Logger
	Dbg   *log.Logger
}

type MargoFunc func(Args)
