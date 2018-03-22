#!python
# -*- Python -*-

import datetime
import re
import struct
import sys

_IS_PY3 = sys.version_info[0] >= 3

if _IS_PY3:
    from io import BytesIO as StringIO
else:
    try:
        from cStringIO import StringIO
    except:
        from StringIO import StringIO


CBOR_TYPE_MASK = 0xE0  # top 3 bits
CBOR_INFO_BITS = 0x1F  # low 5 bits


CBOR_UINT    = 0x00
CBOR_NEGINT  = 0x20
CBOR_BYTES   = 0x40
CBOR_TEXT    = 0x60
CBOR_ARRAY   = 0x80
CBOR_MAP     = 0xA0
CBOR_TAG     = 0xC0
CBOR_7       = 0xE0  # float and other types

CBOR_UINT8_FOLLOWS  = 24  # 0x18
CBOR_UINT16_FOLLOWS = 25  # 0x19
CBOR_UINT32_FOLLOWS = 26  # 0x1a
CBOR_UINT64_FOLLOWS = 27  # 0x1b
CBOR_VAR_FOLLOWS    = 31  # 0x1f

CBOR_BREAK  = 0xFF

CBOR_FALSE  = (CBOR_7 | 20)
CBOR_TRUE   = (CBOR_7 | 21)
CBOR_NULL   = (CBOR_7 | 22)
CBOR_UNDEFINED   = (CBOR_7 | 23)  # js 'undefined' value

CBOR_FLOAT16 = (CBOR_7 | 25)
CBOR_FLOAT32 = (CBOR_7 | 26)
CBOR_FLOAT64 = (CBOR_7 | 27)

CBOR_TAG_DATE_STRING = 0 # RFC3339
CBOR_TAG_DATE_ARRAY = 1 # any number type follows, seconds since 1970-01-01T00:00:00 UTC
CBOR_TAG_BIGNUM = 2 # big endian byte string follows
CBOR_TAG_NEGBIGNUM = 3 # big endian byte string follows
CBOR_TAG_DECIMAL = 4 # [ 10^x exponent, number ]
CBOR_TAG_BIGFLOAT = 5 # [ 2^x exponent, number ]
CBOR_TAG_BASE64URL = 21
CBOR_TAG_BASE64 = 22
CBOR_TAG_BASE16 = 23
CBOR_TAG_CBOR = 24 # following byte string is embedded CBOR data

CBOR_TAG_URI = 32
CBOR_TAG_BASE64URL = 33
CBOR_TAG_BASE64 = 34
CBOR_TAG_REGEX = 35
CBOR_TAG_MIME = 36 # following text is MIME message, headers, separators and all
CBOR_TAG_CBOR_FILEHEADER = 55799 # can open a file with 0xd9d9f7

_CBOR_TAG_BIGNUM_BYTES = struct.pack('B', CBOR_TAG | CBOR_TAG_BIGNUM)


def dumps_int(val):
    "return bytes representing int val in CBOR"
    if val >= 0:
        # CBOR_UINT is 0, so I'm lazy/efficient about not OR-ing it in.
        if val <= 23:
            return struct.pack('B', val)
        if val <= 0x0ff:
            return struct.pack('BB', CBOR_UINT8_FOLLOWS, val)
        if val <= 0x0ffff:
            return struct.pack('!BH', CBOR_UINT16_FOLLOWS, val)
        if val <= 0x0ffffffff:
            return struct.pack('!BI', CBOR_UINT32_FOLLOWS, val)
        if val <= 0x0ffffffffffffffff:
            return struct.pack('!BQ', CBOR_UINT64_FOLLOWS, val)
        outb = _dumps_bignum_to_bytearray(val)
        return _CBOR_TAG_BIGNUM_BYTES + _encode_type_num(CBOR_BYTES, len(outb)) + outb
    val = -1 - val
    return _encode_type_num(CBOR_NEGINT, val)


if _IS_PY3:
    def _dumps_bignum_to_bytearray(val):
        out = []
        while val > 0:
            out.insert(0, val & 0x0ff)
            val = val >> 8
        return bytes(out)
else:
    def _dumps_bignum_to_bytearray(val):
        out = []
        while val > 0:
            out.insert(0, chr(val & 0x0ff))
            val = val >> 8
        return b''.join(out)


def dumps_float(val):
    return struct.pack("!Bd", CBOR_FLOAT64, val)


_CBOR_TAG_NEGBIGNUM_BYTES = struct.pack('B', CBOR_TAG | CBOR_TAG_NEGBIGNUM)


def _encode_type_num(cbor_type, val):
    """For some CBOR primary type [0..7] and an auxiliary unsigned number, return CBOR encoded bytes"""
    assert val >= 0
    if val <= 23:
        return struct.pack('B', cbor_type | val)
    if val <= 0x0ff:
        return struct.pack('BB', cbor_type | CBOR_UINT8_FOLLOWS, val)
    if val <= 0x0ffff:
        return struct.pack('!BH', cbor_type | CBOR_UINT16_FOLLOWS, val)
    if val <= 0x0ffffffff:
        return struct.pack('!BI', cbor_type | CBOR_UINT32_FOLLOWS, val)
    if (((cbor_type == CBOR_NEGINT) and (val <= 0x07fffffffffffffff)) or
        ((cbor_type != CBOR_NEGINT) and (val <= 0x0ffffffffffffffff))):
        return struct.pack('!BQ', cbor_type | CBOR_UINT64_FOLLOWS, val)
    if cbor_type != CBOR_NEGINT:
        raise Exception("value too big for CBOR unsigned number: {0!r}".format(val))
    outb = _dumps_bignum_to_bytearray(val)
    return _CBOR_TAG_NEGBIGNUM_BYTES + _encode_type_num(CBOR_BYTES, len(outb)) + outb


if _IS_PY3:
    def _is_unicode(val):
        return isinstance(val, str)
else:
    def _is_unicode(val):
        return isinstance(val, unicode)


def dumps_string(val, is_text=None, is_bytes=None):
    if _is_unicode(val):
        val = val.encode('utf8')
        is_text = True
        is_bytes = False
    if (is_bytes) or not (is_text == True):
        return _encode_type_num(CBOR_BYTES, len(val)) + val
    return _encode_type_num(CBOR_TEXT, len(val)) + val


def dumps_array(arr, sort_keys=False):
    head = _encode_type_num(CBOR_ARRAY, len(arr))
    parts = [dumps(x, sort_keys=sort_keys) for x in arr]
    return head + b''.join(parts)


if _IS_PY3:
    def dumps_dict(d, sort_keys=False):
        head = _encode_type_num(CBOR_MAP, len(d))
        parts = [head]
        if sort_keys:
            for k in sorted(d.keys()):
                v = d[k]
                parts.append(dumps(k, sort_keys=sort_keys))
                parts.append(dumps(v, sort_keys=sort_keys))
        else:
            for k,v in d.items():
                parts.append(dumps(k, sort_keys=sort_keys))
                parts.append(dumps(v, sort_keys=sort_keys))
        return b''.join(parts)
else:
    def dumps_dict(d, sort_keys=False):
        head = _encode_type_num(CBOR_MAP, len(d))
        parts = [head]
        if sort_keys:
            for k in sorted(d.iterkeys()):
                v = d[k]
                parts.append(dumps(k, sort_keys=sort_keys))
                parts.append(dumps(v, sort_keys=sort_keys))
        else:
            for k,v in d.iteritems():
                parts.append(dumps(k, sort_keys=sort_keys))
                parts.append(dumps(v, sort_keys=sort_keys))
        return b''.join(parts)


def dumps_bool(b):
    if b:
        return struct.pack('B', CBOR_TRUE)
    return struct.pack('B', CBOR_FALSE)


def dumps_tag(t, sort_keys=False):
    return _encode_type_num(CBOR_TAG, t.tag) + dumps(t.value, sort_keys=sort_keys)
    

if _IS_PY3:
    def _is_stringish(x):
        return isinstance(x, (str, bytes))
    def _is_intish(x):
        return isinstance(x, int)
else:
    def _is_stringish(x):
        return isinstance(x, (str, basestring, bytes, unicode))
    def _is_intish(x):
        return isinstance(x, (int, long))


def dumps(ob, sort_keys=False):
    if ob is None:
        return struct.pack('B', CBOR_NULL)
    if isinstance(ob, bool):
        return dumps_bool(ob)
    if _is_stringish(ob):
        return dumps_string(ob)
    if isinstance(ob, (list, tuple)):
        return dumps_array(ob, sort_keys=sort_keys)
    # TODO: accept other enumerables and emit a variable length array
    if isinstance(ob, dict):
        return dumps_dict(ob, sort_keys=sort_keys)
    if isinstance(ob, float):
        return dumps_float(ob)
    if _is_intish(ob):
        return dumps_int(ob)
    if isinstance(ob, Tag):
        return dumps_tag(ob, sort_keys=sort_keys)
    raise Exception("don't know how to cbor serialize object of type %s", type(ob))


# same basic signature as json.dump, but with no options (yet)
def dump(obj, fp, sort_keys=False):
    """
    obj: Python object to serialize
    fp: file-like object capable of .write(bytes)
    """
    # this is kinda lame, but probably not inefficient for non-huge objects
    # TODO: .write() to fp as we go as each inner object is serialized
    blob = dumps(obj, sort_keys=sort_keys)
    fp.write(blob)


class Tag(object):
    def __init__(self, tag=None, value=None):
        self.tag = tag
        self.value = value

    def __repr__(self):
        return "Tag({0!r}, {1!r})".format(self.tag, self.value)

    def __eq__(self, other):
        if not isinstance(other, Tag):
            return False
        return (self.tag == other.tag) and (self.value == other.value)


def loads(data):
    """
    Parse CBOR bytes and return Python objects.
    """
    if data is None:
        raise ValueError("got None for buffer to decode in loads")
    fp = StringIO(data)
    return _loads(fp)[0]


def load(fp):
    """
    Parse and return object from fp, a file-like object supporting .read(n)
    """
    return _loads(fp)[0]


_MAX_DEPTH = 100


def _tag_aux(fp, tb):
    bytes_read = 1
    tag = tb & CBOR_TYPE_MASK
    tag_aux = tb & CBOR_INFO_BITS
    if tag_aux <= 23:
        aux = tag_aux
    elif tag_aux == CBOR_UINT8_FOLLOWS:
        data = fp.read(1)
        aux = struct.unpack_from("!B", data, 0)[0]
        bytes_read += 1
    elif tag_aux == CBOR_UINT16_FOLLOWS:
        data = fp.read(2)
        aux = struct.unpack_from("!H", data, 0)[0]
        bytes_read += 2
    elif tag_aux == CBOR_UINT32_FOLLOWS:
        data = fp.read(4)
        aux = struct.unpack_from("!I", data, 0)[0]
        bytes_read += 4
    elif tag_aux == CBOR_UINT64_FOLLOWS:
        data = fp.read(8)
        aux = struct.unpack_from("!Q", data, 0)[0]
        bytes_read += 8
    else:
        assert tag_aux == CBOR_VAR_FOLLOWS, "bogus tag {0:02x}".format(tb)
        aux = None

    return tag, tag_aux, aux, bytes_read


def _read_byte(fp):
    tb = fp.read(1)
    if len(tb) == 0:
        # I guess not all file-like objects do this
        raise EOFError()
    return ord(tb)


def _loads_var_array(fp, limit, depth, returntags, bytes_read):
    ob = []
    tb = _read_byte(fp)
    while tb != CBOR_BREAK:
        (subob, sub_len) = _loads_tb(fp, tb, limit, depth, returntags)
        bytes_read += 1 + sub_len
        ob.append(subob)
        tb = _read_byte(fp)
    return (ob, bytes_read + 1)


def _loads_var_map(fp, limit, depth, returntags, bytes_read):
    ob = {}
    tb = _read_byte(fp)
    while tb != CBOR_BREAK:
        (subk, sub_len) = _loads_tb(fp, tb, limit, depth, returntags)
        bytes_read += 1 + sub_len
        (subv, sub_len) = _loads(fp, limit, depth, returntags)
        bytes_read += sub_len
        ob[subk] = subv
        tb = _read_byte(fp)
    return (ob, bytes_read + 1)


if _IS_PY3:
    def _loads_array(fp, limit, depth, returntags, aux, bytes_read):
        ob = []
        for i in range(aux):
            subob, subpos = _loads(fp)
            bytes_read += subpos
            ob.append(subob)
        return ob, bytes_read
    def _loads_map(fp, limit, depth, returntags, aux, bytes_read):
        ob = {}
        for i in range(aux):
            subk, subpos = _loads(fp)
            bytes_read += subpos
            subv, subpos = _loads(fp)
            bytes_read += subpos
            ob[subk] = subv
        return ob, bytes_read
else:
    def _loads_array(fp, limit, depth, returntags, aux, bytes_read):
        ob = []
        for i in xrange(aux):
            subob, subpos = _loads(fp)
            bytes_read += subpos
            ob.append(subob)
        return ob, bytes_read
    def _loads_map(fp, limit, depth, returntags, aux, bytes_read):
        ob = {}
        for i in xrange(aux):
            subk, subpos = _loads(fp)
            bytes_read += subpos
            subv, subpos = _loads(fp)
            bytes_read += subpos
            ob[subk] = subv
        return ob, bytes_read
        

def _loads(fp, limit=None, depth=0, returntags=False):
    "return (object, bytes read)"
    if depth > _MAX_DEPTH:
        raise Exception("hit CBOR loads recursion depth limit")

    tb = _read_byte(fp)

    return _loads_tb(fp, tb, limit, depth, returntags)

def _loads_tb(fp, tb, limit=None, depth=0, returntags=False):
    # Some special cases of CBOR_7 best handled by special struct.unpack logic here
    if tb == CBOR_FLOAT16:
        data = fp.read(2)
        hibyte, lowbyte = struct.unpack_from("BB", data, 0)
        exp = (hibyte >> 2) & 0x1F
        mant = ((hibyte & 0x03) << 8) | lowbyte
        if exp == 0:
            val = mant * (2.0 ** -24)
        elif exp == 31:
            if mant == 0:
                val = float('Inf')
            else:
                val = float('NaN')
        else:
            val = (mant + 1024.0) * (2 ** (exp - 25))
        if hibyte & 0x80:
            val = -1.0 * val
        return (val, 3)
    elif tb == CBOR_FLOAT32:
        data = fp.read(4)
        pf = struct.unpack_from("!f", data, 0)
        return (pf[0], 5)
    elif tb == CBOR_FLOAT64:
        data = fp.read(8)
        pf = struct.unpack_from("!d", data, 0)
        return (pf[0], 9)

    tag, tag_aux, aux, bytes_read = _tag_aux(fp, tb)

    if tag == CBOR_UINT:
        return (aux, bytes_read)
    elif tag == CBOR_NEGINT:
        return (-1 - aux, bytes_read)
    elif tag == CBOR_BYTES:
        ob, subpos = loads_bytes(fp, aux)
        return (ob, bytes_read + subpos)
    elif tag == CBOR_TEXT:
        raw, subpos = loads_bytes(fp, aux, btag=CBOR_TEXT)
        ob = raw.decode('utf8')
        return (ob, bytes_read + subpos)
    elif tag == CBOR_ARRAY:
        if aux is None:
            return _loads_var_array(fp, limit, depth, returntags, bytes_read)
        return _loads_array(fp, limit, depth, returntags, aux, bytes_read)
    elif tag == CBOR_MAP:
        if aux is None:
            return _loads_var_map(fp, limit, depth, returntags, bytes_read)
        return _loads_map(fp, limit, depth, returntags, aux, bytes_read)
    elif tag == CBOR_TAG:
        ob, subpos = _loads(fp)
        bytes_read += subpos
        if returntags:
            # Don't interpret the tag, return it and the tagged object.
            ob = Tag(aux, ob)
        else:
            # attempt to interpet the tag and the value into a Python object.
            ob = tagify(ob, aux)
        return ob, bytes_read
    elif tag == CBOR_7:
        if tb == CBOR_TRUE:
            return (True, bytes_read)
        if tb == CBOR_FALSE:
            return (False, bytes_read)
        if tb == CBOR_NULL:
            return (None, bytes_read)
        if tb == CBOR_UNDEFINED:
            return (None, bytes_read)
        raise ValueError("unknown cbor tag 7 byte: {:02x}".format(tb))


def loads_bytes(fp, aux, btag=CBOR_BYTES):
    # TODO: limit to some maximum number of chunks and some maximum total bytes
    if aux is not None:
        # simple case
        ob = fp.read(aux)
        return (ob, aux)
    # read chunks of bytes
    chunklist = []
    total_bytes_read = 0
    while True:
        tb = fp.read(1)[0]
        if not _IS_PY3:
            tb = ord(tb)
        if tb == CBOR_BREAK:
            total_bytes_read += 1
            break
        tag, tag_aux, aux, bytes_read = _tag_aux(fp, tb)
        assert tag == btag, 'variable length value contains unexpected component'
        ob = fp.read(aux)
        chunklist.append(ob)
        total_bytes_read += bytes_read + aux
    return (b''.join(chunklist), total_bytes_read)


if _IS_PY3:
    def _bytes_to_biguint(bs):
        out = 0
        for ch in bs:
            out = out << 8
            out = out | ch
        return out
else:
    def _bytes_to_biguint(bs):
        out = 0
        for ch in bs:
            out = out << 8
            out = out | ord(ch)
        return out


def tagify(ob, aux):
    # TODO: make this extensible?
    # cbor.register_tag_handler(tagnumber, tag_handler)
    # where tag_handler takes (tagnumber, tagged_object)
    if aux == CBOR_TAG_DATE_STRING:
        # TODO: parse RFC3339 date string
        pass
    if aux == CBOR_TAG_DATE_ARRAY:
        return datetime.datetime.utcfromtimestamp(ob)
    if aux == CBOR_TAG_BIGNUM:
        return _bytes_to_biguint(ob)
    if aux == CBOR_TAG_NEGBIGNUM:
        return -1 - _bytes_to_biguint(ob)
    if aux == CBOR_TAG_REGEX:
        # Is this actually a good idea? Should we just return the tag and the raw value to the user somehow?
        return re.compile(ob)
    return Tag(aux, ob)
