package vfs

import (
	"margo.sh/memo"
	"os"
	"sync"
	"time"
)

const (
	fmodeZero    = fmode(os.ModeIrregular)
	fmodeDir     = fmode(os.ModeDir)
	fmodeSymlink = fmode(os.ModeSymlink)
	fmodeType    = fmode(os.ModeType)
	metaMaxAge   = 17
)

var (
	epoch = time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC).Unix()

	_ os.FileInfo = (*FileInfo)(nil)
)

type fmode os.FileMode

func (fm fmode) IsValid() bool { return fm != fmodeZero }

func (fm fmode) IsDir() bool { return fm&(fmodeDir|fmodeSymlink) == fmodeDir }

func (fm fmode) IsRegular() bool { return fm&fmodeType == 0 }

func (fm fmode) IsSymlink() bool { return fm&fmodeSymlink != 0 }

func (fm fmode) Mode() os.FileMode { return os.FileMode(fm) }

type timestamp uint32

func (ts timestamp) time() time.Time { return time.Unix(epoch+int64(ts), 0) }

type meta struct {
	fmode fmode
	modts timestamp
	expts timestamp
	mo    *memo.M
}

func (mt *meta) memo(poke bool) *memo.M {
	if mt == nil {
		return nil
	}
	if mt.mo == nil && poke {
		mt.mo = &memo.M{}
	}
	return mt.mo
}

func (mt *meta) invalidate() {
	if mt == nil {
		return
	}
	mt.expts = 0
	mt.mo.Clear()
}

func (mt *meta) ok() bool {
	return mt != nil && mt.expts > 0 && mt.fmode.IsValid() && tsNow() < mt.expts
}

func (mt *meta) resetMemoAfter(ts timestamp) {
	if mt == nil {
		return
	}
	if ts > 0 && mt.modts >= ts {
		return
	}
	mt.mo.Clear()
}

func (mt *meta) resetMemo() {
	mt.resetMemoAfter(0)
}

func (mt *meta) resetInfo(mode os.FileMode, mtime time.Time) {
	now := tsNow()
	mt.fmode = fmode(mode)
	if !mtime.IsZero() || !mt.fmode.IsDir() {
		mt.expts = now + metaMaxAge
	}
	switch {
	case !mtime.IsZero():
		mt.modts = tsTime(mtime)
	case mt.modts == 0:
		mt.modts = now
	}
}

type FileInfo struct {
	Node  *Node
	mu    sync.Mutex
	fmode fmode
	mtime time.Time
	size  int64
	sys   interface{}
}

func (fi *FileInfo) Name() string { return fi.Node.Name() }

func (fi *FileInfo) load() {
	st, err := os.Lstat(fi.Node.Path())
	if err != nil {
		return
	}

	fi.fmode = fmode(st.Mode())
	fi.mtime = st.ModTime()
	fi.size = st.Size()
	fi.sys = st.Sys()

	fi.Node.mu.Lock()
	fi.Node.meta(true).resetInfo(st.Mode(), st.ModTime())
	fi.Node.mu.Unlock()
}

func (fi *FileInfo) IsDir() bool { return fi.Mode().IsDir() }

func (fi *FileInfo) Mode() os.FileMode {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if !fi.fmode.IsValid() {
		fi.load()
	}
	return os.FileMode(fi.fmode)
}

func (fi *FileInfo) Size() int64 {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if fi.size == 0 {
		fi.load()
	}
	return fi.size
}

func (fi *FileInfo) Sys() interface{} {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if fi.sys == nil {
		fi.load()
	}
	return fi.sys
}

func (fi *FileInfo) ModTime() time.Time {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	if fi.mtime.IsZero() {
		fi.load()
	}
	return fi.mtime
}

type Dirent struct {
	name string
	fmode
}

func (de *Dirent) Name() string { return de.name }

func tsTime(t time.Time) timestamp {
	n := t.Unix()
	if n <= epoch {
		return 0
	}
	return timestamp(n - epoch)
}

func tsNow() timestamp {
	return tsTime(time.Now())
}
