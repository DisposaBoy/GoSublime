package vfs

import (
	"bytes"
	"io"
	"io/ioutil"
	"margo.sh/memo"
)

type blobNodeKey struct {
	// new fields might be added in the future
}

type Blob struct {
	Limit int
	src   []byte
	err   error
}

func (b *Blob) Reader() io.Reader {
	return bytes.NewReader(b.src)
}

func (b *Blob) ReadCloser() io.ReadCloser { return ioutil.NopCloser(b.Reader()) }

func (b *Blob) Error() error { return b.err }

func (b *Blob) ReadFile() ([]byte, error) { return b.src, nil }

func (b *Blob) OpenFile() (io.ReadCloser, error) { return b.ReadCloser(), b.Error() }

func (b *Blob) Len() int { return len(b.src) }

// TODO: add support for size-classed blobs
func peekBlob(nd *Node) *Blob {
	b, _ := nd.PeekMemo(blobNodeKey{}).(*Blob)
	return b
}

// TODO: add support for size-classed blobs
func readBlob(nd *Node) *Blob {
	return nd.ReadMemo(blobNodeKey{}, func() interface{} {
		fn := nd.Path()
		src, err := ioutil.ReadFile(fn)
		if err != nil {
			return &Blob{err: err}
		}
		return &Blob{src: src}
	}).(*Blob)
}

func Blobs(nd *Node) (blobs []*Blob) {
	nd.pkMemo().Range(func(k memo.K, v memo.V) {
		if _, ok := k.(blobNodeKey); !ok {
			return
		}
		if b, ok := v.(*Blob); ok && b != nil {
			blobs = append(blobs, b)
		}
	})
	return blobs
}
