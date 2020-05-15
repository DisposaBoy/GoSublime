package mgutil

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

type SplitFunc func(buf []byte) (next, rest []byte, ok bool)

var ErrSplitWriterClosed = errors.New("SplitWriter: closed")

func SplitLine(s []byte) (line, rest []byte, ok bool) {
	i := bytes.IndexByte(s, '\n')
	if i < 0 {
		return nil, s, false
	}
	i++
	return s[:i], s[i:], true
}

func SplitLineOrCR(s []byte) (line, rest []byte, ok bool) {
	i := bytes.IndexByte(s, '\n')
	if i < 0 {
		i = bytes.IndexByte(s, '\r')
		if i < 0 {
			return nil, s, false
		}
	}
	i++
	return s[:i], s[i:], true
}

type SplitWriter struct {
	split  SplitFunc
	write  func([]byte) (int, error)
	close  func() error
	fflush func() error

	mu  sync.Mutex
	err error
	buf []byte
}

func (w *SplitWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.err != nil {
		return 0, w.err
	}

	w.buf = append(w.buf, p...)
	rest := w.buf
	for len(rest) != 0 {
		p, s, ok := w.split(rest)
		rest = s
		if !ok {
			break
		}
		if _, w.err = w.write(p[:len(p):len(p)]); w.err != nil {
			return len(p), w.err
		}
	}
	if len(rest) == 0 {
		w.buf = nil
	} else if len(rest) < len(w.buf) {
		w.buf = append(w.buf[:0], rest...)
	}
	return len(p), nil
}

func (w *SplitWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.flush(false)
}

func (w *SplitWriter) flush(closing bool) error {
	if w.err != nil {
		return w.err
	}
	if closing && len(w.buf) != 0 {
		_, w.err = w.write(w.buf)
		w.buf = nil
	}
	if err := w.fflush(); err != nil && w.err == nil {
		w.err = err
	}
	return w.err
}

func (w *SplitWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.err == ErrSplitWriterClosed {
		return w.err
	}
	flushErr := w.flush(true)
	w.err = ErrSplitWriterClosed
	if err := w.close(); err != nil {
		return err
	}
	return flushErr
}

func NewSplitWriter(split SplitFunc, w io.WriteCloser) *SplitWriter {
	return &SplitWriter{
		split:  split,
		write:  w.Write,
		close:  w.Close,
		fflush: func() error { return nil },
	}
}

func NewSplitStream(split SplitFunc, w OutputStream) *SplitWriter {
	return &SplitWriter{
		split:  split,
		write:  w.Write,
		close:  w.Close,
		fflush: w.Flush,
	}
}
