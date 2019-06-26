// +build !safe
// +build !appengine
// +build go1.7

// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import (
	"reflect"
	"sync/atomic"
	"time"
	"unsafe"
)

// This file has unsafe variants of some helper methods.
// NOTE: See helper_not_unsafe.go for the usage information.

// For reflect.Value code, we decided to do the following:
//    - if we know the kind, we can elide conditional checks for
//      - SetXXX (Int, Uint, String, Bool, etc)
//      - SetLen
//
// We can also optimize
//      - IsNil
//
// We cannot do the same for Cap, Len if we still have to do conditional.

// var zeroRTv [4]uintptr

const safeMode = false

// keep in sync with GO_ROOT/src/reflect/value.go
const (
	unsafeFlagIndir    = 1 << 7
	unsafeFlagAddr     = 1 << 8
	unsafeFlagKindMask = (1 << 5) - 1 // 5 bits for 27 kinds (up to 31)
	// unsafeTypeKindDirectIface = 1 << 5
)

type unsafeString struct {
	Data unsafe.Pointer
	Len  int
}

type unsafeSlice struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

type unsafeIntf struct {
	typ  unsafe.Pointer
	word unsafe.Pointer
}

type unsafeReflectValue struct {
	typ  unsafe.Pointer
	ptr  unsafe.Pointer
	flag uintptr
}

func stringView(v []byte) string {
	if len(v) == 0 {
		return ""
	}
	bx := (*unsafeSlice)(unsafe.Pointer(&v))
	return *(*string)(unsafe.Pointer(&unsafeString{bx.Data, bx.Len}))
}

func bytesView(v string) []byte {
	if len(v) == 0 {
		return zeroByteSlice
	}
	sx := (*unsafeString)(unsafe.Pointer(&v))
	return *(*[]byte)(unsafe.Pointer(&unsafeSlice{sx.Data, sx.Len, sx.Len}))
}

// // isNilRef says whether the interface is a nil reference or not.
// //
// // A reference here is a pointer-sized reference i.e. map, ptr, chan, func, unsafepointer.
// // It is optional to extend this to also check if slices or interfaces are nil also.
// //
// // NOTE: There is no global way of checking if an interface is nil.
// // For true references (map, ptr, func, chan), you can just look
// // at the word of the interface.
// // However, for slices, you have to dereference
// // the word, and get a pointer to the 3-word interface value.
// func isNilRef(v interface{}) (rv reflect.Value, isnil bool) {
// 	isnil = ((*unsafeIntf)(unsafe.Pointer(&v))).word == nil
// 	return
// }

func isNil(v interface{}) (rv reflect.Value, isnil bool) {
	var ui = (*unsafeIntf)(unsafe.Pointer(&v))
	if ui.word == nil {
		isnil = true
		return
	}
	rv = rv4i(v) // reflect.value is cheap and inline'able
	tk := rv.Kind()
	isnil = (tk == reflect.Interface || tk == reflect.Slice) && *(*unsafe.Pointer)(ui.word) == nil
	return
	// fmt.Printf(">>>> isNil: isnil: %v, TYPE: %T, word: %v, *word: %v, type: %v, nil: %v\n",
	// 	v == nil, v, word, *((*unsafe.Pointer)(word)), ui.typ, nil)
}

func rv2ptr(urv *unsafeReflectValue) (ptr unsafe.Pointer) {
	// true references (map, func, chan, ptr - NOT slice) may be double-referenced? as flagIndir
	// rv := *((*reflect.Value)(unsafe.Pointer(urv)))
	if refBitset.isset(byte(urv.flag&unsafeFlagKindMask)) && urv.flag&unsafeFlagIndir != 0 {
		ptr = *(*unsafe.Pointer)(urv.ptr)
	} else {
		ptr = urv.ptr
	}
	return
}

func rv4i(i interface{}) (rv reflect.Value) {
	// Unfortunately, we cannot get the "kind" of the interface directly here.
	// We need the 'rtype', whose structure changes in different go versions.
	// Finally, it's not clear that there is benefit to reimplementin it,
	// as the "escapes(i)" is not clearly expensive.
	//
	// urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	// ui := (*unsafeIntf)(unsafe.Pointer(&i))
	return reflect.ValueOf(i)
}

func rv2i(rv reflect.Value) interface{} {
	// TODO: consider a more generally-known optimization for reflect.Value ==> Interface
	//
	// Currently, we use this fragile method that taps into implememtation details from
	// the source go stdlib reflect/value.go, and trims the implementation.

	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*interface{})(unsafe.Pointer(&unsafeIntf{typ: urv.typ, word: rv2ptr(urv)}))
}

func rvIsNil(rv reflect.Value) bool {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	if urv.flag&unsafeFlagIndir != 0 {
		return *(*unsafe.Pointer)(urv.ptr) == nil
	}
	return urv.ptr == nil
}

func rvSetSliceLen(rv reflect.Value, length int) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	(*unsafeString)(urv.ptr).Len = length
}

// func rvzeroaddr(t reflect.Type) (rv reflect.Value) {
// 	// return reflect.New(t).Elem()
// 	var ui = (*unsafeIntf)(unsafe.Pointer(&t))
// 	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
// 	urv.typ = ui.word
// 	urv.flag = uintptr(t.Kind()) | unsafeFlagIndir | unsafeFlagAddr
// 	urv.ptr = unsafe_New(ui.word)
// 	return
// }

func rvZeroAddrK(t reflect.Type, k reflect.Kind) (rv reflect.Value) {
	// return reflect.New(t).Elem()
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	urv.flag = uintptr(k) | unsafeFlagIndir | unsafeFlagAddr
	urv.typ = ((*unsafeIntf)(unsafe.Pointer(&t))).word
	urv.ptr = unsafe_New(urv.typ)
	return
}

func rvConvert(v reflect.Value, t reflect.Type) (rv reflect.Value) {
	uv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*urv = *uv
	urv.typ = ((*unsafeIntf)(unsafe.Pointer(&t))).word
	return
}

// func rvisnilref(rv reflect.Value) bool {
// 	return (*unsafeReflectValue)(unsafe.Pointer(&rv)).ptr == nil
// }

// func rvslen(rv reflect.Value) int {
// 	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
// 	return (*unsafeString)(urv.ptr).Len
// }

// func rv2rtid(rv reflect.Value) uintptr {
// 	return uintptr((*unsafeReflectValue)(unsafe.Pointer(&rv)).typ)
// }

func rt2id(rt reflect.Type) uintptr {
	return uintptr(((*unsafeIntf)(unsafe.Pointer(&rt))).word)
}

func i2rtid(i interface{}) uintptr {
	return uintptr(((*unsafeIntf)(unsafe.Pointer(&i))).typ)
}

// --------------------------

func isEmptyValue(v reflect.Value, tinfos *TypeInfos, deref, checkStruct bool) bool {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	if urv.flag == 0 {
		return true
	}
	switch v.Kind() {
	case reflect.Invalid:
		return true
	case reflect.String:
		return (*unsafeString)(urv.ptr).Len == 0
	case reflect.Slice:
		return (*unsafeSlice)(urv.ptr).Len == 0
	case reflect.Bool:
		return !*(*bool)(urv.ptr)
	case reflect.Int:
		return *(*int)(urv.ptr) == 0
	case reflect.Int8:
		return *(*int8)(urv.ptr) == 0
	case reflect.Int16:
		return *(*int16)(urv.ptr) == 0
	case reflect.Int32:
		return *(*int32)(urv.ptr) == 0
	case reflect.Int64:
		return *(*int64)(urv.ptr) == 0
	case reflect.Uint:
		return *(*uint)(urv.ptr) == 0
	case reflect.Uint8:
		return *(*uint8)(urv.ptr) == 0
	case reflect.Uint16:
		return *(*uint16)(urv.ptr) == 0
	case reflect.Uint32:
		return *(*uint32)(urv.ptr) == 0
	case reflect.Uint64:
		return *(*uint64)(urv.ptr) == 0
	case reflect.Uintptr:
		return *(*uintptr)(urv.ptr) == 0
	case reflect.Float32:
		return *(*float32)(urv.ptr) == 0
	case reflect.Float64:
		return *(*float64)(urv.ptr) == 0
	case reflect.Interface:
		isnil := urv.ptr == nil || *(*unsafe.Pointer)(urv.ptr) == nil
		if deref {
			if isnil {
				return true
			}
			return isEmptyValue(v.Elem(), tinfos, deref, checkStruct)
		}
		return isnil
	case reflect.Ptr:
		// isnil := urv.ptr == nil (not sufficient, as a pointer value encodes the type)
		isnil := urv.ptr == nil || *(*unsafe.Pointer)(urv.ptr) == nil
		if deref {
			if isnil {
				return true
			}
			return isEmptyValue(v.Elem(), tinfos, deref, checkStruct)
		}
		return isnil
	case reflect.Struct:
		return isEmptyStruct(v, tinfos, deref, checkStruct)
	case reflect.Map, reflect.Array, reflect.Chan:
		return v.Len() == 0
	}
	return false
}

// --------------------------

// atomicXXX is expected to be 2 words (for symmetry with atomic.Value)
//
// Note that we do not atomically load/store length and data pointer separately,
// as this could lead to some races. Instead, we atomically load/store cappedSlice.
//
// Note: with atomic.(Load|Store)Pointer, we MUST work with an unsafe.Pointer directly.

// ----------------------
type atomicTypeInfoSlice struct {
	v unsafe.Pointer // *[]rtid2ti
	_ uint64         // padding (atomicXXX expected to be 2 words)
}

func (x *atomicTypeInfoSlice) load() (s []rtid2ti) {
	x2 := atomic.LoadPointer(&x.v)
	if x2 != nil {
		s = *(*[]rtid2ti)(x2)
	}
	return
}

func (x *atomicTypeInfoSlice) store(p []rtid2ti) {
	atomic.StorePointer(&x.v, unsafe.Pointer(&p))
}

// --------------------------
type atomicRtidFnSlice struct {
	v unsafe.Pointer // *[]codecRtidFn
	_ uint64         // padding (atomicXXX expected to be 2 words) (make 1 word so JsonHandle fits)
}

func (x *atomicRtidFnSlice) load() (s []codecRtidFn) {
	x2 := atomic.LoadPointer(&x.v)
	if x2 != nil {
		s = *(*[]codecRtidFn)(x2)
	}
	return
}

func (x *atomicRtidFnSlice) store(p []codecRtidFn) {
	atomic.StorePointer(&x.v, unsafe.Pointer(&p))
}

// --------------------------
type atomicClsErr struct {
	v unsafe.Pointer // *clsErr
	_ uint64         // padding (atomicXXX expected to be 2 words)
}

func (x *atomicClsErr) load() (e clsErr) {
	x2 := (*clsErr)(atomic.LoadPointer(&x.v))
	if x2 != nil {
		e = *x2
	}
	return
}

func (x *atomicClsErr) store(p clsErr) {
	atomic.StorePointer(&x.v, unsafe.Pointer(&p))
}

// --------------------------

// to create a reflect.Value for each member field of decNaked,
// we first create a global decNaked, and create reflect.Value
// for them all.
// This way, we have the flags and type in the reflect.Value.
// Then, when a reflect.Value is called, we just copy it,
// update the ptr to the decNaked's, and return it.

type unsafeDecNakedWrapper struct {
	decNaked
	ru, ri, rf, rl, rs, rb, rt reflect.Value // mapping to the primitives above
}

func (n *unsafeDecNakedWrapper) init() {
	n.ru = rv4i(&n.u).Elem()
	n.ri = rv4i(&n.i).Elem()
	n.rf = rv4i(&n.f).Elem()
	n.rl = rv4i(&n.l).Elem()
	n.rs = rv4i(&n.s).Elem()
	n.rt = rv4i(&n.t).Elem()
	n.rb = rv4i(&n.b).Elem()
	// n.rr[] = rv4i(&n.)
}

var defUnsafeDecNakedWrapper unsafeDecNakedWrapper

func init() {
	defUnsafeDecNakedWrapper.init()
}

func (n *decNaked) ru() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.ru
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.u)
	return
}
func (n *decNaked) ri() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.ri
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.i)
	return
}
func (n *decNaked) rf() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.rf
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.f)
	return
}
func (n *decNaked) rl() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.rl
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.l)
	return
}
func (n *decNaked) rs() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.rs
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.s)
	return
}
func (n *decNaked) rt() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.rt
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.t)
	return
}
func (n *decNaked) rb() (v reflect.Value) {
	v = defUnsafeDecNakedWrapper.rb
	((*unsafeReflectValue)(unsafe.Pointer(&v))).ptr = unsafe.Pointer(&n.b)
	return
}

// --------------------------
func rvSetBytes(rv reflect.Value, v []byte) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*[]byte)(urv.ptr) = v
}

func rvSetString(rv reflect.Value, v string) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*string)(urv.ptr) = v
}

func rvSetBool(rv reflect.Value, v bool) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*bool)(urv.ptr) = v
}

func rvSetTime(rv reflect.Value, v time.Time) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*time.Time)(urv.ptr) = v
}

func rvSetFloat32(rv reflect.Value, v float32) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*float32)(urv.ptr) = v
}

func rvSetFloat64(rv reflect.Value, v float64) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*float64)(urv.ptr) = v
}

func rvSetInt(rv reflect.Value, v int) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*int)(urv.ptr) = v
}

func rvSetInt8(rv reflect.Value, v int8) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*int8)(urv.ptr) = v
}

func rvSetInt16(rv reflect.Value, v int16) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*int16)(urv.ptr) = v
}

func rvSetInt32(rv reflect.Value, v int32) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*int32)(urv.ptr) = v
}

func rvSetInt64(rv reflect.Value, v int64) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*int64)(urv.ptr) = v
}

func rvSetUint(rv reflect.Value, v uint) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*uint)(urv.ptr) = v
}

func rvSetUintptr(rv reflect.Value, v uintptr) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*uintptr)(urv.ptr) = v
}

func rvSetUint8(rv reflect.Value, v uint8) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*uint8)(urv.ptr) = v
}

func rvSetUint16(rv reflect.Value, v uint16) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*uint16)(urv.ptr) = v
}

func rvSetUint32(rv reflect.Value, v uint32) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*uint32)(urv.ptr) = v
}

func rvSetUint64(rv reflect.Value, v uint64) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	*(*uint64)(urv.ptr) = v
}

// ----------------

// rvSetDirect is rv.Set for all kinds except reflect.Interface
func rvSetDirect(rv reflect.Value, v reflect.Value) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	uv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	if uv.flag&unsafeFlagIndir == 0 {
		*(*unsafe.Pointer)(urv.ptr) = uv.ptr
	} else {
		typedmemmove(urv.typ, urv.ptr, uv.ptr)
	}
}

// rvSlice returns a slice of the slice of lenth
func rvSlice(rv reflect.Value, length int) (v reflect.Value) {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	uv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	*uv = *urv
	var x []unsafe.Pointer
	uv.ptr = unsafe.Pointer(&x)
	*(*unsafeSlice)(uv.ptr) = *(*unsafeSlice)(urv.ptr)
	(*unsafeSlice)(uv.ptr).Len = length
	// xdebugf("length: %d, slice: from: %#v, to: %#v",
	// 	length, *(*unsafeSlice)(urv.ptr), *(*unsafeSlice)(uv.ptr))
	return
}

// ------------

func rvGetSliceLen(rv reflect.Value) int {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return (*unsafeSlice)(urv.ptr).Len
}

func rvGetSliceCap(rv reflect.Value) int {
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return (*unsafeSlice)(urv.ptr).Cap
}

func rvGetArrayBytesRO(rv reflect.Value, scratch []byte) (bs []byte) {
	l := rv.Len()
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	bx := (*unsafeSlice)(unsafe.Pointer(&bs))
	bx.Data = urv.ptr
	bx.Len, bx.Cap = l, l
	return
}

func rvGetArray4Slice(rv reflect.Value) (v reflect.Value) {
	v = rvZeroAddrK(reflectArrayOf(rvGetSliceLen(rv), rv.Type().Elem()), reflect.Array)
	// xdebugf("rvGetArray4Slice: b4 copy: rv: %#v, v: %#v", rv, v)
	urv := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	uv := (*unsafeReflectValue)(unsafe.Pointer(&v))
	uv.ptr = *(*unsafe.Pointer)(urv.ptr) // slice rv has a ptr to the slice.
	// xdebugf("rvGetArray4Slice: after copy: v: %#v", v)
	return
}

func rvGetSlice4Array(rv reflect.Value, tslice reflect.Type) (v reflect.Value) {
	uv := (*unsafeReflectValue)(unsafe.Pointer(&v))

	var x []unsafe.Pointer

	uv.ptr = unsafe.Pointer(&x)
	uv.typ = ((*unsafeIntf)(unsafe.Pointer(&tslice))).word
	uv.flag = unsafeFlagIndir | uintptr(reflect.Slice)

	s := (*unsafeSlice)(uv.ptr)
	s.Data = ((*unsafeReflectValue)(unsafe.Pointer(&rv))).ptr
	s.Len = rv.Len()
	s.Cap = s.Len
	return
}

func rvCopySlice(dest, src reflect.Value) {
	t := dest.Type().Elem()
	urv := (*unsafeReflectValue)(unsafe.Pointer(&dest))
	destPtr := urv.ptr
	urv = (*unsafeReflectValue)(unsafe.Pointer(&src))
	typedslicecopy((*unsafeIntf)(unsafe.Pointer(&t)).word,
		*(*unsafeSlice)(destPtr), *(*unsafeSlice)(urv.ptr))
}

// ------------

func rvGetBool(rv reflect.Value) bool {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*bool)(v.ptr)
}

func rvGetBytes(rv reflect.Value) []byte {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*[]byte)(v.ptr)
}

func rvGetTime(rv reflect.Value) time.Time {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*time.Time)(v.ptr)
}

func rvGetString(rv reflect.Value) string {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*string)(v.ptr)
}

func rvGetFloat64(rv reflect.Value) float64 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*float64)(v.ptr)
}

func rvGetFloat32(rv reflect.Value) float32 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*float32)(v.ptr)
}

func rvGetInt(rv reflect.Value) int {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*int)(v.ptr)
}

func rvGetInt8(rv reflect.Value) int8 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*int8)(v.ptr)
}

func rvGetInt16(rv reflect.Value) int16 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*int16)(v.ptr)
}

func rvGetInt32(rv reflect.Value) int32 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*int32)(v.ptr)
}

func rvGetInt64(rv reflect.Value) int64 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*int64)(v.ptr)
}

func rvGetUint(rv reflect.Value) uint {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*uint)(v.ptr)
}

func rvGetUint8(rv reflect.Value) uint8 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*uint8)(v.ptr)
}

func rvGetUint16(rv reflect.Value) uint16 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*uint16)(v.ptr)
}

func rvGetUint32(rv reflect.Value) uint32 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*uint32)(v.ptr)
}

func rvGetUint64(rv reflect.Value) uint64 {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*uint64)(v.ptr)
}

func rvGetUintptr(rv reflect.Value) uintptr {
	v := (*unsafeReflectValue)(unsafe.Pointer(&rv))
	return *(*uintptr)(v.ptr)
}

// ------------ map range and map indexing ----------

// regular calls to map via reflection: MapKeys, MapIndex, MapRange/MapIter etc
// will always allocate for each map key or value.
//
// It is more performant to provide a value that the map entry is set into,
// and that elides the allocation.

// unsafeMapHashIter
//
// go 1.4+ has runtime/hashmap.go or runtime/map.go which has a
// hIter struct with the first 2 values being key and value
// of the current iteration.
//
// This *hIter is passed to mapiterinit, mapiternext, mapiterkey, mapiterelem.
// We bypass the reflect wrapper functions and just use the *hIter directly.
//
// Though *hIter has many fields, we only care about the first 2.
type unsafeMapHashIter struct {
	key, value unsafe.Pointer
	// other fields are ignored
}

// type unsafeReflectMapIter struct {
// 	m  unsafeReflectValue
// 	it unsafe.Pointer
// }

type unsafeMapIter struct {
	it *unsafeMapHashIter
	// k, v             reflect.Value
	mtyp, ktyp, vtyp unsafe.Pointer
	mptr, kptr, vptr unsafe.Pointer
	kisref, visref   bool
	mapvalues        bool
	done             bool

	// _ [2]uint64 // padding (cache-aligned)
}

// // pprof show that 13% of cbor encode time taken in
// // allocation of unsafeMapIter.
// // Options are to try to alloc on stack, or pool it.
// // Easiest to pool it.
// const unsafeMapIterUsePool = false

// var unsafeMapIterPool = sync.Pool{
// 	New: func() interface{} { return new(unsafeMapIter) },
// }

func (t *unsafeMapIter) ValidKV() (r bool) {
	return false
}

func (t *unsafeMapIter) Next() (r bool) {
	if t == nil || t.done {
		return
	}
	if t.it == nil {
		t.it = (*unsafeMapHashIter)(mapiterinit(t.mtyp, t.mptr))
	} else {
		mapiternext((unsafe.Pointer)(t.it))
	}
	t.done = t.it.key == nil
	if t.done {
		return
	}
	unsafeMapSet(t.kptr, t.ktyp, t.it.key, t.kisref)
	if t.mapvalues {
		unsafeMapSet(t.vptr, t.vtyp, t.it.value, t.visref)
	}
	return true
}

func (t *unsafeMapIter) Key() (r reflect.Value) {
	// return t.k
	return
}

func (t *unsafeMapIter) Value() (r reflect.Value) {
	// if t.mapvalues {
	// 	return t.v
	// }
	return
}

func (t *unsafeMapIter) Done() {
	// if unsafeMapIterUsePool && t != nil {
	// 	*t = unsafeMapIter{}
	// 	unsafeMapIterPool.Put(t)
	// }
}

func unsafeMapSet(p, ptyp, p2 unsafe.Pointer, isref bool) {
	if isref {
		*(*unsafe.Pointer)(p) = *(*unsafe.Pointer)(p2) // p2
	} else {
		typedmemmove(ptyp, p, p2) // *(*unsafe.Pointer)(p2)) // p2)
	}
}

func unsafeMapKVPtr(urv *unsafeReflectValue) unsafe.Pointer {
	if urv.flag&unsafeFlagIndir == 0 {
		return unsafe.Pointer(&urv.ptr)
	}
	return urv.ptr
}

func mapRange(m, k, v reflect.Value, mapvalues bool) (t *unsafeMapIter) {
	if rvIsNil(m) {
		// return &unsafeMapIter{done: true}
		return
	}
	// if unsafeMapIterUsePool {
	// 	t = unsafeMapIterPool.Get().(*unsafeMapIter)
	// } else {
	//	t = new(unsafeMapIter)
	// }
	t = new(unsafeMapIter)
	// t.k = k
	// t.v = v
	t.mapvalues = mapvalues

	var urv *unsafeReflectValue

	urv = (*unsafeReflectValue)(unsafe.Pointer(&m))
	t.mtyp = urv.typ
	t.mptr = rv2ptr(urv)

	urv = (*unsafeReflectValue)(unsafe.Pointer(&k))
	t.ktyp = urv.typ
	t.kptr = urv.ptr
	t.kisref = refBitset.isset(byte(k.Kind()))

	if mapvalues {
		urv = (*unsafeReflectValue)(unsafe.Pointer(&v))
		t.vtyp = urv.typ
		t.vptr = urv.ptr
		t.visref = refBitset.isset(byte(v.Kind()))
	}

	return
}

func mapGet(m, k, v reflect.Value) (vv reflect.Value) {
	var urv = (*unsafeReflectValue)(unsafe.Pointer(&k))
	var kptr = unsafeMapKVPtr(urv)

	urv = (*unsafeReflectValue)(unsafe.Pointer(&m))

	vvptr := mapaccess(urv.typ, rv2ptr(urv), kptr)
	if vvptr == nil {
		return
	}
	// vvptr = *(*unsafe.Pointer)(vvptr)

	urv = (*unsafeReflectValue)(unsafe.Pointer(&v))

	unsafeMapSet(urv.ptr, urv.typ, vvptr, refBitset.isset(byte(v.Kind())))
	return v
}

func mapSet(m, k, v reflect.Value) {
	var urv = (*unsafeReflectValue)(unsafe.Pointer(&k))
	var kptr = unsafeMapKVPtr(urv)
	urv = (*unsafeReflectValue)(unsafe.Pointer(&v))
	var vptr = unsafeMapKVPtr(urv)
	urv = (*unsafeReflectValue)(unsafe.Pointer(&m))
	mapassign(urv.typ, rv2ptr(urv), kptr, vptr)
}

func mapDelete(m, k reflect.Value) {
	var urv = (*unsafeReflectValue)(unsafe.Pointer(&k))
	var kptr = unsafeMapKVPtr(urv)
	urv = (*unsafeReflectValue)(unsafe.Pointer(&m))
	mapdelete(urv.typ, rv2ptr(urv), kptr)
}

// return an addressable reflect value that can be used in mapRange and mapGet operations.
//
// all calls to mapGet or mapRange will call here to get an addressable reflect.Value.
func mapAddressableRV(t reflect.Type, k reflect.Kind) (r reflect.Value) {
	// return reflect.New(t).Elem()
	return rvZeroAddrK(t, k)
}

//go:linkname mapiterinit reflect.mapiterinit
//go:noescape
func mapiterinit(typ unsafe.Pointer, it unsafe.Pointer) (key unsafe.Pointer)

//go:linkname mapiternext reflect.mapiternext
//go:noescape
func mapiternext(it unsafe.Pointer) (key unsafe.Pointer)

//go:linkname mapaccess reflect.mapaccess
//go:noescape
func mapaccess(typ unsafe.Pointer, m unsafe.Pointer, key unsafe.Pointer) (val unsafe.Pointer)

//go:linkname mapassign reflect.mapassign
//go:noescape
func mapassign(typ unsafe.Pointer, m unsafe.Pointer, key, val unsafe.Pointer)

//go:linkname mapdelete reflect.mapdelete
//go:noescape
func mapdelete(typ unsafe.Pointer, m unsafe.Pointer, key unsafe.Pointer)

//go:linkname typedmemmove reflect.typedmemmove
//go:noescape
func typedmemmove(typ unsafe.Pointer, dst, src unsafe.Pointer)

//go:linkname unsafe_New reflect.unsafe_New
//go:noescape
func unsafe_New(typ unsafe.Pointer) unsafe.Pointer

//go:linkname typedslicecopy reflect.typedslicecopy
//go:noescape
func typedslicecopy(elemType unsafe.Pointer, dst, src unsafeSlice) int
