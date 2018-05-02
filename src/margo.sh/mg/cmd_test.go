package mg_test

import (
	"bytes"
	"fmt"
	"margo.sh/mg"
	"os"
	"testing"
)

func TestCmdOutputWriter_Copy(t *testing.T) {
	var called1, called2 bool
	u1 := func(*mg.CmdOutputWriter) { called1 = true }
	u2 := func(*mg.CmdOutputWriter) { called2 = true }

	w := &mg.CmdOutputWriter{}
	w.Copy(u1, u2)
	if !called1 {
		t.Error("u1 wasn't called")
	}
	if !called2 {
		t.Error("u2 wasn't called")
	}
}

var errNotEnoughSize = fmt.Errorf("not enough size")

type limitWriter []byte

func (l limitWriter) Write(p []byte) (n int, err error) {
	n = copy(l, p)
	if n < len(p) {
		return n, errNotEnoughSize
	}
	return n, nil
}

// sub-tests that their names end with `closed` tests the first condition of the
// function. Different sizes are provided to examine the returned error. Writes
// should not exceed the size of limitWriter, hence the wantSize.
func TestCmdOutputWriter_Write(t *testing.T) {
	p := []byte("vbfh7H8 P8tSKkJrKKklBnktkOLChBW")

	tcs := []struct {
		name     string
		size     int  // length of limitWriter
		closed   bool // will close the writer before assertions if set true.
		wantSize int
		err      error
	}{
		{"zero size", 0, false, 0, errNotEnoughSize},
		{"small size", len(p) / 2, false, len(p) / 2, errNotEnoughSize},
		{"full size", len(p), false, len(p), nil},
		{"larger size", len(p) * 2, false, len(p), nil},
		{"zero size closed", 0, true, 0, os.ErrClosed},
		{"small size closed", len(p) / 2, true, 0, os.ErrClosed},
		{"full size closed", len(p), true, 0, os.ErrClosed},
		{"larger size closed", len(p) * 2, true, 0, os.ErrClosed},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			buf := make(limitWriter, tc.size)
			w := &mg.CmdOutputWriter{Writer: &buf}
			if tc.closed {
				err := w.Close()
				if err != nil {
					t.Errorf("w.Close() = %v, want nil", err)
					return
				}
			}

			n, err := w.Write(p)
			if err != tc.err {
				t.Errorf("_, err = w.Write(%s): err = %v, want %v", string(p), err, tc.err)
			}
			if n != tc.wantSize {
				t.Errorf("n, _ = w.Write(%s): n = %d, want %d", string(p), n, tc.wantSize)
			}
			readBytes := p[:tc.wantSize]
			wroteBytes := buf[:tc.wantSize]
			if !bytes.Equal(readBytes, wroteBytes) {
				t.Errorf("wroteBytes = %v, want %v", wroteBytes, readBytes)
			}
		})
	}
}

type writerStub struct {
	writeFunc func([]byte) (int, error)
	closeFunc func() error
}

func (w *writerStub) Write(p []byte) (int, error) { return w.writeFunc(p) }
func (w *writerStub) Close() error                { return w.closeFunc() }

func TestCmdOutputWriter_Close_noOutput(t *testing.T) {
	var (
		called    bool
		closed    bool
		errClosed = fmt.Errorf("close call")
	)
	buf := &writerStub{
		writeFunc: func(p []byte) (int, error) {
			called = true
			return len(p), nil
		},
		closeFunc: func() error {
			closed = true
			return errClosed
		},
	}

	w := &mg.CmdOutputWriter{
		Writer: buf,
		Closer: buf,
	}
	err := w.Close()
	if err != errClosed {
		t.Errorf("w.Close() = %v, want %v", err, errClosed)
	}
	if called {
		t.Fatal("didn't expect to call Write")
	}
	if !closed {
		t.Error("expected to call Close")
	}

	err = w.Close()
	if err == nil {
		t.Errorf("w.Close() = nil, want %v", os.ErrClosed)
	}
}

func TestCmdOutputWriter_Close_withOutput(t *testing.T) {
	o1 := []byte("rTuYF2ZPez1wDQNt4")
	o2 := []byte("DJW63ZoTl")
	out := make(chan []byte, 2)
	buf := &writerStub{
		writeFunc: func(p []byte) (int, error) {
			out <- p
			return 0, nil
		},
	}
	w := &mg.CmdOutputWriter{Writer: buf}
	err := w.Close(o1, o2)
	if err != nil {
		t.Errorf("w.Close() = %v, want nil", err)
	}
	for i := 0; i < 2; i++ {
		select {
		case p := <-out:
			if !bytes.Equal(p, o1) && !bytes.Equal(p, o2) {
				t.Errorf("got %v, want %v or %v", p, o1, o2)
			}
		default:
			t.Errorf("expected writing %v or %v", o1, o2)
		}
	}
}

func TestCmdOutputWriter_Output(t *testing.T) {
	buf := new(bytes.Buffer)
	p := []byte("1I9zyYlh98jq5PeW")
	w := &mg.CmdOutputWriter{
		Fd:     "DgJe9ymB0q",
		Writer: buf,
	}
	if _, err := w.Write(p); err != nil {
		t.Fatalf("_, err = w.Write(p); err = %v, want nil", err)
	}

	o := w.Output()
	if o.Fd != w.Fd {
		t.Errorf("o.Fd = %s, want %s", o.Fd, w.Fd)
	}
	if !bytes.Equal(o.Output, buf.Bytes()) {
		t.Errorf("o.Output = %s, want %s", string(o.Output), buf.String())
	}
	if o.Close {
		t.Error("o.Close = true, want false")
	}

	if err := w.Close(); err != nil {
		t.Fatalf("w.Close() = %v, want nil", err)
	}
	o = w.Output()
	if !o.Close {
		t.Error("o.Close = false, want true")
	}
}
