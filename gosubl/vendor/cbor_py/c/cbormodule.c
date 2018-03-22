#include "Python.h"

#include "cbor.h"

#include <math.h>
#include <stdint.h>

//#include <stdio.h>
#include <arpa/inet.h>


#ifndef DEBUG_LOGGING
// causes things to be written to stderr
#define DEBUG_LOGGING 0
//#define DEBUG_LOGGING 1
#endif


#ifdef Py_InitModule
// Python 2.7

#define HAS_FILE_READER 1
#define IS_PY3 0

#else

#define HAS_FILE_READER 0
#define IS_PY3 1

#endif

typedef struct {
    unsigned int sort_keys;
} EncodeOptions;

// Hey Look! It's a polymorphic object structure in C!

// read(, len): read len bytes and return in buffer, or NULL on error
// read1(, uint8_t*): read one byte and return 0 on success
// return_buffer(, *): release result of read(, len)
// delete(): destructor. free thiz and contents.
#define READER_FUNCTIONS \
    void* (*read)(void* self, Py_ssize_t len); \
    int (*read1)(void* self, uint8_t* oneByte); \
    void (*return_buffer)(void* self, void* buffer); \
    void (*delete)(void* self);

#define SET_READER_FUNCTIONS(thiz, clazz) (thiz)->read = clazz##_read;\
    (thiz)->read1 = clazz##_read1;\
    (thiz)->return_buffer = clazz##_return_buffer;\
    (thiz)->delete = clazz##_delete;

typedef struct _Reader {
    READER_FUNCTIONS;
} Reader;

static Reader* NewBufferReader(PyObject* ob);
static Reader* NewObjectReader(PyObject* ob);
#if HAS_FILE_READER
static Reader* NewFileReader(PyObject* ob);
#endif


static PyObject* loads_tag(Reader* rin, uint64_t aux);
static int loads_kv(PyObject* out, Reader* rin);

typedef struct VarBufferPart {
    void* start;
    uint64_t len;
    struct VarBufferPart* next;
} VarBufferPart;


static int logprintf(const char* fmt, ...) {
    va_list ap;
    int ret;
    va_start(ap, fmt);
#if DEBUG_LOGGING
    ret = vfprintf(stderr, fmt, ap);
#else
    ret = 0;
#endif
    va_end(ap);
    return ret;
}

// TODO: portably work this out at compile time
static int _is_big_endian = 0;

static int is_big_endian(void) {
    uint32_t val = 1234;
    _is_big_endian = val == htonl(val);
    //logprintf("is_big_endian=%d\n", _is_big_endian);
    return _is_big_endian;
}


PyObject* decodeFloat16(Reader* rin) {
    // float16 parsing adapted from example code in spec
    uint8_t hibyte, lobyte;// = raw[pos];
    int err;
    int exp;
    int mant;
    double val;

    err = rin->read1(rin, &hibyte);
    if (err) { logprintf("fail in float16[0]\n"); return NULL; }
    err = rin->read1(rin, &lobyte);
    if (err) { logprintf("fail in float16[1]\n"); return NULL; }

    exp = (hibyte >> 2) & 0x1f;
    mant = ((hibyte & 0x3) << 8) | lobyte;
    if (exp == 0) {
	val = ldexp(mant, -24);
    } else if (exp != 31) {
	val = ldexp(mant + 1024, exp - 25);
    } else {
	val = mant == 0 ? INFINITY : NAN;
    }
    if (hibyte & 0x80) {
	val = -val;
    }
    return PyFloat_FromDouble(val);
}
PyObject* decodeFloat32(Reader* rin) {
    float val;
    uint8_t* raw = rin->read(rin, 4);
    if (!raw) { logprintf("fail in float32\n"); return NULL; }
    if (_is_big_endian) {
	// easy!
	val = *((float*)raw);
    } else {
	uint8_t* dest = (uint8_t*)(&val);
	dest[3] = raw[0];
	dest[2] = raw[1];
	dest[1] = raw[2];
	dest[0] = raw[3];
    }
    rin->return_buffer(rin, raw);
    return PyFloat_FromDouble(val);
}
PyObject* decodeFloat64(Reader* rin) {
    int si;
    uint64_t aux = 0;
    uint8_t* raw = rin->read(rin, 8);
    if (!raw) { logprintf("fail in float64\n"); return NULL; }
    for (si = 0; si < 8; si++) {
	aux = aux << 8;
	aux |= raw[si];
    }
    rin->return_buffer(rin, raw);
    return PyFloat_FromDouble(*((double*)(&aux)));
}

// parse following int value into *auxP
// return 0 on success, -1 on fail
static int handle_info_bits(Reader* rin, uint8_t cbor_info, uint64_t* auxP) {
    uint64_t aux;

    if (cbor_info <= 23) {
	// literal value <=23
	aux = cbor_info;
    } else if (cbor_info == CBOR_UINT8_FOLLOWS) {
	uint8_t taux;
	if (rin->read1(rin, &taux)) { logprintf("fail in uint8\n"); return -1; }
	aux = taux;
    } else if (cbor_info == CBOR_UINT16_FOLLOWS) {
	uint8_t hibyte, lobyte;
	if (rin->read1(rin, &hibyte)) { logprintf("fail in uint16[0]\n"); return -1; }
	if (rin->read1(rin, &lobyte)) { logprintf("fail in uint16[1]\n"); return -1; }
	aux = (hibyte << 8) | lobyte;
    } else if (cbor_info == CBOR_UINT32_FOLLOWS) {
	uint8_t* raw = (uint8_t*)rin->read(rin, 4);
	if (!raw) { logprintf("fail in uint32[1]\n"); return -1; }
	aux = 
            (((uint64_t)raw[0]) << 24) |
	    (((uint64_t)raw[1]) << 16) |
	    (((uint64_t)raw[2]) <<  8) |
	    ((uint64_t)raw[3]);
	rin->return_buffer(rin, raw);
    } else if (cbor_info == CBOR_UINT64_FOLLOWS) {
        int si;
	uint8_t* raw = (uint8_t*)rin->read(rin, 8);
	if (!raw) { logprintf("fail in uint64[1]\n"); return -1; }
	aux = 0;
	for (si = 0; si < 8; si++) {
	    aux = aux << 8;
	    aux |= raw[si];
	}
	rin->return_buffer(rin, raw);
    } else {
	aux = 0;
    }
    *auxP = aux;
    return 0;
}

static PyObject* inner_loads_c(Reader* rin, uint8_t c);

static PyObject* inner_loads(Reader* rin) {
    uint8_t c;
    int err;

    err = rin->read1(rin, &c);
    if (err) { logprintf("fail in loads tag\n"); return NULL; }
    return inner_loads_c(rin, c);
}

PyObject* inner_loads_c(Reader* rin, uint8_t c) {
    uint8_t cbor_type;
    uint8_t cbor_info;
    uint64_t aux;

    cbor_type = c & CBOR_TYPE_MASK;
    cbor_info = c & CBOR_INFO_BITS;

#if 0
    if (pos > len) {
	PyErr_SetString(PyExc_ValueError, "misparse, token went longer than buffer");
	return NULL;
    }

    pos += 1;
#endif

    if (cbor_type == CBOR_7) {
	if (cbor_info == CBOR_UINT16_FOLLOWS) { // float16
	    return decodeFloat16(rin);
	} else if (cbor_info == CBOR_UINT32_FOLLOWS) { // float32
	    return decodeFloat32(rin);
	} else if (cbor_info == CBOR_UINT64_FOLLOWS) {  // float64
	    return decodeFloat64(rin);
	}
	// not a float, fall through to other CBOR_7 interpretations
    }
    if (handle_info_bits(rin, cbor_info, &aux)) { logprintf("info bits failed\n"); return NULL; }

    PyObject* out = NULL;
    switch (cbor_type) {
    case CBOR_UINT:
	out = PyLong_FromUnsignedLongLong(aux);
        if (out == NULL) {
            PyErr_SetString(PyExc_RuntimeError, "unknown error decoding UINT");
        }
        return out;
    case CBOR_NEGINT:
	if (aux > 0x7fffffffffffffff) {
	    PyObject* bignum = PyLong_FromUnsignedLongLong(aux);
	    PyObject* minusOne = PyLong_FromLong(-1);
	    out = PyNumber_Subtract(minusOne, bignum);
	    Py_DECREF(minusOne);
	    Py_DECREF(bignum);
	} else {
	    out = PyLong_FromLongLong((long long)(((long long)-1) - aux));
	}
        if (out == NULL) {
            PyErr_SetString(PyExc_RuntimeError, "unknown error decoding NEGINT");
        }
        return out;
    case CBOR_BYTES:
	if (cbor_info == CBOR_VAR_FOLLOWS) {
	    size_t total = 0;
	    VarBufferPart* parts = NULL;
	    VarBufferPart* parts_tail = NULL;
	    uint8_t sc;
	    if (rin->read1(rin, &sc)) { logprintf("r1 fail in var bytes tag\n"); return NULL; }
	    while (sc != CBOR_BREAK) {
		uint8_t scbor_type = sc & CBOR_TYPE_MASK;
		uint8_t scbor_info = sc & CBOR_INFO_BITS;
		uint64_t saux;
		void* blob;

		if (scbor_type != CBOR_BYTES) {
		    PyErr_Format(PyExc_ValueError, "expected subordinate BYTES block under VAR BYTES, but got %x", scbor_type);
		    return NULL;
		}
		if(handle_info_bits(rin, scbor_info, &saux)) { logprintf("var bytes sub infobits failed\n"); return NULL; }
		blob = rin->read(rin, saux);
		if (!blob) { logprintf("var bytes sub bytes read failed\n"); return NULL; }
		if (parts_tail == NULL) {
		    parts = parts_tail = (VarBufferPart*)PyMem_Malloc(sizeof(VarBufferPart) + saux);
		} else {
		    parts_tail->next = (VarBufferPart*)PyMem_Malloc(sizeof(VarBufferPart) + saux);
		    parts_tail = parts_tail->next;
		}
                parts_tail->start = (void*)(parts_tail + 1);
                memcpy(parts_tail->start, blob, saux);
                rin->return_buffer(rin, blob);
		parts_tail->len = saux;
		parts_tail->next = NULL;
		total += saux;
		if (rin->read1(rin, &sc)) { logprintf("r1 fail in var bytes tag\n"); return NULL; }
	    }
	    // Done
	    {
		uint8_t* allbytes = (uint8_t*)PyMem_Malloc(total);
		uintptr_t op = 0;
		while (parts != NULL) {
		    VarBufferPart* next;
		    memcpy(allbytes + op, parts->start, parts->len);
		    op += parts->len;
		    next = parts->next;
		    PyMem_Free(parts);
		    parts = next;
		}
		out = PyBytes_FromStringAndSize((char*)allbytes, total);
		PyMem_Free(allbytes);
	    }
            if (out == NULL) {
                PyErr_SetString(PyExc_RuntimeError, "unknown error decoding VAR BYTES");
            }
	} else {
	    void* raw;
	    if (aux == 0) {
		static void* empty_string = "";
		raw = empty_string;
	    } else {
		raw = rin->read(rin, aux);
		if (!raw) { logprintf("bytes read failed\n"); return NULL; }
	    }
	    out = PyBytes_FromStringAndSize(raw, (Py_ssize_t)aux);
            if (out == NULL) {
                PyErr_SetString(PyExc_RuntimeError, "unknown error decoding BYTES");
            }
            if (aux != 0) {
                rin->return_buffer(rin, raw);
            }
	}
        return out;
    case CBOR_TEXT:
	if (cbor_info == CBOR_VAR_FOLLOWS) {
	    PyObject* parts = PyList_New(0);
	    PyObject* joiner = PyUnicode_FromString("");
	    uint8_t sc;
	    if (rin->read1(rin, &sc)) { logprintf("r1 fail in var text tag\n"); return NULL; }
	    while (sc != CBOR_BREAK) {
		PyObject* subitem = inner_loads_c(rin, sc);
		if (subitem == NULL) { logprintf("fail in var text subitem\n"); return NULL; }
		PyList_Append(parts, subitem);
                Py_DECREF(subitem);
		if (rin->read1(rin, &sc)) { logprintf("r1 fail in var text tag\n"); return NULL; }
	    }
	    // Done
	    out = PyUnicode_Join(joiner, parts);
	    Py_DECREF(joiner);
	    Py_DECREF(parts);
            if (out == NULL) {
                PyErr_SetString(PyExc_RuntimeError, "unknown error decoding VAR TEXT");
            }
	} else {
            void* raw;
	    if (aux == 0) {
		static void* empty_string = "";
		raw = empty_string;
	    } else {
                raw = rin->read(rin, aux);
                if (!raw) { logprintf("read text failed\n"); return NULL; }
            }
	    out = PyUnicode_FromStringAndSize((char*)raw, (Py_ssize_t)aux);
            if (out == NULL) {
                PyErr_SetString(PyExc_RuntimeError, "unknown error decoding TEXT");
            }
            if (aux != 0) {
                rin->return_buffer(rin, raw);
            }
	}
        return out;
    case CBOR_ARRAY:
	if (cbor_info == CBOR_VAR_FOLLOWS) {
	    uint8_t sc;
	    out = PyList_New(0);
	    if (rin->read1(rin, &sc)) { logprintf("r1 fail in var array tag\n"); return NULL; }
	    while (sc != CBOR_BREAK) {
		PyObject* subitem = inner_loads_c(rin, sc);
		if (subitem == NULL) { logprintf("fail in var array subitem\n"); return NULL; }
		PyList_Append(out, subitem);
                Py_DECREF(subitem);
		if (rin->read1(rin, &sc)) { logprintf("r1 fail in var array tag\n"); return NULL; }
	    }
	    // Done
            if (out == NULL) {
                PyErr_SetString(PyExc_RuntimeError, "unknown error decoding VAR ARRAY");
            }
	} else {
            unsigned int i;
	    out = PyList_New((Py_ssize_t)aux);
	    for (i = 0; i < aux; i++) {
		PyObject* subitem = inner_loads(rin);
		if (subitem == NULL) { logprintf("array subitem[%d] (of %d) failed\n", i, aux); return NULL; }
		PyList_SetItem(out, (Py_ssize_t)i, subitem);
                // PyList_SetItem became the owner of the reference count of subitem, we don't need to DECREF it
	    }
            if (out == NULL) {
                PyErr_SetString(PyExc_RuntimeError, "unknown error decoding ARRAY");
            }
	}
        return out;
    case CBOR_MAP:
	out = PyDict_New();
	if (cbor_info == CBOR_VAR_FOLLOWS) {
	    uint8_t sc;
	    if (rin->read1(rin, &sc)) { logprintf("r1 fail in var map tag\n"); return NULL; }
	    while (sc != CBOR_BREAK) {
		PyObject* key = inner_loads_c(rin, sc);
		PyObject* value;
		if (key == NULL) { logprintf("var map key fail\n"); return NULL; }
		value = inner_loads(rin);
		if (value == NULL) { logprintf("var map val vail\n"); return NULL; }
		PyDict_SetItem(out, key, value);
                Py_DECREF(key);
                Py_DECREF(value);

		if (rin->read1(rin, &sc)) { logprintf("r1 fail in var map tag\n"); return NULL; }
	    }
            if (out == NULL) {
                PyErr_SetString(PyExc_RuntimeError, "unknown error decoding VAR MAP");
            }
	} else {
            unsigned int i;
	    for (i = 0; i < aux; i++) {
		if (loads_kv(out, rin) != 0) {
		    logprintf("map kv[%d] failed\n", i);
		    return NULL;
		}
	    }
            if (out == NULL) {
                PyErr_SetString(PyExc_RuntimeError, "unknown error decoding MAP");
            }
	}
        return out;
    case CBOR_TAG:
	return loads_tag(rin, aux);
    case CBOR_7:
	if (aux == 20) {
	    out = Py_False;
	    Py_INCREF(out);
	} else if (aux == 21) {
	    out = Py_True;
	    Py_INCREF(out);
	} else if (aux == 22) {
	    out = Py_None;
	    Py_INCREF(out);
	} else if (aux == 23) {
            // js `undefined`, closest is py None
	    out = Py_None;
	    Py_INCREF(out);
	}
        if (out == NULL) {
            PyErr_Format(PyExc_ValueError, "unknown section 7 marker %02x, aux=%llu", c, aux);
        }
        return out;
    default:
        PyErr_Format(PyExc_RuntimeError, "unknown cbor marker %02x", c);
        return NULL;
    }
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wunreachable-code"
    PyErr_SetString(PyExc_RuntimeError, "cbor library internal error moof!");
    return NULL;
#pragma GCC diagnostic pop
}

static int loads_kv(PyObject* out, Reader* rin) {
    PyObject* key = inner_loads(rin);
    PyObject* value;
    if (key == NULL) { logprintf("map key fail\n"); return -1; }
    value = inner_loads(rin);
    if (value == NULL) { logprintf("map val fail\n"); return -1; }
    PyDict_SetItem(out, key, value);
    Py_DECREF(key);
    Py_DECREF(value);
    return 0;
}

static PyObject* loads_bignum(Reader* rin, uint8_t c) {
    PyObject* out = NULL;

    uint8_t bytes_info = c & CBOR_INFO_BITS;
    if (bytes_info < 24) {
        int i;
	PyObject* eight = PyLong_FromLong(8);
	out = PyLong_FromLong(0);
	for (i = 0; i < bytes_info; i++) {
	    // TODO: is this leaking like crazy?
	    PyObject* curbyte;
	    PyObject* tout = PyNumber_Lshift(out, eight);
	    Py_DECREF(out);
	    out = tout;
	    uint8_t cb;
	    if (rin->read1(rin, &cb)) {
                logprintf("r1 fail in bignum %d/%d\n", i, bytes_info);
                Py_DECREF(eight);
                Py_DECREF(out);
                return NULL;
            }
	    curbyte = PyLong_FromLong(cb);
	    tout = PyNumber_Or(out, curbyte);
	    Py_DECREF(curbyte);
	    Py_DECREF(out);
	    out = tout;
	}
        Py_DECREF(eight);
	return out;
    } else {
	PyErr_Format(PyExc_NotImplementedError, "TODO: TAG BIGNUM for bigger bignum bytes_info=%d, len(ull)=%lu\n", bytes_info, sizeof(unsigned long long));
	return NULL;
    }
}


// returns a PyObject for cbor.cbor.Tag
// Returned PyObject* is a BORROWED reference from the module dict
static PyObject* getCborTagClass(void) {
    PyObject* cbor_module = PyImport_ImportModule("cbor.cbor");
    PyObject* moddict = PyModule_GetDict(cbor_module);
    PyObject* tag_class = PyDict_GetItemString(moddict, "Tag");
    // moddict and tag_class are 'borrowed reference'
    Py_DECREF(cbor_module);

    return tag_class;
}


static PyObject* loads_tag(Reader* rin, uint64_t aux) {
    PyObject* out = NULL;
    // return an object CBORTag(tagnum, nextob)
    if (aux == CBOR_TAG_BIGNUM) {
	// If the next object is bytes, interpret it here without making a PyObject for it.
	uint8_t sc;
	if (rin->read1(rin, &sc)) { logprintf("r1 fail in bignum tag\n"); return NULL; }
	if ((sc & CBOR_TYPE_MASK) == CBOR_BYTES) {
	    return loads_bignum(rin, sc);
	} else {
	    PyErr_Format(PyExc_ValueError, "TAG BIGNUM not followed by bytes but %02x", sc);
	    return NULL;
	}
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wunreachable-code"
	PyErr_Format(PyExc_ValueError, "TODO: WRITEME CBOR TAG BIGNUM %02x ...\n", sc);
	return NULL;
#pragma GCC diagnostic pop
    } else if (aux == CBOR_TAG_NEGBIGNUM) {
	// If the next object is bytes, interpret it here without making a PyObject for it.
	uint8_t sc;
	if (rin->read1(rin, &sc)) { logprintf("r1 fail in negbignum tag\n"); return NULL; }
	if ((sc & CBOR_TYPE_MASK) == CBOR_BYTES) {
	    out = loads_bignum(rin, sc);
            if (out == NULL) { logprintf("loads_bignum fail inside TAG_NEGBIGNUM\n"); return NULL; }
            PyObject* minusOne = PyLong_FromLong(-1);
            PyObject* tout = PyNumber_Subtract(minusOne, out);
            Py_DECREF(minusOne);
            Py_DECREF(out);
            out = tout;
            return out;
	} else {
	    PyErr_Format(PyExc_ValueError, "TAG NEGBIGNUM not followed by bytes but %02x", sc);
	    return NULL;
	}
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wunreachable-code"
	PyErr_Format(PyExc_ValueError, "TODO: WRITEME CBOR TAG NEGBIGNUM %02x ...\n", sc);
	return NULL;
#pragma GCC diagnostic pop
    }
    out = inner_loads(rin);
    if (out == NULL) { return NULL; }
    {
        PyObject* tag_class = getCborTagClass();
	PyObject* args = Py_BuildValue("(K,O)", aux, out);
        PyObject* tout = PyObject_CallObject(tag_class, args);
	Py_DECREF(args);
	Py_DECREF(out);
        // tag_class was just a borrowed reference
	out = tout;
    }
    return out;
}


static PyObject*
cbor_loads(PyObject* noself, PyObject* args) {
    PyObject* ob;
    is_big_endian();
    if (PyType_IsSubtype(Py_TYPE(args), &PyList_Type)) {
	ob = PyList_GetItem(args, 0);
    } else if (PyType_IsSubtype(Py_TYPE(args), &PyTuple_Type)) {
	ob = PyTuple_GetItem(args, 0);
    } else {
	PyErr_Format(PyExc_ValueError, "args not list or tuple: %R\n", args);
	return NULL;
    }

    if (ob == Py_None) {
	PyErr_SetString(PyExc_ValueError, "got None for buffer to decode in loads");
	return NULL;
    }

    {
        PyObject* out = NULL;
	Reader* r = NewBufferReader(ob);
	if (!r) {
	    return NULL;
	}
	out = inner_loads(r);
        r->delete(r);
        return out;
    }
}


#if HAS_FILE_READER

typedef struct _FileReader {
    READER_FUNCTIONS;
    FILE* fin;
    void* dst;
    Py_ssize_t dst_size;
    Py_ssize_t read_count;
} FileReader;

// read from a python builtin file which contains a C FILE*
static void* FileReader_read(void* self, Py_ssize_t len) {
    FileReader* thiz = (FileReader*)self;
    Py_ssize_t rtotal = 0;
    uintptr_t opos;
    //logprintf("file read %d\n", len);
    if (len > thiz->dst_size) {
	thiz->dst = PyMem_Realloc(thiz->dst, len);
	thiz->dst_size = len;
    } else if ((thiz->dst_size > (128 * 1024)) && (len < 4096)) {
	PyMem_Free(thiz->dst);
	thiz->dst = PyMem_Malloc(len);
	thiz->dst_size = len;
    }
    opos = (uintptr_t)(thiz->dst);
    while (1) {
	size_t rlen = fread((void*)opos, 1, len, thiz->fin);
	if (rlen == 0) {
	    // file isn't going to give any more
	    PyErr_Format(PyExc_ValueError, "only got %zd bytes with %zd stil to read from file", rtotal, len);
	    PyMem_Free(thiz->dst);
	    thiz->dst = NULL;
            thiz->dst_size = 0;
	    return NULL;
	}
	thiz->read_count += rlen;
	rtotal += rlen;
	opos += rlen;
	len -= rlen;
	if (rtotal >= len) {
            if (thiz->dst == NULL) {
                PyErr_SetString(PyExc_RuntimeError, "known error in file reader, NULL dst");
                return NULL;
            }
	    return thiz->dst;
	}
    }
}
static int FileReader_read1(void* self, uint8_t* oneByte) {
    FileReader* thiz = (FileReader*)self;
    size_t didread = fread((void*)oneByte, 1, 1, thiz->fin);
    if (didread == 0) {
	logprintf("failed to read 1 from file\n");
	PyErr_SetString(PyExc_ValueError, "got nothing reading 1 from file");
	return -1;
    }
    thiz->read_count++;
    return 0;
}
static void FileReader_return_buffer(void* self, void* buffer) {
    // Nothing to do, we hold onto the buffer and maybe reuse it for next read
}
static void FileReader_delete(void* self) {
    FileReader* thiz = (FileReader*)self;
    if (thiz->dst) {
	PyMem_Free(thiz->dst);
    }
    PyMem_Free(thiz);
}
static Reader* NewFileReader(PyObject* ob) {
    FileReader* fr = (FileReader*)PyMem_Malloc(sizeof(FileReader));
    if (fr == NULL) {
        PyErr_SetString(PyExc_MemoryError, "failed to allocate FileReader");
        return NULL;
    }
    fr->fin = PyFile_AsFile(ob);
    if (fr->fin == NULL) {
        PyErr_SetString(PyExc_RuntimeError, "PyFile_AsFile NULL");
        PyMem_Free(fr);
        return NULL;
    }
    fr->dst = NULL;
    fr->dst_size = 0;
    fr->read_count = 0;
    SET_READER_FUNCTIONS(fr, FileReader);
    return (Reader*)fr;
}

#endif /* Python 2.7 FileReader */


typedef struct _ObjectReader {
    READER_FUNCTIONS;
    PyObject* ob;

    // We got one object with all the bytes neccessary, and need to
    // DECREF it later.
    PyObject* retval;
    void* bytes;

    // OR, we got several objects, we DECREFed them as we went, and
    // need to Free() this buffer at the end.
    void* dst;

    Py_ssize_t read_count;
    int exception_is_external;
} ObjectReader;

// read from a python file-like object which has a .read(n) method
static void* ObjectReader_read(void* context, Py_ssize_t len) {
    ObjectReader* thiz = (ObjectReader*)context;
    Py_ssize_t rtotal = 0;
    uintptr_t opos = 0;
    //logprintf("ob read %d\n", len);
    assert(!thiz->dst);
    assert(!thiz->bytes);
    while (rtotal < len) {
	PyObject* retval = PyObject_CallMethod(thiz->ob, "read", "n", len - rtotal, NULL);
	Py_ssize_t rlen;
	if (retval == NULL) {
	    thiz->exception_is_external = 1;
            logprintf("exception in object.read()\n");
	    return NULL;
	}
	if (!PyBytes_Check(retval)) {
            logprintf("object.read() is not bytes\n");
	    PyErr_SetString(PyExc_ValueError, "expected ob.read() to return a bytes object\n");
            Py_DECREF(retval);
	    return NULL;
	}
	rlen = PyBytes_Size(retval);
	thiz->read_count += rlen;
	if (rlen > len - rtotal) {
            logprintf("object.read() is too much!\n");
            PyErr_Format(PyExc_ValueError, "ob.read() returned %ld bytes but only wanted %lu\n", rlen, len - rtotal);
            Py_DECREF(retval);
            return NULL;
	}
	if (rlen == len) {
	    // best case! All in one call to read()
	    // We _keep_ a reference to retval until later.
	    thiz->retval = retval;
	    thiz->bytes = PyBytes_AsString(retval);
	    assert(thiz->bytes);
	    thiz->dst = NULL;
	    opos = 0;
	    return thiz->bytes;
	}
	if (thiz->dst == NULL) {
	    thiz->dst = PyMem_Malloc(len);
	    opos = (uintptr_t)thiz->dst;
	}
	// else, not enough all in one go
	memcpy((void*)opos, PyBytes_AsString(retval), rlen);
	Py_DECREF(retval);
	opos += rlen;
	rtotal += rlen;
    }
    assert(thiz->dst);
    return thiz->dst;
}
static int ObjectReader_read1(void* self, uint8_t* oneByte) {
    ObjectReader* thiz = (ObjectReader*)self;
    PyObject* retval = PyObject_CallMethod(thiz->ob, "read", "i", 1, NULL);
    Py_ssize_t rlen;
    if (retval == NULL) {
	thiz->exception_is_external = 1;
	//logprintf("call ob read(1) failed\n");
	return -1;
    }
    if (!PyBytes_Check(retval)) {
	PyErr_SetString(PyExc_ValueError, "expected ob.read() to return a bytes object\n");
	return -1;
    }
    rlen = PyBytes_Size(retval);
    thiz->read_count += rlen;
    if (rlen > 1) {
	PyErr_Format(PyExc_ValueError, "TODO: raise exception: WAT ob.read() returned %ld bytes but only wanted 1\n", rlen);
	return -1;
    }
    if (rlen == 1) {
	*oneByte = PyBytes_AsString(retval)[0];
	Py_DECREF(retval);
	return 0;
    }
    PyErr_SetString(PyExc_ValueError, "got nothing reading 1");
    return -1;
}
static void ObjectReader_return_buffer(void* context, void* buffer) {
    ObjectReader* thiz = (ObjectReader*)context;
    if (buffer == thiz->bytes) {
	Py_DECREF(thiz->retval);
	thiz->retval = NULL;
	thiz->bytes = NULL;
    } else if (buffer == thiz->dst) {
	PyMem_Free(thiz->dst);
	thiz->dst = NULL;
    } else {
	logprintf("TODO: raise exception, could not release buffer %p, wanted dst=%p or bytes=%p\n", buffer, thiz->dst, thiz->bytes);
    }
}
static void ObjectReader_delete(void* context) {
    ObjectReader* thiz = (ObjectReader*)context;
    if (thiz->retval != NULL) {
	Py_DECREF(thiz->retval);
    }
    if (thiz->dst != NULL) {
	PyMem_Free(thiz->dst);
    }
    PyMem_Free(thiz);
}
static Reader* NewObjectReader(PyObject* ob) {
    ObjectReader* r = (ObjectReader*)PyMem_Malloc(sizeof(ObjectReader));
    r->ob = ob;
    r->retval = NULL;
    r->bytes = NULL;
    r->dst = NULL;
    r->read_count = 0;
    r->exception_is_external = 0;
    SET_READER_FUNCTIONS(r, ObjectReader);
    return (Reader*)r;
}

typedef struct _BufferReader {
    READER_FUNCTIONS;
    uint8_t* raw;
    Py_ssize_t len;
    uintptr_t pos;
} BufferReader;

// read from a buffer, aka loads()
static void* BufferReader_read(void* context, Py_ssize_t len) {
    BufferReader* thiz = (BufferReader*)context;
    //logprintf("br %p %d (%d)\n", thiz, len, thiz->len);
    if (len <= thiz->len) {
	void* out = (void*)thiz->pos;
	thiz->pos += len;
	thiz->len -= len;
	assert(out);
	return out;
    }
    PyErr_Format(PyExc_ValueError, "buffer read for %zd but only have %zd\n", len, thiz->len);
    return NULL;
}
static int BufferReader_read1(void* self, uint8_t* oneByte) {
    BufferReader* thiz = (BufferReader*)self;
    //logprintf("br %p _1_ (%d)\n", thiz, thiz->len);
    if (thiz->len <= 0) {
	PyErr_SetString(PyExc_LookupError, "buffer exhausted");
	return -1;
    }
    *oneByte = *((uint8_t*)thiz->pos);
    thiz->pos += 1;
    thiz->len -= 1;
    return 0;
}
static void BufferReader_return_buffer(void* context, void* buffer) {
    // nothing to do
}
static void BufferReader_delete(void* context) {
    BufferReader* thiz = (BufferReader*)context;
    PyMem_Free(thiz);
}
static Reader* NewBufferReader(PyObject* ob) {
    BufferReader* r = (BufferReader*)PyMem_Malloc(sizeof(BufferReader));
    SET_READER_FUNCTIONS(r, BufferReader);
    if (PyByteArray_Check(ob)) {
        r->raw = (uint8_t*)PyByteArray_AsString(ob);
        r->len = PyByteArray_Size(ob);
    } else if (PyBytes_Check(ob)) {
        r->raw = (uint8_t*)PyBytes_AsString(ob);
        r->len = PyBytes_Size(ob);
    } else {
        PyErr_SetString(PyExc_ValueError, "input of unknown type not bytes or bytearray");
        return NULL;
    }
    r->pos = (uintptr_t)r->raw;
    if (r->len == 0) {
	PyErr_SetString(PyExc_ValueError, "got zero length string in loads");
	return NULL;
    }
    if (r->raw == NULL) {
	PyErr_SetString(PyExc_ValueError, "got NULL buffer for string");
	return NULL;
    }
    //logprintf("NBR(%llu, %ld)\n", r->pos, r->len);
    return (Reader*)r;
}


static PyObject*
cbor_load(PyObject* noself, PyObject* args) {
    PyObject* ob;
    Reader* reader;
    is_big_endian();
    if (PyType_IsSubtype(Py_TYPE(args), &PyList_Type)) {
	ob = PyList_GetItem(args, 0);
    } else if (PyType_IsSubtype(Py_TYPE(args), &PyTuple_Type)) {
	ob = PyTuple_GetItem(args, 0);
    } else {
	PyErr_Format(PyExc_ValueError, "args not list or tuple: %R\n", args);
	return NULL;
    }

    if (ob == Py_None) {
	PyErr_SetString(PyExc_ValueError, "got None for buffer to decode in loads");
	return NULL;
    }
    PyObject* retval;
#if HAS_FILE_READER
    if (PyFile_Check(ob)) {
	reader = NewFileReader(ob);
        if (reader == NULL) { return NULL; }
	retval = inner_loads(reader);
        if ((retval == NULL) &&
            (((FileReader*)reader)->read_count == 0) &&
            (feof(((FileReader*)reader)->fin) != 0)) {
	    // never got anything, started at EOF
	    PyErr_Clear();
	    PyErr_SetString(PyExc_EOFError, "read nothing, apparent EOF");
        }
        reader->delete(reader);
    } else
#endif
    {
	reader = NewObjectReader(ob);
	retval = inner_loads(reader);
	if ((retval == NULL) &&
	    (!((ObjectReader*)reader)->exception_is_external) &&
	    ((ObjectReader*)reader)->read_count == 0) {
	    // never got anything, assume EOF
	    PyErr_Clear();
	    PyErr_SetString(PyExc_EOFError, "read nothing, apparent EOF");
	}
        reader->delete(reader);
    }
    return retval;
}


static void tag_u64_out(uint8_t cbor_type, uint64_t aux, uint8_t* out, uintptr_t* posp) {
    uintptr_t pos = *posp;
    if (out != NULL) {
	out[pos] = cbor_type | CBOR_UINT64_FOLLOWS;
	out[pos+1] = (aux >> 56) & 0x0ff;
	out[pos+2] = (aux >> 48) & 0x0ff;
	out[pos+3] = (aux >> 40) & 0x0ff;
	out[pos+4] = (aux >> 32) & 0x0ff;
	out[pos+5] = (aux >> 24) & 0x0ff;
	out[pos+6] = (aux >> 16) & 0x0ff;
	out[pos+7] = (aux >>  8) & 0x0ff;
	out[pos+8] = aux & 0x0ff;
    }
    pos += 9;
    *posp = pos;
}


static void tag_aux_out(uint8_t cbor_type, uint64_t aux, uint8_t* out, uintptr_t* posp) {
    uintptr_t pos = *posp;
    if (aux <= 23) {
	// tiny literal
	if (out != NULL) {
	    out[pos] = cbor_type | aux;
	}
	pos += 1;
    } else if (aux <= 0x0ff) {
	// one byte value
	if (out != NULL) {
	    out[pos] = cbor_type | CBOR_UINT8_FOLLOWS;
	    out[pos+1] = aux;
	}
	pos += 2;
    } else if (aux <= 0x0ffff) {
	// two byte value
	if (out != NULL) {
	    out[pos] = cbor_type | CBOR_UINT16_FOLLOWS;
	    out[pos+1] = (aux >> 8) & 0x0ff;
	    out[pos+2] = aux & 0x0ff;
	}
	pos += 3;
    } else if (aux <= 0x0ffffffffL) {
	// four byte value
	if (out != NULL) {
	    out[pos] = cbor_type | CBOR_UINT32_FOLLOWS;
	    out[pos+1] = (aux >> 24) & 0x0ff;
	    out[pos+2] = (aux >> 16) & 0x0ff;
	    out[pos+3] = (aux >>  8) & 0x0ff;
	    out[pos+4] = aux & 0x0ff;
	}
	pos += 5;
    } else {
	// eight byte value
	tag_u64_out(cbor_type, aux, out, posp);
	return;
    }
    *posp = pos;
    return;
}

static int inner_dumps(EncodeOptions *optp, PyObject* ob, uint8_t* out, uintptr_t* posp);

static int dumps_dict(EncodeOptions *optp, PyObject* ob, uint8_t* out, uintptr_t* posp) {
    uintptr_t pos = *posp;
    Py_ssize_t dictlen = PyDict_Size(ob);
    PyObject* key;
    PyObject* val;
    int err;

    tag_aux_out(CBOR_MAP, dictlen, out, &pos);

    if (optp->sort_keys) {
        Py_ssize_t index = 0;
        PyObject* keylist = PyDict_Keys(ob);
        PyList_Sort(keylist);

        //fprintf(stderr, "sortking keys\n");
        for (index = 0; index < PyList_Size(keylist); index++) {
            key = PyList_GetItem(keylist, index); // Borrowed ref
            val = PyDict_GetItem(ob, key); // Borrowed ref
            err = inner_dumps(optp, key, out, &pos);
            if (err != 0) { return err; }
            err = inner_dumps(optp, val, out, &pos);
            if (err != 0) { return err; }
        }
        Py_DECREF(keylist);
    } else {
        Py_ssize_t dictiter = 0;
        //fprintf(stderr, "unsorted keys\n");
        while (PyDict_Next(ob, &dictiter, &key, &val)) {
            err = inner_dumps(optp, key, out, &pos);
            if (err != 0) { return err; }
            err = inner_dumps(optp, val, out, &pos);
            if (err != 0) { return err; }
        }
    }

    *posp = pos;
    return 0;
}


static void dumps_bignum(EncodeOptions *optp, uint8_t tag, PyObject* val, uint8_t* out, uintptr_t* posp) {
    uintptr_t pos = (posp != NULL) ? *posp : 0;
    PyObject* eight = PyLong_FromLong(8);
    PyObject* bytemask = NULL;
    PyObject* nval = NULL;
    uint8_t* revbytes = NULL;
    int revbytepos = 0;
    int val_is_orig = 1;
    if (out != NULL) {
	bytemask = PyLong_FromLongLong(0x0ff);
	revbytes = PyMem_Malloc(23);
    }
    while (PyObject_IsTrue(val) && (revbytepos < 23)) {
	if (revbytes != NULL) {
	    PyObject* tbyte = PyNumber_And(val, bytemask);
	    revbytes[revbytepos] = PyLong_AsLong(tbyte);
	    Py_DECREF(tbyte);
	}
	revbytepos++;
	nval = PyNumber_InPlaceRshift(val, eight);
        if (val_is_orig) {
            val_is_orig = 0;
        } else {
            Py_DECREF(val);
        }
        val = nval;
    }
    if (revbytes != NULL) {
	out[pos] = CBOR_TAG | tag;
	pos++;
	out[pos] = CBOR_BYTES | revbytepos;
	pos++;
	revbytepos--;
	while (revbytepos >= 0) {
	    out[pos] = revbytes[revbytepos];
	    pos++;
	    revbytepos--;
	}
        PyMem_Free(revbytes);
	Py_DECREF(bytemask);
    } else {
	pos += 2 + revbytepos;
    }
    if (!val_is_orig) {
        Py_DECREF(val);
    }
    Py_DECREF(eight);
    *posp = pos;
}

static int dumps_tag(EncodeOptions *optp, PyObject* ob, uint8_t* out, uintptr_t* posp) {
    uintptr_t pos = (posp != NULL) ? *posp : 0;
    int err = 0;


    PyObject* tag_num;
    PyObject* tag_value;
    err = 0;

    tag_num = PyObject_GetAttrString(ob, "tag");
    if (tag_num != NULL) {
        tag_value = PyObject_GetAttrString(ob, "value");
        if (tag_value != NULL) {
#ifdef Py_INTOBJECT_H
            if (PyInt_Check(tag_num)) {
                long val = PyInt_AsLong(tag_num);
                if (val >= 0) {
                    tag_aux_out(CBOR_TAG, val, out, &pos);
                    err = inner_dumps(optp, tag_value, out, &pos);
                } else {
                    PyErr_Format(PyExc_ValueError, "tag cannot be a negative int: %ld", val);
                    err = -1;
                }
            } else
#endif
            if (PyLong_Check(tag_num)) {
                int overflow = -1;
                long long val = PyLong_AsLongLongAndOverflow(tag_num, &overflow);
                if (overflow == 0) {
                    if (val >= 0) {
                        tag_aux_out(CBOR_TAG, val, out, &pos);
                        err = inner_dumps(optp, tag_value, out, &pos);
                    } else {
                        PyErr_Format(PyExc_ValueError, "tag cannot be a negative long: %lld", val);
                        err = -1;
                    }
                } else {
                    PyErr_SetString(PyExc_ValueError, "tag number too large");
                    err = -1;
                }
            }
            Py_DECREF(tag_value);
        } else {
            PyErr_SetString(PyExc_ValueError, "broken Tag object has .tag but not .value");
            err = -1;
        }
        Py_DECREF(tag_num);
    } else {
        PyErr_SetString(PyExc_ValueError, "broken Tag object with no .tag");
        err = -1;
    }
    if (err != 0) { return err; }

    *posp = pos;
    return err;
}


// With out=NULL it just counts the length.
// return err, 0=OK
static int inner_dumps(EncodeOptions *optp, PyObject* ob, uint8_t* out, uintptr_t* posp) {
    uintptr_t pos = (posp != NULL) ? *posp : 0;

    if (ob == Py_None) {
	if (out != NULL) {
	    out[pos] = CBOR_NULL;
	}
	pos += 1;
    } else if (PyBool_Check(ob)) {
	if (out != NULL) {
	    if (PyObject_IsTrue(ob)) {
		out[pos] = CBOR_TRUE;
	    } else {
		out[pos] = CBOR_FALSE;
	    }
	}
	pos += 1;
    } else if (PyDict_Check(ob)) {
	int err = dumps_dict(optp, ob, out, &pos);
	if (err != 0) { return err; }
    } else if (PyList_Check(ob)) {
        Py_ssize_t i;
	Py_ssize_t listlen = PyList_Size(ob);
	tag_aux_out(CBOR_ARRAY, listlen, out, &pos);
	for (i = 0; i < listlen; i++) {
	    int err = inner_dumps(optp, PyList_GetItem(ob, i), out, &pos);
	    if (err != 0) { return err; }
	}
    } else if (PyTuple_Check(ob)) {
        Py_ssize_t i;
	Py_ssize_t listlen = PyTuple_Size(ob);
	tag_aux_out(CBOR_ARRAY, listlen, out, &pos);
	for (i = 0; i < listlen; i++) {
	    int err = inner_dumps(optp, PyTuple_GetItem(ob, i), out, &pos);
	    if (err != 0) { return err; }
	}
	// TODO: accept other enumerables and emit a variable length array
#ifdef Py_INTOBJECT_H
	// PyInt exists in Python 2 but not 3
    } else if (PyInt_Check(ob)) {
	long val = PyInt_AsLong(ob);
	if (val >= 0) {
	    tag_aux_out(CBOR_UINT, val, out, &pos);
	} else {
	    tag_aux_out(CBOR_NEGINT, -1 - val, out, &pos);
	}
#endif
    } else if (PyLong_Check(ob)) {
	int overflow = 0;
	long long val = PyLong_AsLongLongAndOverflow(ob, &overflow);
	if (overflow == 0) {
	    if (val >= 0) {
		tag_aux_out(CBOR_UINT, val, out, &pos);
	    } else {
		tag_aux_out(CBOR_NEGINT, -1L - val, out, &pos);
	    }
	} else {
	    if (overflow < 0) {
		// BIG NEGINT
		PyObject* minusone = PyLong_FromLongLong(-1L);
		PyObject* val = PyNumber_Subtract(minusone, ob);
		Py_DECREF(minusone);
		dumps_bignum(optp, CBOR_TAG_NEGBIGNUM, val, out, &pos);
		Py_DECREF(val);
	    } else {
		// BIG INT
		dumps_bignum(optp, CBOR_TAG_BIGNUM, ob, out, &pos);
	    }
	}
    } else if (PyFloat_Check(ob)) {
	double val = PyFloat_AsDouble(ob);
	tag_u64_out(CBOR_7, *((uint64_t*)(&val)), out, &pos);
    } else if (PyBytes_Check(ob)) {
	Py_ssize_t len = PyBytes_Size(ob);
	tag_aux_out(CBOR_BYTES, len, out, &pos);
	if (out != NULL) {
	    memcpy(out + pos, PyBytes_AsString(ob), len);
	}
	pos += len;
    } else if (PyUnicode_Check(ob)) {
	PyObject* utf8 = PyUnicode_AsUTF8String(ob);
	Py_ssize_t len = PyBytes_Size(utf8);
	tag_aux_out(CBOR_TEXT, len, out, &pos);
	if (out != NULL) {
	    memcpy(out + pos, PyBytes_AsString(utf8), len);
	}
	pos += len;
	Py_DECREF(utf8);
    } else {
        int handled = 0;
        {
            PyObject* tag_class = getCborTagClass();
            if (PyObject_IsInstance(ob, tag_class)) {
                int err = dumps_tag(optp, ob, out, &pos);
                if (err != 0) { return err; }
                handled = 1;
            }
            // tag_class was just a borrowed reference
        }

        // TODO: other special object serializations here

        if (!handled) {
#if IS_PY3
            PyErr_Format(PyExc_ValueError, "cannot serialize unknown object: %R", ob);
#else
            PyObject* badtype = PyObject_Type(ob);
            PyObject* badtypename = PyObject_Str(badtype);
            PyErr_Format(PyExc_ValueError, "cannot serialize unknown object of type %s", PyString_AsString(badtypename));
            Py_DECREF(badtypename);
            Py_DECREF(badtype);
#endif
            return -1;
        }
    }
    if (posp != NULL) {
	*posp = pos;
    }
    return 0;
}

static int _dumps_kwargs(EncodeOptions *optp, PyObject* kwargs) {
    if (kwargs == NULL) {
    } else if (!PyDict_Check(kwargs)) {
	PyErr_Format(PyExc_ValueError, "kwargs not dict: %R\n", kwargs);
	return 0;
    } else {
	PyObject* sort_keys = PyDict_GetItemString(kwargs, "sort_keys");  // Borrowed ref
	if (sort_keys != NULL) {
            optp->sort_keys = PyObject_IsTrue(sort_keys);
            //fprintf(stderr, "sort_keys=%d\n", optp->sort_keys);
	}
    }
    return 1;
}

static PyObject*
cbor_dumps(PyObject* noself, PyObject* args, PyObject* kwargs) {

    PyObject* ob;
    EncodeOptions opts = {0};
    EncodeOptions *optp = &opts;
    is_big_endian();
    if (PyType_IsSubtype(Py_TYPE(args), &PyList_Type)) {
	ob = PyList_GetItem(args, 0);
    } else if (PyType_IsSubtype(Py_TYPE(args), &PyTuple_Type)) {
	ob = PyTuple_GetItem(args, 0);
    } else {
	PyErr_Format(PyExc_ValueError, "args not list or tuple: %R\n", args);
	return NULL;
    }
    if (ob == NULL) {
        return NULL;
    }

    if (!_dumps_kwargs(optp, kwargs)) {
        return NULL;
    }

    {
	Py_ssize_t outlen = 0;
	uintptr_t pos = 0;
	void* out = NULL;
	PyObject* obout = NULL;
	int err;

	// first pass just to count length
	err = inner_dumps(optp, ob, NULL, &pos);
	if (err != 0) {
	    return NULL;
	}

	outlen = pos;

	out = PyMem_Malloc(outlen);
	if (out == NULL) {
	    PyErr_NoMemory();
	    return NULL;
	}

	err = inner_dumps(optp, ob, out, NULL);
	if (err != 0) {
	    PyMem_Free(out);
	    return NULL;
	}

	// TODO: I wish there was a way to do this without this copy.
	obout = PyBytes_FromStringAndSize(out, outlen);
	PyMem_Free(out);
	return obout;
    }
}

static PyObject*
cbor_dump(PyObject* noself, PyObject* args, PyObject *kwargs) {
    // args should be (obj, fp)
    PyObject* ob;
    PyObject* fp;
    EncodeOptions opts = {0};
    EncodeOptions *optp = &opts;

    is_big_endian();
    if (PyType_IsSubtype(Py_TYPE(args), &PyList_Type)) {
	ob = PyList_GetItem(args, 0);
	fp = PyList_GetItem(args, 1);
    } else if (PyType_IsSubtype(Py_TYPE(args), &PyTuple_Type)) {
	ob = PyTuple_GetItem(args, 0);
	fp = PyTuple_GetItem(args, 1);
    } else {
	PyErr_Format(PyExc_ValueError, "args not list or tuple: %R\n", args);
	return NULL;
    }
    if ((ob == NULL) || (fp == NULL)) {
        return NULL;
    }

    if (!_dumps_kwargs(optp, kwargs)) {
        return NULL;
    }

    {
	// TODO: make this smarter, right now it is justt fp.write(dumps(ob))
	Py_ssize_t outlen = 0;
	uintptr_t pos = 0;
	void* out = NULL;
	int err;

	// first pass just to count length
	err = inner_dumps(optp, ob, NULL, &pos);
	if (err != 0) {
	    return NULL;
	}

	outlen = pos;

	out = PyMem_Malloc(outlen);
	if (out == NULL) {
	    PyErr_NoMemory();
	    return NULL;
	}

	err = inner_dumps(optp, ob, out, NULL);
	if (err != 0) {
	    PyMem_Free(out);
	    return NULL;
	}

#if HAS_FILE_READER
	if (PyFile_Check(fp)) {
	    FILE* fout = PyFile_AsFile(fp);
	    fwrite(out, 1, outlen, fout);
	} else
#endif
	{
	    PyObject* ret;
            PyObject* obout = NULL;
#if IS_PY3
	    PyObject* writeStr = PyUnicode_FromString("write");
#else
	    PyObject* writeStr = PyString_FromString("write");
#endif
	    obout = PyBytes_FromStringAndSize(out, outlen);
	    //logprintf("write %zd bytes to %p.write() as %p\n", outlen, fp, obout);
	    ret = PyObject_CallMethodObjArgs(fp, writeStr, obout, NULL);
	    Py_DECREF(writeStr);
	    Py_DECREF(obout);
	    if (ret != NULL) {
		Py_DECREF(ret);
	    } else {
		// exception in fp.write()
		PyMem_Free(out);
		return NULL;
	    }
	    //logprintf("wrote %zd bytes to %p.write() as %p\n", outlen, fp, obout);
	}
	PyMem_Free(out);
    }

    Py_RETURN_NONE;
}


static PyMethodDef CborMethods[] = {
    {"loads",  cbor_loads, METH_VARARGS,
        "parse cbor from data buffer to objects"},
    {"dumps", (PyCFunction)cbor_dumps, METH_VARARGS|METH_KEYWORDS,
        "serialize python object to bytes"},
    {"load",  cbor_load, METH_VARARGS,
     "Parse cbor from data buffer to objects.\n"
     "Takes a file-like object capable of .read(N)\n"},
    {"dump", (PyCFunction)cbor_dump, METH_VARARGS|METH_KEYWORDS,
     "Serialize python object to bytes.\n"
     "dump(obj, fp)\n"
     "obj: object to output; fp: file-like object to .write() to\n"},
    {NULL, NULL, 0, NULL}        /* Sentinel */
};

#ifdef Py_InitModule
// Python 2.7
PyMODINIT_FUNC
init_cbor(void)
{
    (void) Py_InitModule("cbor._cbor", CborMethods);
}
#else
// Python 3
PyMODINIT_FUNC
PyInit__cbor(void)
{
    static PyModuleDef modef = {
	PyModuleDef_HEAD_INIT,
    };
    //modef.m_base = PyModuleDef_HEAD_INIT;
    modef.m_name = "cbor._cbor";
    modef.m_doc = NULL;
    modef.m_size = 0;
    modef.m_methods = CborMethods;
#ifdef Py_mod_exec
    modef.m_slots = NULL; // Py >= 3.5
#else
    modef.m_reload = NULL; // Py < 3.5
#endif
    modef.m_traverse = NULL;
    modef.m_clear = NULL;
    modef.m_free = NULL;
    return PyModule_Create(&modef);
}
#endif

