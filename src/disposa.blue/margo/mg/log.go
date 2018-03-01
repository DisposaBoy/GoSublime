package mg

import (
	"log"
)

type Logger struct {
	*log.Logger
	Dbg *log.Logger
}
