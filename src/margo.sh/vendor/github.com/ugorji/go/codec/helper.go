// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

// Contains code shared by both encode and decode.

// Some shared ideas around encoding/decoding
// ------------------------------------------
//
// If an interface{} is passed, we first do a type assertion to see if it is
// a primitive type or a map/slice of primitive types, and use a fastpath to handle it.
//
// If we start with a reflect.Value, we are already in reflect.Value land and
// will try to grab the function for the underlying Type and directly call that function.
// This is more performant than calling reflect.Value.Interface().
//
// This still helps us bypass many layers of reflection, and give best performance.
//
// Containers
// ------------
// Containers in the stream are either associative arrays (key-value pairs) or
// regular arrays (indexed by incrementing integers).
//
// Some streams support indefinite-length containers, and use a breaking
// byte-sequence to denote that the container has come to an end.
//
// Some streams also are text-based, and use explicit separators to denote the
// end/beginning of different values.
//
// Philosophy
// ------------
// On decode, this codec will update containers appropriately:
//    - If struct, update fields from stream into fields of struct.
//      If field in stream not found in struct, handle appropriately (based on option).
//      If a struct field has no corresponding value in the stream, leave it AS IS.
//      If nil in stream, set value to nil/zero value.
//    - If map, update map from stream.
//      If the stream value is NIL, set the map to nil.
//    - if slice, try to update up to length of array in stream.
//      if container len is less than stream array length,
//      and container cannot be expanded, handled (based on option).
//      This means you can decode 4-element stream array into 1-element array.
//
// ------------------------------------
// On encode, user can specify omitEmpty. This means that the value will be omitted
// if the zero value. The problem may occur during decode, where omitted values do not affect
// the value being decoded into. This means that if decoding into a struct with an
// int field with current value=5, and the field is omitted in the stream, then after
// decoding, the value will still be 5 (not 0).
// omitEmpty only works if you guarantee that you always decode into zero-values.
//
// ------------------------------------
// We could have truncated a map to remove keys not available in the stream,
// or set values in the struct which are not in the stream to their zero values.
// We decided against it because there is no efficient way to do it.
// We may introduce it as an option later.
// However, that will require enabling it for both runtime and code generation modes.
//
// To support truncate, we need to do 2 passes over the container:
//   map
//   - first collect all keys (e.g. in k1)
//   - for each key in stream, mark k1 that the key should not be removed
//   - after updating map, do second pass and call delete for all keys in k1 which are not marked
//   struct:
//   - for each field, track the *typeInfo s1
//   - iterate through all s1, and for each one not marked, set value to zero
//   - this involves checking the possible anonymous fields which are nil ptrs.
//     too much work.
//
// ------------------------------------------
// Error Handling is done within the library using panic.
//
// This way, the code doesn't have to keep checking if an error has happened,
// and we don't have to keep sending the error value along with each call
// or storing it in the En|Decoder and checking it constantly along the way.
//
// We considered storing the error is En|Decoder.
//   - once it has its err field set, it cannot be used again.
//   - panicing will be optional, controlled by const flag.
//   - code should always check error first and return early.
//
// We eventually decided against it as it makes the code clumsier to always
// check for these error conditions.
//
// ------------------------------------------
// We use sync.Pool only for the aid of long-lived objects shared across multiple goroutines.
// Encoder, Decoder, enc|decDriver, reader|writer, etc do not fall into this bucket.
//
// Also, GC is much better now, eliminating some of the reasons to use a shared pool structure.
// Instead, the short-lived objects use free-lists that live as long as the object exists.
//
// ------------------------------------------
// Performance is affected by the following:
//    - Bounds Checking
//    - Inlining
//    - Pointer chasing
// This package tries hard to manage the performance impact of these.
//
// ------------------------------------------
// To alleviate performance due to pointer-chasing:
//    - Prefer non-pointer values in a struct field
//    - Refer to these directly within helper classes
//      e.g. json.go refers directly to d.d.decRd
//
// We made the changes to embed En/Decoder in en/decDriver,
// but we had to explicitly reference the fields as opposed to using a function
// to get the better performance that we were looking for.
// For example, we explicitly call d.d.decRd.fn() instead of d.d.r().fn().
//
// ------------------------------------------
// Bounds Checking
//    - Allow bytesDecReader to incur "bounds check error", and
//      recover that as an io.EOF.
//      This allows the bounds check branch to always be taken by the branch predictor,
//      giving better performance (in theory), while ensuring that the code is shorter.

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// rvNLen is the length of the array for readn or writen calls
	rwNLen = 7

	scratchByteArrayLen = 64
	// initCollectionCap   = 16 // 32 is defensive. 16 is preferred.

	// Support encoding.(Binary|Text)(Unm|M)arshaler.
	// This constant flag will enable or disable it.
	supportMarshalInterfaces = true

	// for debugging, set this to false, to catch panic traces.
	// Note that this will always cause rpc tests to fail, since they need io.EOF sent via panic.
	recoverPanicToErr = true

	// arrayCacheLen is the length of the cache used in encoder or decoder for
	// allowing zero-alloc initialization.
	// arrayCacheLen = 8

	// size of the cacheline: defaulting to value for archs: amd64, arm64, 386
	// should use "runtime/internal/sys".CacheLineSize, but that is not exposed.
	cacheLineSize = 64

	wordSizeBits = 32 << (^uint(0) >> 63) // strconv.IntSize
	wordSize     = wordSizeBits / 8

	// so structFieldInfo fits into 8 bytes
	maxLevelsEmbedding = 14

	// useFinalizers=true configures finalizers to release pool'ed resources
	// acquired by Encoder/Decoder during their GC.
	//
	// Note that calling SetFinalizer is always expensive,
	// as code must be run on the systemstack even for SetFinalizer(t, nil).
	//
	// We document that folks SHOULD call Release() when done, or they can
	// explicitly call SetFinalizer themselves e.g.
	//    runtime.SetFinalizer(e, (*Encoder).Release)
	//    runtime.SetFinalizer(d, (*Decoder).Release)
	useFinalizers = false

	// // usePool controls whether we use sync.Pool or not.
	// //
	// // sync.Pool can help manage memory use, but it may come at a performance cost.
	// usePool = false

	// xdebug controls whether xdebugf prints any output
	xdebug = true
)

var (
	oneByteArr    [1]byte
	zeroByteSlice = oneByteArr[:0:0]

	codecgen bool

	// defPooler pooler
	panicv panicHdl

	refBitset    bitset32
	isnilBitset  bitset32
	scalarBitset bitset32
)

var (
	errMapTypeNotMapKind     = errors.New("MapType MUST be of Map Kind")
	errSliceTypeNotSliceKind = errors.New("SliceType MUST be of Slice Kind")
)

var (
	pool4tiload = sync.Pool{New: func() interface{} { return new(typeInfoLoadArray) }}

	// pool4sfiRv8   = sync.Pool{New: func() interface{} { return new([8]sfiRv) }}
	// pool4sfiRv16  = sync.Pool{New: func() interface{} { return new([16]sfiRv) }}
	// pool4sfiRv32  = sync.Pool{New: func() interface{} { return new([32]sfiRv) }}
	// pool4sfiRv64  = sync.Pool{New: func() interface{} { return new([64]sfiRv) }}
	// pool4sfiRv128 = sync.Pool{New: func() interface{} { return new([128]sfiRv) }}

	// // dn = sync.Pool{ New: func() interface{} { x := new(decNaked); x.init(); return x } }

	// pool4buf256 = sync.Pool{New: func() interface{} { return new([256]byte) }}
	// pool4buf1k  = sync.Pool{New: func() interface{} { return new([1 * 1024]byte) }}
	// pool4buf2k  = sync.Pool{New: func() interface{} { return new([2 * 1024]byte) }}
	// pool4buf4k  = sync.Pool{New: func() interface{} { return new([4 * 1024]byte) }}
	// pool4buf8k  = sync.Pool{New: func() interface{} { return new([8 * 1024]byte) }}
	// pool4buf16k = sync.Pool{New: func() interface{} { return new([16 * 1024]byte) }}
	// pool4buf32k = sync.Pool{New: func() interface{} { return new([32 * 1024]byte) }}
	// pool4buf64k = sync.Pool{New: func() interface{} { return new([64 * 1024]byte) }}

	// pool4mapStrU16 = sync.Pool{New: func() interface{} { return make(map[string]uint16, 16) }}
	// pool4mapU16Str   = sync.Pool{New: func() interface{} { return make(map[uint16]string, 16) }}
	// pool4mapU16Bytes = sync.Pool{New: func() interface{} { return make(map[uint16][]byte, 16) }}
)

func init() {
	// defPooler.init()

	refBitset = refBitset.
		set(byte(reflect.Map)).
		set(byte(reflect.Ptr)).
		set(byte(reflect.Func)).
		set(byte(reflect.Chan)).
		set(byte(reflect.UnsafePointer))

	isnilBitset = isnilBitset.
		set(byte(reflect.Map)).
		set(byte(reflect.Ptr)).
		set(byte(reflect.Func)).
		set(byte(reflect.Chan)).
		set(byte(reflect.UnsafePointer)).
		set(byte(reflect.Interface)).
		set(byte(reflect.Slice))

	scalarBitset = scalarBitset.
		set(byte(reflect.Bool)).
		set(byte(reflect.Int)).
		set(byte(reflect.Int8)).
		set(byte(reflect.Int16)).
		set(byte(reflect.Int32)).
		set(byte(reflect.Int64)).
		set(byte(reflect.Uint)).
		set(byte(reflect.Uint8)).
		set(byte(reflect.Uint16)).
		set(byte(reflect.Uint32)).
		set(byte(reflect.Uint64)).
		set(byte(reflect.Uintptr)).
		set(byte(reflect.Float32)).
		set(byte(reflect.Float64)).
		set(byte(reflect.Complex64)).
		set(byte(reflect.Complex128)).
		set(byte(reflect.String))

	// xdebugf("bitsets: ref: %b, isnil: %b, scalar: %b", refBitset, isnilBitset, scalarBitset)
}

type handleFlag uint8

const (
	initedHandleFlag handleFlag = 1 << iota
	binaryHandleFlag
	jsonHandleFlag
)

type clsErr struct {
	closed    bool  // is it closed?
	errClosed error // error on closing
}

// type entryType uint8

// const (
// 	entryTypeBytes entryType = iota // make this 0, so a comparison is cheap
// 	entryTypeIo
// 	entryTypeBufio
// 	entryTypeUnset = 255
// )

type charEncoding uint8

const (
	_ charEncoding = iota // make 0 unset
	cUTF8
	cUTF16LE
	cUTF16BE
	cUTF32LE
	cUTF32BE
	// Deprecated: not a true char encoding value
	cRAW charEncoding = 255
)

// valueType is the stream type
type valueType uint8

const (
	valueTypeUnset valueType = iota
	valueTypeNil
	valueTypeInt
	valueTypeUint
	valueTypeFloat
	valueTypeBool
	valueTypeString
	valueTypeSymbol
	valueTypeBytes
	valueTypeMap
	valueTypeArray
	valueTypeTime
	valueTypeExt

	// valueTypeInvalid = 0xff
)

var valueTypeStrings = [...]string{
	"Unset",
	"Nil",
	"Int",
	"Uint",
	"Float",
	"Bool",
	"String",
	"Symbol",
	"Bytes",
	"Map",
	"Array",
	"Timestamp",
	"Ext",
}

func (x valueType) String() string {
	if int(x) < len(valueTypeStrings) {
		return valueTypeStrings[x]
	}
	return strconv.FormatInt(int64(x), 10)
}

type seqType uint8

const (
	_ seqType = iota
	seqTypeArray
	seqTypeSlice
	seqTypeChan
)

// note that containerMapStart and containerArraySend are not sent.
// This is because the ReadXXXStart and EncodeXXXStart already does these.
type containerState uint8

const (
	_ containerState = iota

	containerMapStart
	containerMapKey
	containerMapValue
	containerMapEnd
	containerArrayStart
	containerArrayElem
	containerArrayEnd
)

// // sfiIdx used for tracking where a (field/enc)Name is seen in a []*structFieldInfo
// type sfiIdx struct {
// 	name  string
// 	index int
// }

// do not recurse if a containing type refers to an embedded type
// which refers back to its containing type (via a pointer).
// The second time this back-reference happens, break out,
// so as not to cause an infinite loop.
const rgetMaxRecursion = 2

// Anecdotally, we believe most types have <= 12 fields.
// - even Java's PMD rules set TooManyFields threshold to 15.
// However, go has embedded fields, which should be regarded as
// top level, allowing structs to possibly double or triple.
// In addition, we don't want to keep creating transient arrays,
// especially for the sfi index tracking, and the evtypes tracking.
//
// So - try to keep typeInfoLoadArray within 2K bytes
const (
	typeInfoLoadArraySfisLen   = 16
	typeInfoLoadArraySfiidxLen = 8 * 112
	typeInfoLoadArrayEtypesLen = 12
	typeInfoLoadArrayBLen      = 8 * 4
)

// typeInfoLoad is a transient object used while loading up a typeInfo.
type typeInfoLoad struct {
	// fNames   []string
	// encNames []string
	etypes []uintptr
	sfis   []structFieldInfo
}

// typeInfoLoadArray is a cache object used to efficiently load up a typeInfo without
// much allocation.
type typeInfoLoadArray struct {
	// fNames   [typeInfoLoadArrayLen]string
	// encNames [typeInfoLoadArrayLen]string
	sfis   [typeInfoLoadArraySfisLen]structFieldInfo
	sfiidx [typeInfoLoadArraySfiidxLen]byte
	etypes [typeInfoLoadArrayEtypesLen]uintptr
	b      [typeInfoLoadArrayBLen]byte // scratch - used for struct field names
}

// // cacheLineSafer denotes that a type is safe for cache-line access.
// // This could mean that
// type cacheLineSafer interface {
// 	cacheLineSafe()
// }

// mirror json.Marshaler and json.Unmarshaler here,
// so we don't import the encoding/json package

type jsonMarshaler interface {
	MarshalJSON() ([]byte, error)
}
type jsonUnmarshaler interface {
	UnmarshalJSON([]byte) error
}

type isZeroer interface {
	IsZero() bool
}

type codecError struct {
	name string
	err  interface{}
}

func (e codecError) Cause() error {
	switch xerr := e.err.(type) {
	case nil:
		return nil
	case error:
		return xerr
	case string:
		return errors.New(xerr)
	case fmt.Stringer:
		return errors.New(xerr.String())
	default:
		return fmt.Errorf("%v", e.err)
	}
}

func (e codecError) Error() string {
	return fmt.Sprintf("%s error: %v", e.name, e.err)
}

// type byteAccepter func(byte) bool

var (
	bigen               = binary.BigEndian
	structInfoFieldName = "_struct"

	mapStrIntfTyp  = reflect.TypeOf(map[string]interface{}(nil))
	mapIntfIntfTyp = reflect.TypeOf(map[interface{}]interface{}(nil))
	intfSliceTyp   = reflect.TypeOf([]interface{}(nil))
	intfTyp        = intfSliceTyp.Elem()

	reflectValTyp = reflect.TypeOf((*reflect.Value)(nil)).Elem()

	stringTyp     = reflect.TypeOf("")
	timeTyp       = reflect.TypeOf(time.Time{})
	rawExtTyp     = reflect.TypeOf(RawExt{})
	rawTyp        = reflect.TypeOf(Raw{})
	uintptrTyp    = reflect.TypeOf(uintptr(0))
	uint8Typ      = reflect.TypeOf(uint8(0))
	uint8SliceTyp = reflect.TypeOf([]uint8(nil))
	uintTyp       = reflect.TypeOf(uint(0))
	intTyp        = reflect.TypeOf(int(0))

	mapBySliceTyp = reflect.TypeOf((*MapBySlice)(nil)).Elem()

	binaryMarshalerTyp   = reflect.TypeOf((*encoding.BinaryMarshaler)(nil)).Elem()
	binaryUnmarshalerTyp = reflect.TypeOf((*encoding.BinaryUnmarshaler)(nil)).Elem()

	textMarshalerTyp   = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	textUnmarshalerTyp = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

	jsonMarshalerTyp   = reflect.TypeOf((*jsonMarshaler)(nil)).Elem()
	jsonUnmarshalerTyp = reflect.TypeOf((*jsonUnmarshaler)(nil)).Elem()

	selferTyp         = reflect.TypeOf((*Selfer)(nil)).Elem()
	missingFielderTyp = reflect.TypeOf((*MissingFielder)(nil)).Elem()
	iszeroTyp         = reflect.TypeOf((*isZeroer)(nil)).Elem()

	uint8TypId      = rt2id(uint8Typ)
	uint8SliceTypId = rt2id(uint8SliceTyp)
	rawExtTypId     = rt2id(rawExtTyp)
	rawTypId        = rt2id(rawTyp)
	intfTypId       = rt2id(intfTyp)
	timeTypId       = rt2id(timeTyp)
	stringTypId     = rt2id(stringTyp)

	mapStrIntfTypId  = rt2id(mapStrIntfTyp)
	mapIntfIntfTypId = rt2id(mapIntfIntfTyp)
	intfSliceTypId   = rt2id(intfSliceTyp)
	// mapBySliceTypId  = rt2id(mapBySliceTyp)

	intBitsize  = uint8(intTyp.Bits())
	uintBitsize = uint8(uintTyp.Bits())

	// bsAll0x00 = []byte{0, 0, 0, 0, 0, 0, 0, 0}
	bsAll0xff = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

	chkOvf checkOverflow

	errNoFieldNameToStructFieldInfo = errors.New("no field name passed to parseStructFieldInfo")
)

var defTypeInfos = NewTypeInfos([]string{"codec", "json"})

var immutableKindsSet = [32]bool{
	// reflect.Invalid:  ,
	reflect.Bool:       true,
	reflect.Int:        true,
	reflect.Int8:       true,
	reflect.Int16:      true,
	reflect.Int32:      true,
	reflect.Int64:      true,
	reflect.Uint:       true,
	reflect.Uint8:      true,
	reflect.Uint16:     true,
	reflect.Uint32:     true,
	reflect.Uint64:     true,
	reflect.Uintptr:    true,
	reflect.Float32:    true,
	reflect.Float64:    true,
	reflect.Complex64:  true,
	reflect.Complex128: true,
	// reflect.Array
	// reflect.Chan
	// reflect.Func: true,
	// reflect.Interface
	// reflect.Map
	// reflect.Ptr
	// reflect.Slice
	reflect.String: true,
	// reflect.Struct
	// reflect.UnsafePointer
}

// SelfExt is a sentinel extension signifying that types
// registered with it SHOULD be encoded and decoded
// based on the naive mode of the format.
//
// This allows users to define a tag for an extension,
// but signify that the types should be encoded/decoded as the native encoding.
// This way, users need not also define how to encode or decode the extension.
var SelfExt = &extFailWrapper{}

// Selfer defines methods by which a value can encode or decode itself.
//
// Any type which implements Selfer will be able to encode or decode itself.
// Consequently, during (en|de)code, this takes precedence over
// (text|binary)(M|Unm)arshal or extension support.
//
// By definition, it is not allowed for a Selfer to directly call Encode or Decode on itself.
// If that is done, Encode/Decode will rightfully fail with a Stack Overflow style error.
// For example, the snippet below will cause such an error.
//     type testSelferRecur struct{}
//     func (s *testSelferRecur) CodecEncodeSelf(e *Encoder) { e.MustEncode(s) }
//     func (s *testSelferRecur) CodecDecodeSelf(d *Decoder) { d.MustDecode(s) }
//
// Note: *the first set of bytes of any value MUST NOT represent nil in the format*.
// This is because, during each decode, we first check the the next set of bytes
// represent nil, and if so, we just set the value to nil.
type Selfer interface {
	CodecEncodeSelf(*Encoder)
	CodecDecodeSelf(*Decoder)
}

// MissingFielder defines the interface allowing structs to internally decode or encode
// values which do not map to struct fields.
//
// We expect that this interface is bound to a pointer type (so the mutation function works).
//
// A use-case is if a version of a type unexports a field, but you want compatibility between
// both versions during encoding and decoding.
//
// Note that the interface is completely ignored during codecgen.
type MissingFielder interface {
	// CodecMissingField is called to set a missing field and value pair.
	//
	// It returns true if the missing field was set on the struct.
	CodecMissingField(field []byte, value interface{}) bool

	// CodecMissingFields returns the set of fields which are not struct fields
	CodecMissingFields() map[string]interface{}
}

// MapBySlice is a tag interface that denotes wrapped slice should encode as a map in the stream.
// The slice contains a sequence of key-value pairs.
// This affords storing a map in a specific sequence in the stream.
//
// Example usage:
//    type T1 []string         // or []int or []Point or any other "slice" type
//    func (_ T1) MapBySlice{} // T1 now implements MapBySlice, and will be encoded as a map
//    type T2 struct { KeyValues T1 }
//
//    var kvs = []string{"one", "1", "two", "2", "three", "3"}
//    var v2 = T2{ KeyValues: T1(kvs) }
//    // v2 will be encoded like the map: {"KeyValues": {"one": "1", "two": "2", "three": "3"} }
//
// The support of MapBySlice affords the following:
//   - A slice type which implements MapBySlice will be encoded as a map
//   - A slice can be decoded from a map in the stream
//   - It MUST be a slice type (not a pointer receiver) that implements MapBySlice
type MapBySlice interface {
	MapBySlice()
}

// BasicHandle encapsulates the common options and extension functions.
//
// Deprecated: DO NOT USE DIRECTLY. EXPORTED FOR GODOC BENEFIT. WILL BE REMOVED.
type BasicHandle struct {
	// BasicHandle is always a part of a different type.
	// It doesn't have to fit into it own cache lines.

	// TypeInfos is used to get the type info for any type.
	//
	// If not configured, the default TypeInfos is used, which uses struct tag keys: codec, json
	TypeInfos *TypeInfos

	// Note: BasicHandle is not comparable, due to these slices here (extHandle, intf2impls).
	// If *[]T is used instead, this becomes comparable, at the cost of extra indirection.
	// Thses slices are used all the time, so keep as slices (not pointers).

	extHandle

	rtidFns      atomicRtidFnSlice
	rtidFnsNoExt atomicRtidFnSlice

	// ---- cache line

	DecodeOptions

	// ---- cache line

	EncodeOptions

	intf2impls

	mu     sync.Mutex
	inited uint32 // holds if inited, and also handle flags (binary encoding, json handler, etc)

	RPCOptions

	// TimeNotBuiltin configures whether time.Time should be treated as a builtin type.
	//
	// All Handlers should know how to encode/decode time.Time as part of the core
	// format specification, or as a standard extension defined by the format.
	//
	// However, users can elect to handle time.Time as a custom extension, or via the
	// standard library's encoding.Binary(M|Unm)arshaler or Text(M|Unm)arshaler interface.
	// To elect this behavior, users can set TimeNotBuiltin=true.
	//
	// Note: Setting TimeNotBuiltin=true can be used to enable the legacy behavior
	// (for Cbor and Msgpack), where time.Time was not a builtin supported type.
	//
	// Note: DO NOT CHANGE AFTER FIRST USE.
	//
	// Once a Handle has been used, do not modify this option.
	// It will lead to unexpected behaviour during encoding and decoding.
	TimeNotBuiltin bool

	// ExplicitRelease configures whether Release() is implicitly called after an encode or
	// decode call.
	//
	// If you will hold onto an Encoder or Decoder for re-use, by calling Reset(...)
	// on it or calling (Must)Encode repeatedly into a given []byte or io.Writer,
	// then you do not want it to be implicitly closed after each Encode/Decode call.
	// Doing so will unnecessarily return resources to the shared pool, only for you to
	// grab them right after again to do another Encode/Decode call.
	//
	// Instead, you configure ExplicitRelease=true, and you explicitly call Release() when
	// you are truly done.
	//
	// As an alternative, you can explicitly set a finalizer - so its resources
	// are returned to the shared pool before it is garbage-collected. Do it as below:
	//    runtime.SetFinalizer(e, (*Encoder).Release)
	//    runtime.SetFinalizer(d, (*Decoder).Release)
	//
	// Deprecated: This is not longer used as pools are only used for long-lived objects
	// which are shared across goroutines.
	// Setting this value has no effect. It is maintained for backward compatibility.
	ExplicitRelease bool

	// flags handleFlag // holds flag for if binaryEncoding, jsonHandler, etc
	// be    bool       // is handle a binary encoding?
	// js    bool       // is handle javascript handler?
	// n  byte // first letter of handle name
	// _  uint16 // padding

	// ---- cache line

	// noBuiltInTypeChecker

	// _      uint32 // padding
	// r []uintptr     // rtids mapped to s above
}

// basicHandle returns an initialized BasicHandle from the Handle.
func basicHandle(hh Handle) (x *BasicHandle) {
	x = hh.getBasicHandle()
	// ** We need to simulate once.Do, to ensure no data race within the block.
	// ** Consequently, below would not work.
	// if atomic.CompareAndSwapUint32(&x.inited, 0, 1) {
	// 	x.be = hh.isBinary()
	// 	_, x.js = hh.(*JsonHandle)
	// 	x.n = hh.Name()[0]
	// }

	// simulate once.Do using our own stored flag and mutex as a CompareAndSwap
	// is not sufficient, since a race condition can occur within init(Handle) function.
	// init is made noinline, so that this function can be inlined by its caller.
	if atomic.LoadUint32(&x.inited) == 0 {
		x.init(hh)
	}
	return
}

func (x *BasicHandle) isJs() bool {
	return handleFlag(x.inited)&jsonHandleFlag != 0
}

func (x *BasicHandle) isBe() bool {
	return handleFlag(x.inited)&binaryHandleFlag != 0
}

//go:noinline
func (x *BasicHandle) init(hh Handle) {
	// make it uninlineable, as it is called at most once
	x.mu.Lock()
	if x.inited == 0 {
		var f = initedHandleFlag
		if hh.isBinary() {
			f |= binaryHandleFlag
		}
		if _, b := hh.(*JsonHandle); b {
			f |= jsonHandleFlag
		}
		// _, x.js = hh.(*JsonHandle)
		// x.n = hh.Name()[0]
		atomic.StoreUint32(&x.inited, uint32(f))
		// ensure MapType and SliceType are of correct type
		if x.MapType != nil && x.MapType.Kind() != reflect.Map {
			panic(errMapTypeNotMapKind)
		}
		if x.SliceType != nil && x.SliceType.Kind() != reflect.Slice {
			panic(errSliceTypeNotSliceKind)
		}
	}
	x.mu.Unlock()
}

func (x *BasicHandle) getBasicHandle() *BasicHandle {
	return x
}

func (x *BasicHandle) getTypeInfo(rtid uintptr, rt reflect.Type) (pti *typeInfo) {
	if x.TypeInfos == nil {
		return defTypeInfos.get(rtid, rt)
	}
	return x.TypeInfos.get(rtid, rt)
}

func findFn(s []codecRtidFn, rtid uintptr) (i uint, fn *codecFn) {
	// binary search. adapted from sort/search.go.
	// Note: we use goto (instead of for loop) so this can be inlined.

	// h, i, j := 0, 0, len(s)
	var h uint // var h, i uint
	var j = uint(len(s))
LOOP:
	if i < j {
		h = i + (j-i)/2
		if s[h].rtid < rtid {
			i = h + 1
		} else {
			j = h
		}
		goto LOOP
	}
	if i < uint(len(s)) && s[i].rtid == rtid {
		fn = s[i].fn
	}
	return
}

func (x *BasicHandle) fn(rt reflect.Type) (fn *codecFn) {
	return x.fnVia(rt, &x.rtidFns, true)
}

func (x *BasicHandle) fnNoExt(rt reflect.Type) (fn *codecFn) {
	return x.fnVia(rt, &x.rtidFnsNoExt, false)
}

func (x *BasicHandle) fnVia(rt reflect.Type, fs *atomicRtidFnSlice, checkExt bool) (fn *codecFn) {
	rtid := rt2id(rt)
	sp := fs.load()
	if sp != nil {
		if _, fn = findFn(sp, rtid); fn != nil {
			return
		}
	}
	fn = x.fnLoad(rt, rtid, checkExt)
	x.mu.Lock()
	var sp2 []codecRtidFn
	sp = fs.load()
	if sp == nil {
		sp2 = []codecRtidFn{{rtid, fn}}
		fs.store(sp2)
	} else {
		idx, fn2 := findFn(sp, rtid)
		if fn2 == nil {
			sp2 = make([]codecRtidFn, len(sp)+1)
			copy(sp2, sp[:idx])
			copy(sp2[idx+1:], sp[idx:])
			sp2[idx] = codecRtidFn{rtid, fn}
			fs.store(sp2)
		}
	}
	x.mu.Unlock()
	return
}

func (x *BasicHandle) fnLoad(rt reflect.Type, rtid uintptr, checkExt bool) (fn *codecFn) {
	fn = new(codecFn)
	fi := &(fn.i)
	ti := x.getTypeInfo(rtid, rt)
	fi.ti = ti

	rk := reflect.Kind(ti.kind)

	// anything can be an extension except the built-in ones: time, raw and rawext

	if rtid == timeTypId && !x.TimeNotBuiltin {
		fn.fe = (*Encoder).kTime
		fn.fd = (*Decoder).kTime
	} else if rtid == rawTypId {
		fn.fe = (*Encoder).raw
		fn.fd = (*Decoder).raw
	} else if rtid == rawExtTypId {
		fn.fe = (*Encoder).rawExt
		fn.fd = (*Decoder).rawExt
		fi.addrF = true
		fi.addrD = true
		fi.addrE = true
	} else if xfFn := x.getExt(rtid, checkExt); xfFn != nil {
		fi.xfTag, fi.xfFn = xfFn.tag, xfFn.ext
		fn.fe = (*Encoder).ext
		fn.fd = (*Decoder).ext
		fi.addrF = true
		fi.addrD = true
		if rk == reflect.Struct || rk == reflect.Array {
			fi.addrE = true
		}
	} else if ti.isFlag(tiflagSelfer) || ti.isFlag(tiflagSelferPtr) {
		fn.fe = (*Encoder).selferMarshal
		fn.fd = (*Decoder).selferUnmarshal
		fi.addrF = true
		fi.addrD = ti.isFlag(tiflagSelferPtr)
		fi.addrE = ti.isFlag(tiflagSelferPtr)
	} else if supportMarshalInterfaces && x.isBe() &&
		(ti.isFlag(tiflagBinaryMarshaler) || ti.isFlag(tiflagBinaryMarshalerPtr)) &&
		(ti.isFlag(tiflagBinaryUnmarshaler) || ti.isFlag(tiflagBinaryUnmarshalerPtr)) {
		fn.fe = (*Encoder).binaryMarshal
		fn.fd = (*Decoder).binaryUnmarshal
		fi.addrF = true
		fi.addrD = ti.isFlag(tiflagBinaryUnmarshalerPtr)
		fi.addrE = ti.isFlag(tiflagBinaryMarshalerPtr)
	} else if supportMarshalInterfaces && !x.isBe() && x.isJs() &&
		(ti.isFlag(tiflagJsonMarshaler) || ti.isFlag(tiflagJsonMarshalerPtr)) &&
		(ti.isFlag(tiflagJsonUnmarshaler) || ti.isFlag(tiflagJsonUnmarshalerPtr)) {
		//If JSON, we should check JSONMarshal before textMarshal
		fn.fe = (*Encoder).jsonMarshal
		fn.fd = (*Decoder).jsonUnmarshal
		fi.addrF = true
		fi.addrD = ti.isFlag(tiflagJsonUnmarshalerPtr)
		fi.addrE = ti.isFlag(tiflagJsonMarshalerPtr)
	} else if supportMarshalInterfaces && !x.isBe() &&
		(ti.isFlag(tiflagTextMarshaler) || ti.isFlag(tiflagTextMarshalerPtr)) &&
		(ti.isFlag(tiflagTextUnmarshaler) || ti.isFlag(tiflagTextUnmarshalerPtr)) {
		fn.fe = (*Encoder).textMarshal
		fn.fd = (*Decoder).textUnmarshal
		fi.addrF = true
		fi.addrD = ti.isFlag(tiflagTextUnmarshalerPtr)
		fi.addrE = ti.isFlag(tiflagTextMarshalerPtr)
	} else {
		if fastpathEnabled && (rk == reflect.Map || rk == reflect.Slice) {
			if ti.pkgpath == "" { // un-named slice or map
				if idx := fastpathAV.index(rtid); idx != -1 {
					fn.fe = fastpathAV[idx].encfn
					fn.fd = fastpathAV[idx].decfn
					fi.addrD = true
					fi.addrF = false
				}
			} else {
				// use mapping for underlying type if there
				var rtu reflect.Type
				if rk == reflect.Map {
					rtu = reflect.MapOf(ti.key, ti.elem)
				} else {
					rtu = reflect.SliceOf(ti.elem)
				}
				rtuid := rt2id(rtu)
				if idx := fastpathAV.index(rtuid); idx != -1 {
					xfnf := fastpathAV[idx].encfn
					xrt := fastpathAV[idx].rt
					fn.fe = func(e *Encoder, xf *codecFnInfo, xrv reflect.Value) {
						xfnf(e, xf, rvConvert(xrv, xrt))
					}
					fi.addrD = true
					fi.addrF = false // meaning it can be an address(ptr) or a value
					xfnf2 := fastpathAV[idx].decfn
					xptr2rt := reflect.PtrTo(xrt)
					fn.fd = func(d *Decoder, xf *codecFnInfo, xrv reflect.Value) {
						// xdebug2f("fd: convert from %v to %v", xrv.Type(), xrt)
						if xrv.Kind() == reflect.Ptr {
							xfnf2(d, xf, rvConvert(xrv, xptr2rt))
						} else {
							xfnf2(d, xf, rvConvert(xrv, xrt))
						}
					}
				}
			}
		}
		if fn.fe == nil && fn.fd == nil {
			switch rk {
			case reflect.Bool:
				fn.fe = (*Encoder).kBool
				fn.fd = (*Decoder).kBool
			case reflect.String:
				// Do not check this here, as it will statically set the function for a string
				// type, and if the Handle is modified thereafter, behaviour is non-deterministic.
				//
				// if x.StringToRaw {
				// 	fn.fe = (*Encoder).kStringToRaw
				// } else {
				// 	fn.fe = (*Encoder).kStringEnc
				// }
				fn.fe = (*Encoder).kString
				fn.fd = (*Decoder).kString
			case reflect.Int:
				fn.fd = (*Decoder).kInt
				fn.fe = (*Encoder).kInt
			case reflect.Int8:
				fn.fe = (*Encoder).kInt8
				fn.fd = (*Decoder).kInt8
			case reflect.Int16:
				fn.fe = (*Encoder).kInt16
				fn.fd = (*Decoder).kInt16
			case reflect.Int32:
				fn.fe = (*Encoder).kInt32
				fn.fd = (*Decoder).kInt32
			case reflect.Int64:
				fn.fe = (*Encoder).kInt64
				fn.fd = (*Decoder).kInt64
			case reflect.Uint:
				fn.fd = (*Decoder).kUint
				fn.fe = (*Encoder).kUint
			case reflect.Uint8:
				fn.fe = (*Encoder).kUint8
				fn.fd = (*Decoder).kUint8
			case reflect.Uint16:
				fn.fe = (*Encoder).kUint16
				fn.fd = (*Decoder).kUint16
			case reflect.Uint32:
				fn.fe = (*Encoder).kUint32
				fn.fd = (*Decoder).kUint32
			case reflect.Uint64:
				fn.fe = (*Encoder).kUint64
				fn.fd = (*Decoder).kUint64
			case reflect.Uintptr:
				fn.fe = (*Encoder).kUintptr
				fn.fd = (*Decoder).kUintptr
			case reflect.Float32:
				fn.fe = (*Encoder).kFloat32
				fn.fd = (*Decoder).kFloat32
			case reflect.Float64:
				fn.fe = (*Encoder).kFloat64
				fn.fd = (*Decoder).kFloat64
			case reflect.Invalid:
				fn.fe = (*Encoder).kInvalid
				fn.fd = (*Decoder).kErr
			case reflect.Chan:
				fi.seq = seqTypeChan
				fn.fe = (*Encoder).kSlice
				fn.fd = (*Decoder).kSliceForChan
			case reflect.Slice:
				fi.seq = seqTypeSlice
				fn.fe = (*Encoder).kSlice
				fn.fd = (*Decoder).kSlice
			case reflect.Array:
				fi.seq = seqTypeArray
				fn.fe = (*Encoder).kSlice
				fi.addrF = false
				fi.addrD = false
				rt2 := reflect.SliceOf(ti.elem)
				fn.fd = func(d *Decoder, xf *codecFnInfo, xrv reflect.Value) {
					d.h.fn(rt2).fd(d, xf, rvGetSlice4Array(xrv, rt2))
				}
				// fn.fd = (*Decoder).kArray
			case reflect.Struct:
				if ti.anyOmitEmpty ||
					ti.isFlag(tiflagMissingFielder) ||
					ti.isFlag(tiflagMissingFielderPtr) {
					fn.fe = (*Encoder).kStruct
				} else {
					fn.fe = (*Encoder).kStructNoOmitempty
				}
				fn.fd = (*Decoder).kStruct
			case reflect.Map:
				fn.fe = (*Encoder).kMap
				fn.fd = (*Decoder).kMap
			case reflect.Interface:
				// encode: reflect.Interface are handled already by preEncodeValue
				fn.fd = (*Decoder).kInterface
				fn.fe = (*Encoder).kErr
			default:
				// reflect.Ptr and reflect.Interface are handled already by preEncodeValue
				fn.fe = (*Encoder).kErr
				fn.fd = (*Decoder).kErr
			}
		}
	}
	return
}

// Handle defines a specific encoding format. It also stores any runtime state
// used during an Encoding or Decoding session e.g. stored state about Types, etc.
//
// Once a handle is configured, it can be shared across multiple Encoders and Decoders.
//
// Note that a Handle is NOT safe for concurrent modification.
//
// A Handle also should not be modified after it is configured and has
// been used at least once. This is because stored state may be out of sync with the
// new configuration, and a data race can occur when multiple goroutines access it.
// i.e. multiple Encoders or Decoders in different goroutines.
//
// Consequently, the typical usage model is that a Handle is pre-configured
// before first time use, and not modified while in use.
// Such a pre-configured Handle is safe for concurrent access.
type Handle interface {
	Name() string
	// return the basic handle. It may not have been inited.
	// Prefer to use basicHandle() helper function that ensures it has been inited.
	getBasicHandle() *BasicHandle
	// recreateEncDriver(encDriver) bool
	newEncDriver() encDriver
	newDecDriver() decDriver
	isBinary() bool
	// hasElemSeparators() bool
	// IsBuiltinType(rtid uintptr) bool
}

// Raw represents raw formatted bytes.
// We "blindly" store it during encode and retrieve the raw bytes during decode.
// Note: it is dangerous during encode, so we may gate the behaviour
// behind an Encode flag which must be explicitly set.
type Raw []byte

// RawExt represents raw unprocessed extension data.
// Some codecs will decode extension data as a *RawExt
// if there is no registered extension for the tag.
//
// Only one of Data or Value is nil.
// If Data is nil, then the content of the RawExt is in the Value.
type RawExt struct {
	Tag uint64
	// Data is the []byte which represents the raw ext. If nil, ext is exposed in Value.
	// Data is used by codecs (e.g. binc, msgpack, simple) which do custom serialization of types
	Data []byte
	// Value represents the extension, if Data is nil.
	// Value is used by codecs (e.g. cbor, json) which leverage the format to do
	// custom serialization of the types.
	Value interface{}
}

// BytesExt handles custom (de)serialization of types to/from []byte.
// It is used by codecs (e.g. binc, msgpack, simple) which do custom serialization of the types.
type BytesExt interface {
	// WriteExt converts a value to a []byte.
	//
	// Note: v is a pointer iff the registered extension type is a struct or array kind.
	WriteExt(v interface{}) []byte

	// ReadExt updates a value from a []byte.
	//
	// Note: dst is always a pointer kind to the registered extension type.
	ReadExt(dst interface{}, src []byte)
}

// InterfaceExt handles custom (de)serialization of types to/from another interface{} value.
// The Encoder or Decoder will then handle the further (de)serialization of that known type.
//
// It is used by codecs (e.g. cbor, json) which use the format to do custom serialization of types.
type InterfaceExt interface {
	// ConvertExt converts a value into a simpler interface for easy encoding
	// e.g. convert time.Time to int64.
	//
	// Note: v is a pointer iff the registered extension type is a struct or array kind.
	ConvertExt(v interface{}) interface{}

	// UpdateExt updates a value from a simpler interface for easy decoding
	// e.g. convert int64 to time.Time.
	//
	// Note: dst is always a pointer kind to the registered extension type.
	UpdateExt(dst interface{}, src interface{})
}

// Ext handles custom (de)serialization of custom types / extensions.
type Ext interface {
	BytesExt
	InterfaceExt
}

// addExtWrapper is a wrapper implementation to support former AddExt exported method.
type addExtWrapper struct {
	encFn func(reflect.Value) ([]byte, error)
	decFn func(reflect.Value, []byte) error
}

func (x addExtWrapper) WriteExt(v interface{}) []byte {
	bs, err := x.encFn(rv4i(v))
	if err != nil {
		panic(err)
	}
	return bs
}

func (x addExtWrapper) ReadExt(v interface{}, bs []byte) {
	if err := x.decFn(rv4i(v), bs); err != nil {
		panic(err)
	}
}

func (x addExtWrapper) ConvertExt(v interface{}) interface{} {
	return x.WriteExt(v)
}

func (x addExtWrapper) UpdateExt(dest interface{}, v interface{}) {
	x.ReadExt(dest, v.([]byte))
}

type bytesExtFailer struct{}

func (bytesExtFailer) WriteExt(v interface{}) []byte {
	panicv.errorstr("BytesExt.WriteExt is not supported")
	return nil
}
func (bytesExtFailer) ReadExt(v interface{}, bs []byte) {
	panicv.errorstr("BytesExt.ReadExt is not supported")
}

type interfaceExtFailer struct{}

func (interfaceExtFailer) ConvertExt(v interface{}) interface{} {
	panicv.errorstr("InterfaceExt.ConvertExt is not supported")
	return nil
}
func (interfaceExtFailer) UpdateExt(dest interface{}, v interface{}) {
	panicv.errorstr("InterfaceExt.UpdateExt is not supported")
}

// type extWrapper struct {
// 	BytesExt
// 	InterfaceExt
// }

type bytesExtWrapper struct {
	interfaceExtFailer
	BytesExt
}

type interfaceExtWrapper struct {
	bytesExtFailer
	InterfaceExt
}

type extFailWrapper struct {
	bytesExtFailer
	interfaceExtFailer
}

type binaryEncodingType struct{}

func (binaryEncodingType) isBinary() bool { return true }

type textEncodingType struct{}

func (textEncodingType) isBinary() bool { return false }

// noBuiltInTypes is embedded into many types which do not support builtins
// e.g. msgpack, simple, cbor.

// type noBuiltInTypeChecker struct{}
// func (noBuiltInTypeChecker) IsBuiltinType(rt uintptr) bool { return false }
// type noBuiltInTypes struct{ noBuiltInTypeChecker }

type noBuiltInTypes struct{}

func (noBuiltInTypes) EncodeBuiltin(rt uintptr, v interface{}) {}
func (noBuiltInTypes) DecodeBuiltin(rt uintptr, v interface{}) {}

// type noStreamingCodec struct{}
// func (noStreamingCodec) CheckBreak() bool { return false }
// func (noStreamingCodec) hasElemSeparators() bool { return false }

// type noElemSeparators struct{}

// func (noElemSeparators) hasElemSeparators() (v bool)            { return }
// func (noElemSeparators) recreateEncDriver(e encDriver) (v bool) { return }

// bigenHelper.
// Users must already slice the x completely, because we will not reslice.
type bigenHelper struct {
	x []byte // must be correctly sliced to appropriate len. slicing is a cost.
	w *encWr
}

func (z bigenHelper) writeUint16(v uint16) {
	bigen.PutUint16(z.x, v)
	z.w.writeb(z.x)
}

func (z bigenHelper) writeUint32(v uint32) {
	bigen.PutUint32(z.x, v)
	z.w.writeb(z.x)
}

func (z bigenHelper) writeUint64(v uint64) {
	bigen.PutUint64(z.x, v)
	z.w.writeb(z.x)
}

type extTypeTagFn struct {
	rtid    uintptr
	rtidptr uintptr
	rt      reflect.Type
	tag     uint64
	ext     Ext
	// _       [1]uint64 // padding
}

type extHandle []extTypeTagFn

// AddExt registes an encode and decode function for a reflect.Type.
// To deregister an Ext, call AddExt with nil encfn and/or nil decfn.
//
// Deprecated: Use SetBytesExt or SetInterfaceExt on the Handle instead.
func (o *extHandle) AddExt(rt reflect.Type, tag byte,
	encfn func(reflect.Value) ([]byte, error),
	decfn func(reflect.Value, []byte) error) (err error) {
	if encfn == nil || decfn == nil {
		return o.SetExt(rt, uint64(tag), nil)
	}
	return o.SetExt(rt, uint64(tag), addExtWrapper{encfn, decfn})
}

// SetExt will set the extension for a tag and reflect.Type.
// Note that the type must be a named type, and specifically not a pointer or Interface.
// An error is returned if that is not honored.
// To Deregister an ext, call SetExt with nil Ext.
//
// Deprecated: Use SetBytesExt or SetInterfaceExt on the Handle instead.
func (o *extHandle) SetExt(rt reflect.Type, tag uint64, ext Ext) (err error) {
	// o is a pointer, because we may need to initialize it
	rk := rt.Kind()
	for rk == reflect.Ptr {
		rt = rt.Elem()
		rk = rt.Kind()
	}

	if rt.PkgPath() == "" || rk == reflect.Interface { // || rk == reflect.Ptr {
		return fmt.Errorf("codec.Handle.SetExt: Takes named type, not a pointer or interface: %v", rt)
	}

	rtid := rt2id(rt)
	switch rtid {
	case timeTypId, rawTypId, rawExtTypId:
		// all natively supported type, so cannot have an extension
		return // TODO: should we silently ignore, or return an error???
	}
	// if o == nil {
	// 	return errors.New("codec.Handle.SetExt: extHandle not initialized")
	// }
	o2 := *o
	// if o2 == nil {
	// 	return errors.New("codec.Handle.SetExt: extHandle not initialized")
	// }
	for i := range o2 {
		v := &o2[i]
		if v.rtid == rtid {
			v.tag, v.ext = tag, ext
			return
		}
	}
	rtidptr := rt2id(reflect.PtrTo(rt))
	*o = append(o2, extTypeTagFn{rtid, rtidptr, rt, tag, ext}) // , [1]uint64{}})
	return
}

func (o extHandle) getExt(rtid uintptr, check bool) (v *extTypeTagFn) {
	if !check {
		return
	}
	for i := range o {
		v = &o[i]
		if v.rtid == rtid || v.rtidptr == rtid {
			return
		}
	}
	return nil
}

func (o extHandle) getExtForTag(tag uint64) (v *extTypeTagFn) {
	for i := range o {
		v = &o[i]
		if v.tag == tag {
			return
		}
	}
	return nil
}

type intf2impl struct {
	rtid uintptr // for intf
	impl reflect.Type
	// _    [1]uint64 // padding // not-needed, as *intf2impl is never returned.
}

type intf2impls []intf2impl

// Intf2Impl maps an interface to an implementing type.
// This allows us support infering the concrete type
// and populating it when passed an interface.
// e.g. var v io.Reader can be decoded as a bytes.Buffer, etc.
//
// Passing a nil impl will clear the mapping.
func (o *intf2impls) Intf2Impl(intf, impl reflect.Type) (err error) {
	if impl != nil && !impl.Implements(intf) {
		return fmt.Errorf("Intf2Impl: %v does not implement %v", impl, intf)
	}
	rtid := rt2id(intf)
	o2 := *o
	for i := range o2 {
		v := &o2[i]
		if v.rtid == rtid {
			v.impl = impl
			return
		}
	}
	*o = append(o2, intf2impl{rtid, impl})
	return
}

func (o intf2impls) intf2impl(rtid uintptr) (rv reflect.Value) {
	for i := range o {
		v := &o[i]
		if v.rtid == rtid {
			if v.impl == nil {
				return
			}
			vkind := v.impl.Kind()
			if vkind == reflect.Ptr {
				return reflect.New(v.impl.Elem())
			}
			return rvZeroAddrK(v.impl, vkind)
		}
	}
	return
}

type structFieldInfoFlag uint8

const (
	_ structFieldInfoFlag = 1 << iota
	structFieldInfoFlagReady
	structFieldInfoFlagOmitEmpty
)

func (x *structFieldInfoFlag) flagSet(f structFieldInfoFlag) {
	*x = *x | f
}

func (x *structFieldInfoFlag) flagClr(f structFieldInfoFlag) {
	*x = *x &^ f
}

func (x structFieldInfoFlag) flagGet(f structFieldInfoFlag) bool {
	return x&f != 0
}

func (x structFieldInfoFlag) omitEmpty() bool {
	return x.flagGet(structFieldInfoFlagOmitEmpty)
}

func (x structFieldInfoFlag) ready() bool {
	return x.flagGet(structFieldInfoFlagReady)
}

type structFieldInfo struct {
	encName   string // encode name
	fieldName string // field name

	is  [maxLevelsEmbedding]uint16 // (recursive/embedded) field index in struct
	nis uint8                      // num levels of embedding. if 1, then it's not embedded.

	encNameAsciiAlphaNum bool // the encName only contains ascii alphabet and numbers
	structFieldInfoFlag
	// _ [1]byte // padding
}

// func (si *structFieldInfo) setToZeroValue(v reflect.Value) {
// 	if v, valid := si.field(v, false); valid {
// 		v.Set(reflect.Zero(v.Type()))
// 	}
// }

// rv returns the field of the struct.
// If anonymous, it returns an Invalid
func (si *structFieldInfo) field(v reflect.Value, update bool) (rv2 reflect.Value, valid bool) {
	// replicate FieldByIndex
	for i, x := range si.is {
		if uint8(i) == si.nis {
			break
		}
		if v, valid = baseStructRv(v, update); !valid {
			return
		}
		v = v.Field(int(x))
	}

	return v, true
}

// func (si *structFieldInfo) fieldval(v reflect.Value, update bool) reflect.Value {
// 	v, _ = si.field(v, update)
// 	return v
// }

func parseStructInfo(stag string) (toArray, omitEmpty bool, keytype valueType) {
	keytype = valueTypeString // default
	if stag == "" {
		return
	}
	for i, s := range strings.Split(stag, ",") {
		if i == 0 {
		} else {
			switch s {
			case "omitempty":
				omitEmpty = true
			case "toarray":
				toArray = true
			case "int":
				keytype = valueTypeInt
			case "uint":
				keytype = valueTypeUint
			case "float":
				keytype = valueTypeFloat
				// case "bool":
				// 	keytype = valueTypeBool
			case "string":
				keytype = valueTypeString
			}
		}
	}
	return
}

func (si *structFieldInfo) parseTag(stag string) {
	// if fname == "" {
	// 	panic(errNoFieldNameToStructFieldInfo)
	// }

	if stag == "" {
		return
	}
	for i, s := range strings.Split(stag, ",") {
		if i == 0 {
			if s != "" {
				si.encName = s
			}
		} else {
			switch s {
			case "omitempty":
				si.flagSet(structFieldInfoFlagOmitEmpty)
				// si.omitEmpty = true
				// case "toarray":
				// 	si.toArray = true
			}
		}
	}
}

type sfiSortedByEncName []*structFieldInfo

func (p sfiSortedByEncName) Len() int           { return len(p) }
func (p sfiSortedByEncName) Less(i, j int) bool { return p[uint(i)].encName < p[uint(j)].encName }
func (p sfiSortedByEncName) Swap(i, j int)      { p[uint(i)], p[uint(j)] = p[uint(j)], p[uint(i)] }

const structFieldNodeNumToCache = 4

type structFieldNodeCache struct {
	rv  [structFieldNodeNumToCache]reflect.Value
	idx [structFieldNodeNumToCache]uint32
	num uint8
}

func (x *structFieldNodeCache) get(key uint32) (fv reflect.Value, valid bool) {
	for i, k := range &x.idx {
		if uint8(i) == x.num {
			return // break
		}
		if key == k {
			return x.rv[i], true
		}
	}
	return
}

func (x *structFieldNodeCache) tryAdd(fv reflect.Value, key uint32) {
	if x.num < structFieldNodeNumToCache {
		x.rv[x.num] = fv
		x.idx[x.num] = key
		x.num++
		return
	}
}

type structFieldNode struct {
	v      reflect.Value
	cache2 structFieldNodeCache
	cache3 structFieldNodeCache
	update bool
}

func (x *structFieldNode) field(si *structFieldInfo) (fv reflect.Value) {
	// return si.fieldval(x.v, x.update)
	// Note: we only cache if nis=2 or nis=3 i.e. up to 2 levels of embedding
	// This mostly saves us time on the repeated calls to v.Elem, v.Field, etc.
	var valid bool
	switch si.nis {
	case 1:
		fv = x.v.Field(int(si.is[0]))
	case 2:
		if fv, valid = x.cache2.get(uint32(si.is[0])); valid {
			fv = fv.Field(int(si.is[1]))
			return
		}
		fv = x.v.Field(int(si.is[0]))
		if fv, valid = baseStructRv(fv, x.update); !valid {
			return
		}
		x.cache2.tryAdd(fv, uint32(si.is[0]))
		fv = fv.Field(int(si.is[1]))
	case 3:
		var key uint32 = uint32(si.is[0])<<16 | uint32(si.is[1])
		if fv, valid = x.cache3.get(key); valid {
			fv = fv.Field(int(si.is[2]))
			return
		}
		fv = x.v.Field(int(si.is[0]))
		if fv, valid = baseStructRv(fv, x.update); !valid {
			return
		}
		fv = fv.Field(int(si.is[1]))
		if fv, valid = baseStructRv(fv, x.update); !valid {
			return
		}
		x.cache3.tryAdd(fv, key)
		fv = fv.Field(int(si.is[2]))
	default:
		fv, _ = si.field(x.v, x.update)
	}
	return
}

func baseStructRv(v reflect.Value, update bool) (v2 reflect.Value, valid bool) {
	for v.Kind() == reflect.Ptr {
		if rvIsNil(v) {
			if !update {
				return
			}
			rvSetDirect(v, reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	return v, true
}

type tiflag uint32

const (
	_ tiflag = 1 << iota

	tiflagComparable

	tiflagIsZeroer
	tiflagIsZeroerPtr

	tiflagBinaryMarshaler
	tiflagBinaryMarshalerPtr

	tiflagBinaryUnmarshaler
	tiflagBinaryUnmarshalerPtr

	tiflagTextMarshaler
	tiflagTextMarshalerPtr

	tiflagTextUnmarshaler
	tiflagTextUnmarshalerPtr

	tiflagJsonMarshaler
	tiflagJsonMarshalerPtr

	tiflagJsonUnmarshaler
	tiflagJsonUnmarshalerPtr

	tiflagSelfer
	tiflagSelferPtr

	tiflagMissingFielder
	tiflagMissingFielderPtr

	// tiflag
	// tiflag
	// tiflag
	// tiflag
	// tiflag
	// tiflag
)

// typeInfo keeps static (non-changing readonly)information
// about each (non-ptr) type referenced in the encode/decode sequence.
//
// During an encode/decode sequence, we work as below:
//   - If base is a built in type, en/decode base value
//   - If base is registered as an extension, en/decode base value
//   - If type is binary(M/Unm)arshaler, call Binary(M/Unm)arshal method
//   - If type is text(M/Unm)arshaler, call Text(M/Unm)arshal method
//   - Else decode appropriately based on the reflect.Kind
type typeInfo struct {
	rt      reflect.Type
	elem    reflect.Type
	pkgpath string

	rtid uintptr
	// rv0  reflect.Value // saved zero value, used if immutableKind

	numMeth uint16 // number of methods
	kind    uint8
	chandir uint8

	anyOmitEmpty bool      // true if a struct, and any of the fields are tagged "omitempty"
	toArray      bool      // whether this (struct) type should be encoded as an array
	keyType      valueType // if struct, how is the field name stored in a stream? default is string
	mbs          bool      // base type (T or *T) is a MapBySlice

	// ---- cpu cache line boundary?
	sfiSort []*structFieldInfo // sorted. Used when enc/dec struct to map.
	sfiSrc  []*structFieldInfo // unsorted. Used when enc/dec struct to array.

	key reflect.Type

	// ---- cpu cache line boundary?
	// sfis         []structFieldInfo // all sfi, in src order, as created.
	sfiNamesSort []byte // all names, with indexes into the sfiSort

	// rv0 is the zero value for the type.
	// It is mostly beneficial for all non-reference kinds
	// i.e. all but map/chan/func/ptr/unsafe.pointer
	// so beneficial for intXX, bool, slices, structs, etc
	rv0 reflect.Value

	// format of marshal type fields below: [btj][mu]p? OR csp?

	// bm  bool // T is a binaryMarshaler
	// bmp bool // *T is a binaryMarshaler
	// bu  bool // T is a binaryUnmarshaler
	// bup bool // *T is a binaryUnmarshaler
	// tm  bool // T is a textMarshaler
	// tmp bool // *T is a textMarshaler
	// tu  bool // T is a textUnmarshaler
	// tup bool // *T is a textUnmarshaler

	// jm  bool // T is a jsonMarshaler
	// jmp bool // *T is a jsonMarshaler
	// ju  bool // T is a jsonUnmarshaler
	// jup bool // *T is a jsonUnmarshaler
	// cs  bool // T is a Selfer
	// csp bool // *T is a Selfer
	// mf  bool // T is a MissingFielder
	// mfp bool // *T is a MissingFielder

	// other flags, with individual bits representing if set.
	flags tiflag

	infoFieldOmitempty bool

	_ [3]byte   // padding
	_ [1]uint64 // padding
}

func (ti *typeInfo) isFlag(f tiflag) bool {
	return ti.flags&f != 0
}

func (ti *typeInfo) flag(when bool, f tiflag) *typeInfo {
	if when {
		ti.flags |= f
	}
	return ti
}

func (ti *typeInfo) indexForEncName(name []byte) (index int16) {
	var sn []byte
	if len(name)+2 <= 32 {
		var buf [32]byte // should not escape to heap
		sn = buf[:len(name)+2]
	} else {
		sn = make([]byte, len(name)+2)
	}
	copy(sn[1:], name)
	sn[0], sn[len(sn)-1] = tiSep2(name), 0xff
	j := bytes.Index(ti.sfiNamesSort, sn)
	if j < 0 {
		return -1
	}
	index = int16(uint16(ti.sfiNamesSort[j+len(sn)+1]) | uint16(ti.sfiNamesSort[j+len(sn)])<<8)
	return
}

type rtid2ti struct {
	rtid uintptr
	ti   *typeInfo
}

// TypeInfos caches typeInfo for each type on first inspection.
//
// It is configured with a set of tag keys, which are used to get
// configuration for the type.
type TypeInfos struct {
	// infos: formerly map[uintptr]*typeInfo, now *[]rtid2ti, 2 words expected
	infos atomicTypeInfoSlice
	mu    sync.Mutex
	_     uint64 // padding (cache-aligned)
	tags  []string
	_     uint64 // padding (cache-aligned)
}

// NewTypeInfos creates a TypeInfos given a set of struct tags keys.
//
// This allows users customize the struct tag keys which contain configuration
// of their types.
func NewTypeInfos(tags []string) *TypeInfos {
	return &TypeInfos{tags: tags}
}

func (x *TypeInfos) structTag(t reflect.StructTag) (s string) {
	// check for tags: codec, json, in that order.
	// this allows seamless support for many configured structs.
	for _, x := range x.tags {
		s = t.Get(x)
		if s != "" {
			return s
		}
	}
	return
}

func findTypeInfo(s []rtid2ti, rtid uintptr) (i uint, ti *typeInfo) {
	// binary search. adapted from sort/search.go.
	// Note: we use goto (instead of for loop) so this can be inlined.

	// if sp == nil {
	// 	return -1, nil
	// }
	// s := *sp

	// h, i, j := 0, 0, len(s)
	var h uint // var h, i uint
	var j = uint(len(s))
LOOP:
	if i < j {
		h = i + (j-i)/2
		if s[h].rtid < rtid {
			i = h + 1
		} else {
			j = h
		}
		goto LOOP
	}
	if i < uint(len(s)) && s[i].rtid == rtid {
		ti = s[i].ti
	}
	return
}

func (x *TypeInfos) get(rtid uintptr, rt reflect.Type) (pti *typeInfo) {
	sp := x.infos.load()
	if sp != nil {
		_, pti = findTypeInfo(sp, rtid)
		if pti != nil {
			return
		}
	}

	rk := rt.Kind()

	if rk == reflect.Ptr { // || (rk == reflect.Interface && rtid != intfTypId) {
		panicv.errorf("invalid kind passed to TypeInfos.get: %v - %v", rk, rt)
	}

	// do not hold lock while computing this.
	// it may lead to duplication, but that's ok.
	ti := typeInfo{
		rt:      rt,
		rtid:    rtid,
		kind:    uint8(rk),
		pkgpath: rt.PkgPath(),
		keyType: valueTypeString, // default it - so it's never 0
	}
	ti.rv0 = reflect.Zero(rt)

	// ti.comparable = rt.Comparable()
	ti.numMeth = uint16(rt.NumMethod())

	var b1, b2 bool
	b1, b2 = implIntf(rt, binaryMarshalerTyp)
	ti.flag(b1, tiflagBinaryMarshaler).flag(b2, tiflagBinaryMarshalerPtr)
	b1, b2 = implIntf(rt, binaryUnmarshalerTyp)
	ti.flag(b1, tiflagBinaryUnmarshaler).flag(b2, tiflagBinaryUnmarshalerPtr)
	b1, b2 = implIntf(rt, textMarshalerTyp)
	ti.flag(b1, tiflagTextMarshaler).flag(b2, tiflagTextMarshalerPtr)
	b1, b2 = implIntf(rt, textUnmarshalerTyp)
	ti.flag(b1, tiflagTextUnmarshaler).flag(b2, tiflagTextUnmarshalerPtr)
	b1, b2 = implIntf(rt, jsonMarshalerTyp)
	ti.flag(b1, tiflagJsonMarshaler).flag(b2, tiflagJsonMarshalerPtr)
	b1, b2 = implIntf(rt, jsonUnmarshalerTyp)
	ti.flag(b1, tiflagJsonUnmarshaler).flag(b2, tiflagJsonUnmarshalerPtr)
	b1, b2 = implIntf(rt, selferTyp)
	ti.flag(b1, tiflagSelfer).flag(b2, tiflagSelferPtr)
	b1, b2 = implIntf(rt, missingFielderTyp)
	ti.flag(b1, tiflagMissingFielder).flag(b2, tiflagMissingFielderPtr)
	b1, b2 = implIntf(rt, iszeroTyp)
	ti.flag(b1, tiflagIsZeroer).flag(b2, tiflagIsZeroerPtr)
	b1 = rt.Comparable()
	ti.flag(b1, tiflagComparable)

	switch rk {
	case reflect.Struct:
		var omitEmpty bool
		if f, ok := rt.FieldByName(structInfoFieldName); ok {
			ti.toArray, omitEmpty, ti.keyType = parseStructInfo(x.structTag(f.Tag))
			ti.infoFieldOmitempty = omitEmpty
		} else {
			ti.keyType = valueTypeString
		}
		pp, pi := &pool4tiload, pool4tiload.Get() // pool.tiLoad()
		pv := pi.(*typeInfoLoadArray)
		pv.etypes[0] = ti.rtid
		// vv := typeInfoLoad{pv.fNames[:0], pv.encNames[:0], pv.etypes[:1], pv.sfis[:0]}
		vv := typeInfoLoad{pv.etypes[:1], pv.sfis[:0]}
		x.rget(rt, rtid, omitEmpty, nil, &vv)
		// ti.sfis = vv.sfis
		ti.sfiSrc, ti.sfiSort, ti.sfiNamesSort, ti.anyOmitEmpty = rgetResolveSFI(rt, vv.sfis, pv)
		pp.Put(pi)
	case reflect.Map:
		ti.elem = rt.Elem()
		ti.key = rt.Key()
	case reflect.Slice:
		ti.mbs, _ = implIntf(rt, mapBySliceTyp)
		ti.elem = rt.Elem()
	case reflect.Chan:
		ti.elem = rt.Elem()
		ti.chandir = uint8(rt.ChanDir())
	case reflect.Array, reflect.Ptr:
		ti.elem = rt.Elem()
	}
	// sfi = sfiSrc

	x.mu.Lock()
	sp = x.infos.load()
	var sp2 []rtid2ti
	if sp == nil {
		pti = &ti
		sp2 = []rtid2ti{{rtid, pti}}
		x.infos.store(sp2)
	} else {
		var idx uint
		idx, pti = findTypeInfo(sp, rtid)
		if pti == nil {
			pti = &ti
			sp2 = make([]rtid2ti, len(sp)+1)
			copy(sp2, sp[:idx])
			copy(sp2[idx+1:], sp[idx:])
			sp2[idx] = rtid2ti{rtid, pti}
			x.infos.store(sp2)
		}
	}
	x.mu.Unlock()
	return
}

func (x *TypeInfos) rget(rt reflect.Type, rtid uintptr, omitEmpty bool,
	indexstack []uint16, pv *typeInfoLoad) {
	// Read up fields and store how to access the value.
	//
	// It uses go's rules for message selectors,
	// which say that the field with the shallowest depth is selected.
	//
	// Note: we consciously use slices, not a map, to simulate a set.
	//       Typically, types have < 16 fields,
	//       and iteration using equals is faster than maps there
	flen := rt.NumField()
	if flen > (1<<maxLevelsEmbedding - 1) {
		panicv.errorf("codec: types with > %v fields are not supported - has %v fields",
			(1<<maxLevelsEmbedding - 1), flen)
	}
	// pv.sfis = make([]structFieldInfo, flen)
LOOP:
	for j, jlen := uint16(0), uint16(flen); j < jlen; j++ {
		f := rt.Field(int(j))
		fkind := f.Type.Kind()
		// skip if a func type, or is unexported, or structTag value == "-"
		switch fkind {
		case reflect.Func, reflect.Complex64, reflect.Complex128, reflect.UnsafePointer:
			continue LOOP
		}

		isUnexported := f.PkgPath != ""
		if isUnexported && !f.Anonymous {
			continue
		}
		stag := x.structTag(f.Tag)
		if stag == "-" {
			continue
		}
		var si structFieldInfo
		var parsed bool
		// if anonymous and no struct tag (or it's blank),
		// and a struct (or pointer to struct), inline it.
		if f.Anonymous && fkind != reflect.Interface {
			// ^^ redundant but ok: per go spec, an embedded pointer type cannot be to an interface
			ft := f.Type
			isPtr := ft.Kind() == reflect.Ptr
			for ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			isStruct := ft.Kind() == reflect.Struct

			// Ignore embedded fields of unexported non-struct types.
			// Also, from go1.10, ignore pointers to unexported struct types
			// because unmarshal cannot assign a new struct to an unexported field.
			// See https://golang.org/issue/21357
			if (isUnexported && !isStruct) || (!allowSetUnexportedEmbeddedPtr && isUnexported && isPtr) {
				continue
			}
			doInline := stag == ""
			if !doInline {
				si.parseTag(stag)
				parsed = true
				doInline = si.encName == ""
				// doInline = si.isZero()
			}
			if doInline && isStruct {
				// if etypes contains this, don't call rget again (as fields are already seen here)
				ftid := rt2id(ft)
				// We cannot recurse forever, but we need to track other field depths.
				// So - we break if we see a type twice (not the first time).
				// This should be sufficient to handle an embedded type that refers to its
				// owning type, which then refers to its embedded type.
				processIt := true
				numk := 0
				for _, k := range pv.etypes {
					if k == ftid {
						numk++
						if numk == rgetMaxRecursion {
							processIt = false
							break
						}
					}
				}
				if processIt {
					pv.etypes = append(pv.etypes, ftid)
					indexstack2 := make([]uint16, len(indexstack)+1)
					copy(indexstack2, indexstack)
					indexstack2[len(indexstack)] = j
					// indexstack2 := append(append(make([]int, 0, len(indexstack)+4), indexstack...), j)
					x.rget(ft, ftid, omitEmpty, indexstack2, pv)
				}
				continue
			}
		}

		// after the anonymous dance: if an unexported field, skip
		if isUnexported {
			continue
		}

		if f.Name == "" {
			panic(errNoFieldNameToStructFieldInfo)
		}

		// pv.fNames = append(pv.fNames, f.Name)
		// if si.encName == "" {

		if !parsed {
			si.encName = f.Name
			si.parseTag(stag)
			parsed = true
		} else if si.encName == "" {
			si.encName = f.Name
		}
		si.encNameAsciiAlphaNum = true
		for i := len(si.encName) - 1; i >= 0; i-- { // bounds-check elimination
			b := si.encName[i]
			if (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') {
				continue
			}
			si.encNameAsciiAlphaNum = false
			break
		}
		si.fieldName = f.Name
		si.flagSet(structFieldInfoFlagReady)

		// pv.encNames = append(pv.encNames, si.encName)

		// si.ikind = int(f.Type.Kind())
		if len(indexstack) > maxLevelsEmbedding-1 {
			panicv.errorf("codec: only supports up to %v depth of embedding - type has %v depth",
				maxLevelsEmbedding-1, len(indexstack))
		}
		si.nis = uint8(len(indexstack)) + 1
		copy(si.is[:], indexstack)
		si.is[len(indexstack)] = j

		if omitEmpty {
			si.flagSet(structFieldInfoFlagOmitEmpty)
		}
		pv.sfis = append(pv.sfis, si)
	}
}

func tiSep(name string) uint8 {
	// (xn[0]%64) // (between 192-255 - outside ascii BMP)
	// return 0xfe - (name[0] & 63)
	// return 0xfe - (name[0] & 63) - uint8(len(name))
	// return 0xfe - (name[0] & 63) - uint8(len(name)&63)
	// return ((0xfe - (name[0] & 63)) & 0xf8) | (uint8(len(name) & 0x07))
	return 0xfe - (name[0] & 63) - uint8(len(name)&63)
}

func tiSep2(name []byte) uint8 {
	return 0xfe - (name[0] & 63) - uint8(len(name)&63)
}

// resolves the struct field info got from a call to rget.
// Returns a trimmed, unsorted and sorted []*structFieldInfo.
func rgetResolveSFI(rt reflect.Type, x []structFieldInfo, pv *typeInfoLoadArray) (
	y, z []*structFieldInfo, ss []byte, anyOmitEmpty bool) {
	sa := pv.sfiidx[:0]
	sn := pv.b[:]
	n := len(x)

	var xn string
	var ui uint16
	var sep byte

	for i := range x {
		ui = uint16(i)
		xn = x[i].encName // fieldName or encName? use encName for now.
		if len(xn)+2 > cap(sn) {
			sn = make([]byte, len(xn)+2)
		} else {
			sn = sn[:len(xn)+2]
		}
		// use a custom sep, so that misses are less frequent,
		// since the sep (first char in search) is as unique as first char in field name.
		sep = tiSep(xn)
		sn[0], sn[len(sn)-1] = sep, 0xff
		copy(sn[1:], xn)
		j := bytes.Index(sa, sn)
		if j == -1 {
			sa = append(sa, sep)
			sa = append(sa, xn...)
			sa = append(sa, 0xff, byte(ui>>8), byte(ui))
		} else {
			index := uint16(sa[j+len(sn)+1]) | uint16(sa[j+len(sn)])<<8
			// one of them must be cleared (reset to nil),
			// and the index updated appropriately
			i2clear := ui                // index to be cleared
			if x[i].nis < x[index].nis { // this one is shallower
				// update the index to point to this later one.
				sa[j+len(sn)], sa[j+len(sn)+1] = byte(ui>>8), byte(ui)
				// clear the earlier one, as this later one is shallower.
				i2clear = index
			}
			if x[i2clear].ready() {
				x[i2clear].flagClr(structFieldInfoFlagReady)
				n--
			}
		}
	}

	var w []structFieldInfo
	sharingArray := len(x) <= typeInfoLoadArraySfisLen // sharing array with typeInfoLoadArray
	if sharingArray {
		w = make([]structFieldInfo, n)
	}

	// remove all the nils (non-ready)
	y = make([]*structFieldInfo, n)
	n = 0
	var sslen int
	for i := range x {
		if !x[i].ready() {
			continue
		}
		if !anyOmitEmpty && x[i].omitEmpty() {
			anyOmitEmpty = true
		}
		if sharingArray {
			w[n] = x[i]
			y[n] = &w[n]
		} else {
			y[n] = &x[i]
		}
		sslen = sslen + len(x[i].encName) + 4
		n++
	}
	if n != len(y) {
		panicv.errorf("failure reading struct %v - expecting %d of %d valid fields, got %d",
			rt, len(y), len(x), n)
	}

	z = make([]*structFieldInfo, len(y))
	copy(z, y)
	sort.Sort(sfiSortedByEncName(z))

	sharingArray = len(sa) <= typeInfoLoadArraySfiidxLen
	if sharingArray {
		ss = make([]byte, 0, sslen)
	} else {
		ss = sa[:0] // reuse the newly made sa array if necessary
	}
	for i := range z {
		xn = z[i].encName
		sep = tiSep(xn)
		ui = uint16(i)
		ss = append(ss, sep)
		ss = append(ss, xn...)
		ss = append(ss, 0xff, byte(ui>>8), byte(ui))
	}
	return
}

func implIntf(rt, iTyp reflect.Type) (base bool, indir bool) {
	return rt.Implements(iTyp), reflect.PtrTo(rt).Implements(iTyp)
}

// isEmptyStruct is only called from isEmptyValue, and checks if a struct is empty:
//    - does it implement IsZero() bool
//    - is it comparable, and can i compare directly using ==
//    - if checkStruct, then walk through the encodable fields
//      and check if they are empty or not.
func isEmptyStruct(v reflect.Value, tinfos *TypeInfos, deref, checkStruct bool) bool {
	// v is a struct kind - no need to check again.
	// We only check isZero on a struct kind, to reduce the amount of times
	// that we lookup the rtid and typeInfo for each type as we walk the tree.

	vt := v.Type()
	rtid := rt2id(vt)
	if tinfos == nil {
		tinfos = defTypeInfos
	}
	ti := tinfos.get(rtid, vt)
	if ti.rtid == timeTypId {
		return rv2i(v).(time.Time).IsZero()
	}
	if ti.isFlag(tiflagIsZeroerPtr) && v.CanAddr() {
		return rv2i(v.Addr()).(isZeroer).IsZero()
	}
	if ti.isFlag(tiflagIsZeroer) {
		return rv2i(v).(isZeroer).IsZero()
	}
	if ti.isFlag(tiflagComparable) {
		return rv2i(v) == rv2i(reflect.Zero(vt))
	}
	if !checkStruct {
		return false
	}
	// We only care about what we can encode/decode,
	// so that is what we use to check omitEmpty.
	for _, si := range ti.sfiSrc {
		sfv, valid := si.field(v, false)
		if valid && !isEmptyValue(sfv, tinfos, deref, checkStruct) {
			return false
		}
	}
	return true
}

// func roundFloat(x float64) float64 {
// 	t := math.Trunc(x)
// 	if math.Abs(x-t) >= 0.5 {
// 		return t + math.Copysign(1, x)
// 	}
// 	return t
// }

func panicToErr(h errDecorator, err *error) {
	// Note: This method MUST be called directly from defer i.e. defer panicToErr ...
	// else it seems the recover is not fully handled
	if recoverPanicToErr {
		if x := recover(); x != nil {
			// fmt.Printf("panic'ing with: %v\n", x)
			// debug.PrintStack()
			panicValToErr(h, x, err)
		}
	}
}

func isSliceBoundsError(s string) bool {
	return strings.Index(s, "index out of range") != -1 ||
		strings.Index(s, "slice bounds out of range") != -1
}

func panicValToErr(h errDecorator, v interface{}, err *error) {
	d, dok := h.(*Decoder)
	switch xerr := v.(type) {
	case nil:
	case error:
		switch xerr {
		case nil:
		case io.EOF, io.ErrUnexpectedEOF, errEncoderNotInitialized, errDecoderNotInitialized:
			// treat as special (bubble up)
			*err = xerr
		default:
			if dok && d.bytes && isSliceBoundsError(xerr.Error()) {
				*err = io.EOF
			} else {
				h.wrapErr(xerr, err)
			}
		}
	case string:
		if xerr != "" {
			if dok && d.bytes && isSliceBoundsError(xerr) {
				*err = io.EOF
			} else {
				h.wrapErr(xerr, err)
			}
		}
	case fmt.Stringer:
		if xerr != nil {
			h.wrapErr(xerr, err)
		}
	default:
		h.wrapErr(v, err)
	}
}

func isImmutableKind(k reflect.Kind) (v bool) {
	// return immutableKindsSet[k]
	// since we know reflect.Kind is in range 0..31, then use the k%32 == k constraint
	return immutableKindsSet[k%reflect.Kind(len(immutableKindsSet))] // bounds-check-elimination
}

func usableByteSlice(bs []byte, slen int) []byte {
	if cap(bs) >= slen {
		if bs == nil {
			return []byte{}
		}
		return bs[:slen]
	}
	return make([]byte, slen)
}

// ----

type codecFnInfo struct {
	ti    *typeInfo
	xfFn  Ext
	xfTag uint64
	seq   seqType
	addrD bool
	addrF bool // if addrD, this says whether decode function can take a value or a ptr
	addrE bool
}

// codecFn encapsulates the captured variables and the encode function.
// This way, we only do some calculations one times, and pass to the
// code block that should be called (encapsulated in a function)
// instead of executing the checks every time.
type codecFn struct {
	i  codecFnInfo
	fe func(*Encoder, *codecFnInfo, reflect.Value)
	fd func(*Decoder, *codecFnInfo, reflect.Value)
	_  [1]uint64 // padding (cache-aligned)
}

type codecRtidFn struct {
	rtid uintptr
	fn   *codecFn
}

func makeExt(ext interface{}) Ext {
	if ext == nil {
		return &extFailWrapper{}
	}
	switch t := ext.(type) {
	case nil:
		return &extFailWrapper{}
	case Ext:
		return t
	case BytesExt:
		return &bytesExtWrapper{BytesExt: t}
	case InterfaceExt:
		return &interfaceExtWrapper{InterfaceExt: t}
	}
	return &extFailWrapper{}
}

func baseRV(v interface{}) (rv reflect.Value) {
	for rv = rv4i(v); rv.Kind() == reflect.Ptr; rv = rv.Elem() {
	}
	return
}

// func newAddressableRV(t reflect.Type, k reflect.Kind) reflect.Value {
// 	if k == reflect.Ptr {
// 		return reflect.New(t.Elem()) // this is not addressable???
// 	}
// 	return reflect.New(t).Elem()
// }

// func newAddressableRV(t reflect.Type) reflect.Value {
// 	return reflect.New(t).Elem()
// }

// ----

// these "checkOverflow" functions must be inlinable, and not call anybody.
// Overflow means that the value cannot be represented without wrapping/overflow.
// Overflow=false does not mean that the value can be represented without losing precision
// (especially for floating point).

type checkOverflow struct{}

// func (checkOverflow) Float16(f float64) (overflow bool) {
// 	panicv.errorf("unimplemented")
// 	if f < 0 {
// 		f = -f
// 	}
// 	return math.MaxFloat32 < f && f <= math.MaxFloat64
// }

func (checkOverflow) Float32(v float64) (overflow bool) {
	if v < 0 {
		v = -v
	}
	return math.MaxFloat32 < v && v <= math.MaxFloat64
}
func (checkOverflow) Uint(v uint64, bitsize uint8) (overflow bool) {
	if bitsize == 0 || bitsize >= 64 || v == 0 {
		return
	}
	if trunc := (v << (64 - bitsize)) >> (64 - bitsize); v != trunc {
		overflow = true
	}
	return
}
func (checkOverflow) Int(v int64, bitsize uint8) (overflow bool) {
	if bitsize == 0 || bitsize >= 64 || v == 0 {
		return
	}
	if trunc := (v << (64 - bitsize)) >> (64 - bitsize); v != trunc {
		overflow = true
	}
	return
}
func (checkOverflow) SignedInt(v uint64) (overflow bool) {
	//e.g. -127 to 128 for int8
	pos := (v >> 63) == 0
	ui2 := v & 0x7fffffffffffffff
	if pos {
		if ui2 > math.MaxInt64 {
			overflow = true
		}
	} else {
		if ui2 > math.MaxInt64-1 {
			overflow = true
		}
	}
	return
}

func (x checkOverflow) Float32V(v float64) float64 {
	if x.Float32(v) {
		panicv.errorf("float32 overflow: %v", v)
	}
	return v
}
func (x checkOverflow) UintV(v uint64, bitsize uint8) uint64 {
	if x.Uint(v, bitsize) {
		panicv.errorf("uint64 overflow: %v", v)
	}
	return v
}
func (x checkOverflow) IntV(v int64, bitsize uint8) int64 {
	if x.Int(v, bitsize) {
		panicv.errorf("int64 overflow: %v", v)
	}
	return v
}
func (x checkOverflow) SignedIntV(v uint64) int64 {
	if x.SignedInt(v) {
		panicv.errorf("uint64 to int64 overflow: %v", v)
	}
	return int64(v)
}

// ------------------ FLOATING POINT -----------------

func isNaN64(f float64) bool { return f != f }
func isNaN32(f float32) bool { return f != f }
func abs32(f float32) float32 {
	return math.Float32frombits(math.Float32bits(f) &^ (1 << 31))
}

// Per go spec, floats are represented in memory as
// IEEE single or double precision floating point values.
//
// We also looked at the source for stdlib math/modf.go,
// reviewed https://github.com/chewxy/math32
// and read wikipedia documents describing the formats.
//
// It became clear that we could easily look at the bits to determine
// whether any fraction exists.
//
// This is all we need for now.

func noFrac64(f float64) (v bool) {
	x := math.Float64bits(f)
	e := uint64(x>>52)&0x7FF - 1023 // uint(x>>shift)&mask - bias
	// clear top 12+e bits, the integer part; if the rest is 0, then no fraction.
	if e < 52 {
		// return x&((1<<64-1)>>(12+e)) == 0
		return x<<(12+e) == 0
	}
	return
}

func noFrac32(f float32) (v bool) {
	x := math.Float32bits(f)
	e := uint32(x>>23)&0xFF - 127 // uint(x>>shift)&mask - bias
	// clear top 9+e bits, the integer part; if the rest is 0, then no fraction.
	if e < 23 {
		// return x&((1<<32-1)>>(9+e)) == 0
		return x<<(9+e) == 0
	}
	return
}

// func noFrac(f float64) bool {
// 	_, frac := math.Modf(float64(f))
// 	return frac == 0
// }

// -----------------------

type ioFlusher interface {
	Flush() error
}

type ioPeeker interface {
	Peek(int) ([]byte, error)
}

type ioBuffered interface {
	Buffered() int
}

// -----------------------

type sfiRv struct {
	v *structFieldInfo
	r reflect.Value
}

// -----------------

type set []interface{}

func (s *set) add(v interface{}) (exists bool) {
	// e.ci is always nil, or len >= 1
	x := *s

	if x == nil {
		x = make([]interface{}, 1, 8)
		x[0] = v
		*s = x
		return
	}
	// typically, length will be 1. make this perform.
	if len(x) == 1 {
		if j := x[0]; j == 0 {
			x[0] = v
		} else if j == v {
			exists = true
		} else {
			x = append(x, v)
			*s = x
		}
		return
	}
	// check if it exists
	for _, j := range x {
		if j == v {
			exists = true
			return
		}
	}
	// try to replace a "deleted" slot
	for i, j := range x {
		if j == 0 {
			x[i] = v
			return
		}
	}
	// if unable to replace deleted slot, just append it.
	x = append(x, v)
	*s = x
	return
}

func (s *set) remove(v interface{}) (exists bool) {
	x := *s
	if len(x) == 0 {
		return
	}
	if len(x) == 1 {
		if x[0] == v {
			x[0] = 0
		}
		return
	}
	for i, j := range x {
		if j == v {
			exists = true
			x[i] = 0 // set it to 0, as way to delete it.
			// copy(x[i:], x[i+1:])
			// x = x[:len(x)-1]
			return
		}
	}
	return
}

// ------

// bitset types are better than [256]bool, because they permit the whole
// bitset array being on a single cache line and use less memory.
//
// Also, since pos is a byte (0-255), there's no bounds checks on indexing (cheap).
//
// We previously had bitset128 [16]byte, and bitset32 [4]byte, but those introduces
// bounds checking, so we discarded them, and everyone uses bitset256.
//
// given x > 0 and n > 0 and x is exactly 2^n, then pos/x === pos>>n AND pos%x === pos&(x-1).
// consequently, pos/32 === pos>>5, pos/16 === pos>>4, pos/8 === pos>>3, pos%8 == pos&7

type bitset256 [32]byte

func (x *bitset256) check(pos byte) uint8 {
	return x[pos>>3] & (1 << (pos & 7))
}

func (x *bitset256) isset(pos byte) bool {
	return x.check(pos) != 0
	// return x[pos>>3]&(1<<(pos&7)) != 0
}

// func (x *bitset256) issetv(pos byte) byte {
// 	return x[pos>>3] & (1 << (pos & 7))
// }

func (x *bitset256) set(pos byte) {
	x[pos>>3] |= (1 << (pos & 7))
}

type bitset32 uint32

func (x bitset32) set(pos byte) bitset32 {
	return x | (1 << pos)
}

func (x bitset32) check(pos byte) uint32 {
	return uint32(x) & (1 << pos)
}
func (x bitset32) isset(pos byte) bool {
	return x.check(pos) != 0
	// return x&(1<<pos) != 0
}

// func (x *bitset256) unset(pos byte) {
// 	x[pos>>3] &^= (1 << (pos & 7))
// }

// type bit2set256 [64]byte

// func (x *bit2set256) set(pos byte, v1, v2 bool) {
// 	var pos2 uint8 = (pos & 3) << 1 // returning 0, 2, 4 or 6
// 	if v1 {
// 		x[pos>>2] |= 1 << (pos2 + 1)
// 	}
// 	if v2 {
// 		x[pos>>2] |= 1 << pos2
// 	}
// }
// func (x *bit2set256) get(pos byte) uint8 {
// 	var pos2 uint8 = (pos & 3) << 1     // returning 0, 2, 4 or 6
// 	return x[pos>>2] << (6 - pos2) >> 6 // 11000000 -> 00000011
// }

// ------------

// type strBytes struct {
// 	s string
// 	b []byte
// 	// i uint16
// }

// ------------

// type pooler struct {
// 	// function-scoped pooled resources
// 	tiload                                      sync.Pool // for type info loading
// 	sfiRv8, sfiRv16, sfiRv32, sfiRv64, sfiRv128 sync.Pool // for struct encoding

// 	// lifetime-scoped pooled resources
// 	// dn                                 sync.Pool // for decNaked
// 	buf256, buf1k, buf2k, buf4k, buf8k, buf16k, buf32k sync.Pool // for [N]byte

// 	mapStrU16, mapU16Str, mapU16Bytes sync.Pool // for Binc
// 	// mapU16StrBytes sync.Pool // for Binc
// }

// func (p *pooler) init() {
// 	p.tiload.New = func() interface{} { return new(typeInfoLoadArray) }

// 	p.sfiRv8.New = func() interface{} { return new([8]sfiRv) }
// 	p.sfiRv16.New = func() interface{} { return new([16]sfiRv) }
// 	p.sfiRv32.New = func() interface{} { return new([32]sfiRv) }
// 	p.sfiRv64.New = func() interface{} { return new([64]sfiRv) }
// 	p.sfiRv128.New = func() interface{} { return new([128]sfiRv) }

// 	// p.dn.New = func() interface{} { x := new(decNaked); x.init(); return x }

// 	p.buf256.New = func() interface{} { return new([256]byte) }
// 	p.buf1k.New = func() interface{} { return new([1 * 1024]byte) }
// 	p.buf2k.New = func() interface{} { return new([2 * 1024]byte) }
// 	p.buf4k.New = func() interface{} { return new([4 * 1024]byte) }
// 	p.buf8k.New = func() interface{} { return new([8 * 1024]byte) }
// 	p.buf16k.New = func() interface{} { return new([16 * 1024]byte) }
// 	p.buf32k.New = func() interface{} { return new([32 * 1024]byte) }
// 	// p.buf64k.New = func() interface{} { return new([64 * 1024]byte) }

// 	p.mapStrU16.New = func() interface{} { return make(map[string]uint16, 16) }
// 	p.mapU16Str.New = func() interface{} { return make(map[uint16]string, 16) }
// 	p.mapU16Bytes.New = func() interface{} { return make(map[uint16][]byte, 16) }
// 	// p.mapU16StrBytes.New = func() interface{} { return make(map[uint16]strBytes, 16) }
// }

// func (p *pooler) sfiRv8() (sp *sync.Pool, v interface{}) {
// 	return &p.strRv8, p.strRv8.Get()
// }
// func (p *pooler) sfiRv16() (sp *sync.Pool, v interface{}) {
// 	return &p.strRv16, p.strRv16.Get()
// }
// func (p *pooler) sfiRv32() (sp *sync.Pool, v interface{}) {
// 	return &p.strRv32, p.strRv32.Get()
// }
// func (p *pooler) sfiRv64() (sp *sync.Pool, v interface{}) {
// 	return &p.strRv64, p.strRv64.Get()
// }
// func (p *pooler) sfiRv128() (sp *sync.Pool, v interface{}) {
// 	return &p.strRv128, p.strRv128.Get()
// }

// func (p *pooler) bytes1k() (sp *sync.Pool, v interface{}) {
// 	return &p.buf1k, p.buf1k.Get()
// }
// func (p *pooler) bytes2k() (sp *sync.Pool, v interface{}) {
// 	return &p.buf2k, p.buf2k.Get()
// }
// func (p *pooler) bytes4k() (sp *sync.Pool, v interface{}) {
// 	return &p.buf4k, p.buf4k.Get()
// }
// func (p *pooler) bytes8k() (sp *sync.Pool, v interface{}) {
// 	return &p.buf8k, p.buf8k.Get()
// }
// func (p *pooler) bytes16k() (sp *sync.Pool, v interface{}) {
// 	return &p.buf16k, p.buf16k.Get()
// }
// func (p *pooler) bytes32k() (sp *sync.Pool, v interface{}) {
// 	return &p.buf32k, p.buf32k.Get()
// }
// func (p *pooler) bytes64k() (sp *sync.Pool, v interface{}) {
// 	return &p.buf64k, p.buf64k.Get()
// }

// func (p *pooler) tiLoad() (sp *sync.Pool, v interface{}) {
// 	return &p.tiload, p.tiload.Get()
// }

// func (p *pooler) decNaked() (sp *sync.Pool, v interface{}) {
// 	return &p.dn, p.dn.Get()
// }

// func (p *pooler) decNaked() (v *decNaked, f func(*decNaked) ) {
// 	sp := &(p.dn)
// 	vv := sp.Get()
// 	return vv.(*decNaked), func(x *decNaked) { sp.Put(vv) }
// }
// func (p *pooler) decNakedGet() (v interface{}) {
// 	return p.dn.Get()
// }
// func (p *pooler) tiLoadGet() (v interface{}) {
// 	return p.tiload.Get()
// }
// func (p *pooler) decNakedPut(v interface{}) {
// 	p.dn.Put(v)
// }
// func (p *pooler) tiLoadPut(v interface{}) {
// 	p.tiload.Put(v)
// }

// ----------------------------------------------------

type panicHdl struct{}

func (panicHdl) errorv(err error) {
	if err != nil {
		panic(err)
	}
}

func (panicHdl) errorstr(message string) {
	if message != "" {
		panic(message)
	}
}

func (panicHdl) errorf(format string, params ...interface{}) {
	if len(params) != 0 {
		panic(fmt.Sprintf(format, params...))
	}
	if len(params) == 0 {
		panic(format)
	}
	panic("undefined error")
}

// ----------------------------------------------------

type errDecorator interface {
	wrapErr(in interface{}, out *error)
}

type errDecoratorDef struct{}

func (errDecoratorDef) wrapErr(v interface{}, e *error) { *e = fmt.Errorf("%v", v) }

// ----------------------------------------------------

type must struct{}

func (must) String(s string, err error) string {
	if err != nil {
		panicv.errorv(err)
	}
	return s
}
func (must) Int(s int64, err error) int64 {
	if err != nil {
		panicv.errorv(err)
	}
	return s
}
func (must) Uint(s uint64, err error) uint64 {
	if err != nil {
		panicv.errorv(err)
	}
	return s
}
func (must) Float(s float64, err error) float64 {
	if err != nil {
		panicv.errorv(err)
	}
	return s
}

// -------------------

/*

type pooler struct {
	pool  *sync.Pool
	poolv interface{}
}

func (z *pooler) end() {
	if z.pool != nil {
		z.pool.Put(z.poolv)
		z.pool, z.poolv = nil, nil
	}
}

// -------------------

const bytesBufPoolerMaxSize = 32 * 1024

type bytesBufPooler struct {
	pooler
}

func (z *bytesBufPooler) capacity() (c int) {
	switch z.pool {
	case nil:
	case &pool4buf256:
		c = 256
	case &pool4buf1k:
		c = 1024
	case &pool4buf2k:
		c = 2 * 1024
	case &pool4buf4k:
		c = 4 * 1024
	case &pool4buf8k:
		c = 8 * 1024
	case &pool4buf16k:
		c = 16 * 1024
	case &pool4buf32k:
		c = 32 * 1024
	}
	return
}

// func (z *bytesBufPooler) ensureCap(newcap int, bs []byte) (bs2 []byte) {
// 	if z.pool == nil {
// 		bs2 = z.get(newcap)[:len(bs)]
// 		copy(bs2, bs)
// 		return
// 	}
// 	var bp2 bytesBufPooler
// 	bs2 = bp2.get(newcap)[:len(bs)]
// 	copy(bs2, bs)
// 	z.end()
// 	*z = bp2
// 	return
// }

// func (z *bytesBufPooler) buf() (buf []byte) {
// 	switch z.pool {
// 	case nil:
// 	case &pool.buf256:
// 		buf = z.poolv.(*[256]byte)[:]
// 	case &pool.buf1k:
// 		buf = z.poolv.(*[1 * 1024]byte)[:]
// 	case &pool.buf2k:
// 		buf = z.poolv.(*[2 * 1024]byte)[:]
// 	case &pool.buf4k:
// 		buf = z.poolv.(*[4 * 1024]byte)[:]
// 	case &pool.buf8k:
// 		buf = z.poolv.(*[8 * 1024]byte)[:]
// 	case &pool.buf16k:
// 		buf = z.poolv.(*[16 * 1024]byte)[:]
// 	case &pool.buf32k:
// 		buf = z.poolv.(*[32 * 1024]byte)[:]
// 	}
// 	return
// }

func (z *bytesBufPooler) get(bufsize int) (buf []byte) {
	if !usePool {
		return make([]byte, bufsize)
	}

	if bufsize > bytesBufPoolerMaxSize {
		z.end()
		return make([]byte, bufsize)
	}

	switch z.pool {
	case nil:
		goto NEW
	case &pool4buf256:
		if bufsize <= 256 {
			buf = z.poolv.(*[256]byte)[:bufsize]
		}
	case &pool4buf1k:
		if bufsize <= 1*1024 {
			buf = z.poolv.(*[1 * 1024]byte)[:bufsize]
		}
	case &pool4buf2k:
		if bufsize <= 2*1024 {
			buf = z.poolv.(*[2 * 1024]byte)[:bufsize]
		}
	case &pool4buf4k:
		if bufsize <= 4*1024 {
			buf = z.poolv.(*[4 * 1024]byte)[:bufsize]
		}
	case &pool4buf8k:
		if bufsize <= 8*1024 {
			buf = z.poolv.(*[8 * 1024]byte)[:bufsize]
		}
	case &pool4buf16k:
		if bufsize <= 16*1024 {
			buf = z.poolv.(*[16 * 1024]byte)[:bufsize]
		}
	case &pool4buf32k:
		if bufsize <= 32*1024 {
			buf = z.poolv.(*[32 * 1024]byte)[:bufsize]
		}
	}
	if buf != nil {
		return
	}
	z.end()

NEW:

	// // Try to use binary search.
	// // This is not optimal, as most folks select 1k or 2k buffers
	// // so a linear search is better (sequence of if/else blocks)
	// if bufsize < 1 {
	// 	bufsize = 0
	// } else {
	// 	bufsize--
	// 	bufsize /= 1024
	// }
	// switch bufsize {
	// case 0:
	// 	z.pool, z.poolv = pool.bytes1k()
	// 	buf = z.poolv.(*[1 * 1024]byte)[:]
	// case 1:
	// 	z.pool, z.poolv = pool.bytes2k()
	// 	buf = z.poolv.(*[2 * 1024]byte)[:]
	// case 2, 3:
	// 	z.pool, z.poolv = pool.bytes4k()
	// 	buf = z.poolv.(*[4 * 1024]byte)[:]
	// case 4, 5, 6, 7:
	// 	z.pool, z.poolv = pool.bytes8k()
	// 	buf = z.poolv.(*[8 * 1024]byte)[:]
	// case 8, 9, 10, 11, 12, 13, 14, 15:
	// 	z.pool, z.poolv = pool.bytes16k()
	// 	buf = z.poolv.(*[16 * 1024]byte)[:]
	// case 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31:
	// 	z.pool, z.poolv = pool.bytes32k()
	// 	buf = z.poolv.(*[32 * 1024]byte)[:]
	// default:
	// 	z.pool, z.poolv = pool.bytes64k()
	// 	buf = z.poolv.(*[64 * 1024]byte)[:]
	// }
	// return

	if bufsize <= 256 {
		z.pool, z.poolv = &pool4buf256, pool4buf256.Get() // pool.bytes1k()
		buf = z.poolv.(*[256]byte)[:bufsize]
	} else if bufsize <= 1*1024 {
		z.pool, z.poolv = &pool4buf1k, pool4buf1k.Get() // pool.bytes1k()
		buf = z.poolv.(*[1 * 1024]byte)[:bufsize]
	} else if bufsize <= 2*1024 {
		z.pool, z.poolv = &pool4buf2k, pool4buf2k.Get() // pool.bytes2k()
		buf = z.poolv.(*[2 * 1024]byte)[:bufsize]
	} else if bufsize <= 4*1024 {
		z.pool, z.poolv = &pool4buf4k, pool4buf4k.Get() // pool.bytes4k()
		buf = z.poolv.(*[4 * 1024]byte)[:bufsize]
	} else if bufsize <= 8*1024 {
		z.pool, z.poolv = &pool4buf8k, pool4buf8k.Get() // pool.bytes8k()
		buf = z.poolv.(*[8 * 1024]byte)[:bufsize]
	} else if bufsize <= 16*1024 {
		z.pool, z.poolv = &pool4buf16k, pool4buf16k.Get() // pool.bytes16k()
		buf = z.poolv.(*[16 * 1024]byte)[:bufsize]
	} else if bufsize <= 32*1024 {
		z.pool, z.poolv = &pool4buf32k, pool4buf32k.Get() // pool.bytes32k()
		buf = z.poolv.(*[32 * 1024]byte)[:bufsize]
		// } else {
		// 	z.pool, z.poolv = &pool.buf64k, pool.buf64k.Get() // pool.bytes64k()
		// 	buf = z.poolv.(*[64 * 1024]byte)[:]
	}
	return
}

// ----------------

type bytesBufSlicePooler struct {
	bytesBufPooler
	buf []byte
}

func (z *bytesBufSlicePooler) ensureExtraCap(num int) {
	if cap(z.buf) < len(z.buf)+num {
		z.ensureCap(len(z.buf) + num)
	}
}

func (z *bytesBufSlicePooler) ensureCap(newcap int) {
	if cap(z.buf) >= newcap {
		return
	}
	var bs2 []byte
	if z.pool == nil {
		bs2 = z.bytesBufPooler.get(newcap)[:len(z.buf)]
		if z.buf == nil {
			z.buf = bs2
		} else {
			copy(bs2, z.buf)
			z.buf = bs2
		}
		return
	}
	var bp2 bytesBufPooler
	if newcap > bytesBufPoolerMaxSize {
		bs2 = make([]byte, newcap)
	} else {
		bs2 = bp2.get(newcap)
	}
	bs2 = bs2[:len(z.buf)]
	copy(bs2, z.buf)
	z.end()
	z.buf = bs2
	z.bytesBufPooler = bp2
}

func (z *bytesBufSlicePooler) get(length int) {
	z.buf = z.bytesBufPooler.get(length)
}

func (z *bytesBufSlicePooler) append(b byte) {
	z.ensureExtraCap(1)
	z.buf = append(z.buf, b)
}

func (z *bytesBufSlicePooler) appends(b []byte) {
	z.ensureExtraCap(len(b))
	z.buf = append(z.buf, b...)
}

func (z *bytesBufSlicePooler) end() {
	z.bytesBufPooler.end()
	z.buf = nil
}

func (z *bytesBufSlicePooler) resetBuf() {
	if z.buf != nil {
		z.buf = z.buf[:0]
	}
}

// ----------------

type sfiRvPooler struct {
	pooler
}

func (z *sfiRvPooler) get(newlen int) (fkvs []sfiRv) {
	if newlen < 0 { // bounds-check-elimination
		// cannot happen // here for bounds-check-elimination
	} else if newlen <= 8 {
		z.pool, z.poolv = &pool4sfiRv8, pool4sfiRv8.Get() // pool.sfiRv8()
		fkvs = z.poolv.(*[8]sfiRv)[:newlen]
	} else if newlen <= 16 {
		z.pool, z.poolv = &pool4sfiRv16, pool4sfiRv16.Get() // pool.sfiRv16()
		fkvs = z.poolv.(*[16]sfiRv)[:newlen]
	} else if newlen <= 32 {
		z.pool, z.poolv = &pool4sfiRv32, pool4sfiRv32.Get() // pool.sfiRv32()
		fkvs = z.poolv.(*[32]sfiRv)[:newlen]
	} else if newlen <= 64 {
		z.pool, z.poolv = &pool4sfiRv64, pool4sfiRv64.Get() // pool.sfiRv64()
		fkvs = z.poolv.(*[64]sfiRv)[:newlen]
	} else if newlen <= 128 {
		z.pool, z.poolv = &pool4sfiRv128, pool4sfiRv128.Get() // pool.sfiRv128()
		fkvs = z.poolv.(*[128]sfiRv)[:newlen]
	} else {
		fkvs = make([]sfiRv, newlen)
	}
	return
}

*/

// ----------------

func freelistCapacity(length int) (capacity int) {
	for capacity = 8; capacity < length; capacity *= 2 {
	}
	return
}

type bytesFreelist [][]byte

func (x *bytesFreelist) get(length int) (out []byte) {
	var j int = -1
	for i := 0; i < len(*x); i++ {
		if cap((*x)[i]) >= length && (j == -1 || cap((*x)[j]) > cap((*x)[i])) {
			j = i
		}
	}
	if j == -1 {
		return make([]byte, length, freelistCapacity(length))
	}
	out = (*x)[j][:length]
	(*x)[j] = nil
	for i := 0; i < len(out); i++ {
		out[i] = 0
	}
	return
}

func (x *bytesFreelist) put(v []byte) {
	if len(v) == 0 {
		return
	}
	for i := 0; i < len(*x); i++ {
		if cap((*x)[i]) == 0 {
			(*x)[i] = v
			return
		}
	}
	*x = append(*x, v)
}

func (x *bytesFreelist) check(v []byte, length int) (out []byte) {
	if cap(v) < length {
		x.put(v)
		return x.get(length)
	}
	return v[:length]
}

// -------------------------

type sfiRvFreelist [][]sfiRv

func (x *sfiRvFreelist) get(length int) (out []sfiRv) {
	var j int = -1
	for i := 0; i < len(*x); i++ {
		if cap((*x)[i]) >= length && (j == -1 || cap((*x)[j]) > cap((*x)[i])) {
			j = i
		}
	}
	if j == -1 {
		return make([]sfiRv, length, freelistCapacity(length))
	}
	out = (*x)[j][:length]
	(*x)[j] = nil
	for i := 0; i < len(out); i++ {
		out[i] = sfiRv{}
	}
	return
}

func (x *sfiRvFreelist) put(v []sfiRv) {
	for i := 0; i < len(*x); i++ {
		if cap((*x)[i]) == 0 {
			(*x)[i] = v
			return
		}
	}
	*x = append(*x, v)
}

// -----------

// xdebugf printf. the message in red on the terminal.
// Use it in place of fmt.Printf (which it calls internally)
func xdebugf(pattern string, args ...interface{}) {
	xdebugAnyf("31", pattern, args...)
}

// xdebug2f printf. the message in blue on the terminal.
// Use it in place of fmt.Printf (which it calls internally)
func xdebug2f(pattern string, args ...interface{}) {
	xdebugAnyf("34", pattern, args...)
}

func xdebugAnyf(colorcode, pattern string, args ...interface{}) {
	if !xdebug {
		return
	}
	var delim string
	if len(pattern) > 0 && pattern[len(pattern)-1] != '\n' {
		delim = "\n"
	}
	fmt.Printf("\033[1;"+colorcode+"m"+pattern+delim+"\033[0m", args...)
	// os.Stderr.Flush()
}

// register these here, so that staticcheck stops barfing
var _ = xdebug2f
var _ = xdebugf
var _ = isNaN32

// func isImmutableKind(k reflect.Kind) (v bool) {
// 	return false ||
// 		k == reflect.Int ||
// 		k == reflect.Int8 ||
// 		k == reflect.Int16 ||
// 		k == reflect.Int32 ||
// 		k == reflect.Int64 ||
// 		k == reflect.Uint ||
// 		k == reflect.Uint8 ||
// 		k == reflect.Uint16 ||
// 		k == reflect.Uint32 ||
// 		k == reflect.Uint64 ||
// 		k == reflect.Uintptr ||
// 		k == reflect.Float32 ||
// 		k == reflect.Float64 ||
// 		k == reflect.Bool ||
// 		k == reflect.String
// }

// func timeLocUTCName(tzint int16) string {
// 	if tzint == 0 {
// 		return "UTC"
// 	}
// 	var tzname = []byte("UTC+00:00")
// 	//tzname := fmt.Sprintf("UTC%s%02d:%02d", tzsign, tz/60, tz%60) //perf issue using Sprintf.. inline below.
// 	//tzhr, tzmin := tz/60, tz%60 //faster if u convert to int first
// 	var tzhr, tzmin int16
// 	if tzint < 0 {
// 		tzname[3] = '-' // (TODO: verify. this works here)
// 		tzhr, tzmin = -tzint/60, (-tzint)%60
// 	} else {
// 		tzhr, tzmin = tzint/60, tzint%60
// 	}
// 	tzname[4] = timeDigits[tzhr/10]
// 	tzname[5] = timeDigits[tzhr%10]
// 	tzname[7] = timeDigits[tzmin/10]
// 	tzname[8] = timeDigits[tzmin%10]
// 	return string(tzname)
// 	//return time.FixedZone(string(tzname), int(tzint)*60)
// }
