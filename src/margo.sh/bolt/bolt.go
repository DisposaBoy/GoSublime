package bolt

import (
	"bytes"
	"fmt"
	bolt "github.com/coreos/bbolt"
	"github.com/ugorji/go/codec"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

var (
	DS = func() *DataStore {
		dir := os.Getenv("MARGO_DATA_DIR")
		if dir == "" {
			d, err := ioutil.TempDir("", "margo.data~fallback~")
			if err != nil {
				panic("MARGO_DATA_DIR is not defined and ioutill.TempDir failed: " + err.Error())
			}
			dir = d
		}

		return &DataStore{
			Path:   filepath.Join(dir, "bolt.ds"),
			Handle: &codec.MsgpackHandle{},
			Bucket: []byte("ds"),
		}
	}()
)

type DataStore struct {
	Bucket []byte
	Handle codec.Handle
	Path   string

	mu sync.RWMutex
}

func (ds *DataStore) encodeKey(v interface{}) []byte {
	pkg := ""
	if t := reflect.TypeOf(v); t != nil {
		pkg = t.PkgPath()
	}
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "pkg=%s typ=%T str=%#v", pkg, v, v)
	return buf.Bytes()
}

func (ds *DataStore) encodeVal(v interface{}) ([]byte, error) {
	s := []byte{}
	err := codec.NewEncoderBytes(&s, ds.Handle).Encode(v)
	return s, err
}

func (ds *DataStore) decodeVal(s []byte, p interface{}) error {
	return codec.NewDecoderBytes(s, ds.Handle).Decode(p)
}

func (ds *DataStore) view(f func(*bolt.Tx) error) error {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	return ds.tx(true, f)
}

func (ds *DataStore) update(f func(*bolt.Tx) error) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	return ds.tx(false, f)
}

func (ds *DataStore) tx(view bool, f func(*bolt.Tx) error) error {
	db, err := bolt.Open(ds.Path, 0600, &bolt.Options{
		Timeout: 5 * time.Second,
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

func (ds *DataStore) Load(key, ptr interface{}) error {
	k := ds.encodeKey(key)
	return ds.view(func(tx *bolt.Tx) error {
		bck := tx.Bucket(ds.Bucket)
		if bck == nil {
			return bolt.ErrBucketNotFound
		}
		s := bck.Get(k)
		return ds.decodeVal(s, ptr)
	})
}

func (ds *DataStore) Store(key, val interface{}) error {
	k := ds.encodeKey(key)
	v, err := ds.encodeVal(val)
	if err != nil {
		return err
	}

	return ds.update(func(tx *bolt.Tx) error {
		bck, err := tx.CreateBucketIfNotExists(ds.Bucket)
		if err != nil {
			return err
		}
		return bck.Put(k, v)
	})
}

func (ds *DataStore) Delete(key interface{}) error {
	k := ds.encodeKey(key)
	return ds.update(func(tx *bolt.Tx) error {
		bck := tx.Bucket(ds.Bucket)
		if bck == nil {
			return nil
		}
		return bck.Delete(k)
	})
}
