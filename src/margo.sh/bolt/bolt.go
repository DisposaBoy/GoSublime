package bolt

import (
	bolt "github.com/coreos/bbolt"
	"github.com/ugorji/go/codec"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	DefaultKV = func() *BoltKV {
		dir := os.Getenv("MARGO_DATA_DIR")
		if dir == "" {
			d, err := ioutil.TempDir("", "margo.data~fallback~")
			if err != nil {
				panic("MARGO_DATA_DIR is not defined and ioutill.TempDir failed: " + err.Error())
			}
			dir = d
		}

		return &BoltKV{
			Path:   filepath.Join(dir, "bolt.kv"),
			Handle: &codec.MsgpackHandle{},
			Bucket: []byte("kv"),
		}
	}()
)

type BoltKV struct {
	Bucket []byte
	Handle codec.Handle
	Path   string

	mu sync.RWMutex
}

func (bs *BoltKV) encode(v interface{}) ([]byte, error) {
	s := []byte{}
	err := codec.NewEncoderBytes(&s, bs.Handle).Encode(v)
	return s, err
}

func (bs *BoltKV) decode(s []byte, p interface{}) error {
	return codec.NewDecoderBytes(s, bs.Handle).Decode(p)
}

func (bs *BoltKV) view(f func(*bolt.Tx) error) error {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	return bs.tx(true, f)
}

func (bs *BoltKV) update(f func(*bolt.Tx) error) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	return bs.tx(false, f)
}

func (bs *BoltKV) tx(view bool, f func(*bolt.Tx) error) error {
	db, err := bolt.Open(bs.Path, 0600, &bolt.Options{
		Timeout:  5 * time.Second,
		ReadOnly: view,
	})
	if err != nil {
		return err
	}
	defer db.Close()

	if view {
		return db.View(f)
	}
	return db.Update(f)
}

func (bs *BoltKV) Load(key, ptr interface{}) error {
	k, err := bs.encode(key)
	if err != nil {
		return err
	}

	return bs.view(func(tx *bolt.Tx) error {
		bck := tx.Bucket(bs.Bucket)
		if bck == nil {
			return bolt.ErrBucketNotFound
		}

		s := bck.Get(k)
		return bs.decode(s, ptr)
	})
}

func (bs *BoltKV) Store(key, val interface{}) error {
	k, err := bs.encode(key)
	if err != nil {
		return err
	}

	v, err := bs.encode(val)
	if err != nil {
		return err
	}

	return bs.update(func(tx *bolt.Tx) error {
		bck, err := tx.CreateBucketIfNotExists(bs.Bucket)
		if err != nil {
			return err
		}
		return bck.Put(k, v)
	})
}

func (bs *BoltKV) Delete(key interface{}) error {
	k, err := bs.encode(key)
	if err != nil {
		return err
	}

	return bs.update(func(tx *bolt.Tx) error {
		bck := tx.Bucket(bs.Bucket)
		if bck == nil {
			return nil
		}
		return bck.Delete(k)
	})
}
