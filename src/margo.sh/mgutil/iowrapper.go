package mgutil

import (
	"io"
	"sync"
)

// IOWrapper implements various optional io interfaces.
// It delegates to the interface fields that are not nil
type IOWrapper struct {
	// If Locker is not nil, all methods are called while holding the lock
	Locker sync.Locker

	// If Reader is not nil, it will be called to handle reads
	Reader io.Reader

	// If Writer is not nil, it will be called to handle writes
	Writer io.Writer

	// If Closer is not nil, it will be called to handle closes
	Closer io.Closer

	// If Flusher is not nil, it will be called to handle flushes
	Flusher interface{ Flush() error }
}

// lockUnlock locks Locker if it's not nil and returns Locker.Unlock
// otherwise it returns a nop unlock function
func (iow *IOWrapper) lockUnlock() func() {
	if mu := iow.Locker; mu != nil {
		mu.Lock()
		return mu.Unlock
	}
	return func() {}
}

// Read calls Reader.Read() if Reader is not nil
// otherwise it returns `0, io.EOF`
func (iow *IOWrapper) Read(p []byte) (int, error) {
	defer iow.lockUnlock()()

	if r := iow.Reader; r != nil {
		return r.Read(p)
	}
	return 0, io.EOF
}

// Write calls Writer.Write() if Writer is not nil
// otherwise it returns `len(p), nil`
func (iow *IOWrapper) Write(p []byte) (int, error) {
	defer iow.lockUnlock()()

	if w := iow.Writer; w != nil {
		return w.Write(p)
	}
	return len(p), nil
}

// Close calls Closer.Close() if Closer is not nil
// otherwise it returns `nil`
func (iow *IOWrapper) Close() error {
	defer iow.lockUnlock()()

	if c := iow.Closer; c != nil {
		return c.Close()
	}
	return nil
}

// Flush calls Flushr.Flush() if Flusher is not nil
// otherwise it returns `nil`
func (iow *IOWrapper) Flush() error {
	defer iow.lockUnlock()()

	if f := iow.Flusher; f != nil {
		return f.Flush()
	}
	return nil
}
