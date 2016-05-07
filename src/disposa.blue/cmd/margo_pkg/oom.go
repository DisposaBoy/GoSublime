package margo_pkg

import (
	"log"
	"runtime"
	"time"
)

func startOomKiller(maxMb int) {
	go func() {
		const M = uint64(1024 * 1024)
		runtime.LockOSThread()

		var mst runtime.MemStats
		buf := make([]byte, 1*M)
		f := "MarGo: OOM.\n" +
			"Memory limit: %vm\n" +
			"Memory usage: %vm\n" +
			"Number goroutines: %v\n" +
			"------- begin stack trace ----\n" +
			"\n%s\n\n" +
			"-------  end stack trace  ----\n"

		for {
			runtime.ReadMemStats(&mst)
			alloc := int(mst.Sys / M)
			if alloc >= maxMb {
				n := runtime.Stack(buf, true)
				log.Fatalf(f, maxMb, alloc, runtime.NumGoroutine(), buf[:n])
			}
			time.Sleep(time.Second * 2)
		}
	}()
}
