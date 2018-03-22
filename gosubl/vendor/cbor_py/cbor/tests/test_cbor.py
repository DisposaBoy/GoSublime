#!python
# -*- coding: utf-8 -*-

import base64
import datetime
import json
import logging
import random
import sys
import time
import unittest
import zlib


logger = logging.getLogger(__name__)


from cbor.cbor import dumps as pydumps
from cbor.cbor import loads as pyloads
from cbor.cbor import dump as pydump
from cbor.cbor import load as pyload
from cbor.cbor import Tag
try:
    from cbor._cbor import dumps as cdumps
    from cbor._cbor import loads as cloads
    from cbor._cbor import dump as cdump
    from cbor._cbor import load as cload
except ImportError:
    # still test what we can without C fast mode
    logger.warn('testing without C accelerated CBOR', exc_info=True)
    cdumps, cloads, cdump, cload = None, None, None, None


_IS_PY3 = sys.version_info[0] >= 3


if _IS_PY3:
    _range = range
    from io import BytesIO as StringIO
else:
    _range = xrange
    from cStringIO import StringIO


class TestRoot(object):
    @classmethod
    def loads(cls, *args):
        return cls._ld[0](*args)
    @classmethod
    def dumps(cls, *args, **kwargs):
        return cls._ld[1](*args, **kwargs)
    @classmethod
    def speediterations(cls):
        return cls._ld[2]
    @classmethod
    def load(cls, *args):
        return cls._ld[3](*args)
    @classmethod
    def dump(cls, *args, **kwargs):
        return cls._ld[4](*args, **kwargs)
    @classmethod
    def testable(cls):
        ok = (cls._ld[0] is not None) and (cls._ld[1] is not None) and (cls._ld[3] is not None) and (cls._ld[4] is not None)
        if not ok:
            logger.warn('non-testable case %s skipped', cls.__name__)
        return ok

# Can't set class level function pointers, because then they expect a
# (cls) first argument. So, toss them in a list to hide them.
class TestPyPy(TestRoot):
    _ld = [pyloads, pydumps, 1000, pyload, pydump]

class TestPyC(TestRoot):
    _ld = [pyloads, cdumps, 2000, pyload, cdump]

class TestCPy(TestRoot):
    _ld = [cloads, pydumps, 2000, cload, pydump]

class TestCC(TestRoot):
    _ld = [cloads, cdumps, 150000, cload, cdump]


if _IS_PY3:
    def _join_jsers(jsers):
        return (''.join(jsers)).encode('utf8')
    def hexstr(bs):
        return ' '.join(map(lambda x: '{0:02x}'.format(x), bs))
else:
    def _join_jsers(jsers):
        return b''.join(jsers)
    def hexstr(bs):
        return ' '.join(map(lambda x: '{0:02x}'.format(ord(x)), bs))


class XTestCBOR(object):
    def _oso(self, ob):
        ser = self.dumps(ob)
        try:
            o2 = self.loads(ser)
            assert ob == o2, '%r != %r from %s' % (ob, o2, base64.b16encode(ser))
        except Exception as e:
            sys.stderr.write('failure on buf len={0} {1!r} ob={2!r} {3!r}; {4}\n'.format(len(ser), hexstr(ser), ob, ser, e))
            raise

    def _osos(self, ob):
        obs = self.dumps(ob)
        o2 = self.loads(obs)
        o2s = self.dumps(o2)
        assert obs == o2s

    def _oso_bytearray(self, ob):
        ser = self.dumps(ob)
        try:
            o2 = self.loads(bytearray(ser))
            assert ob == o2, '%r != %r from %s' % (ob, o2, base64.b16encode(ser))
        except Exception as e:
            sys.stderr.write('failure on buf len={0} {1!r} ob={2!r} {3!r}; {4}\n'.format(len(ser), hexstr(ser), ob, ser, e))
            raise

    test_objects = [
        1,
        0,
        True,
        False,
        None,
        -1,
        -1.5,
        1.5,
        1000,
        -1000,
        1000000000,
        2376030000,
        -1000000000,
        1000000000000000,
        -1000000000000000,
        [],
        [1,2,3],
        {},
        b'aoeu1234\x00\xff',
        u'åöéûのかめ亀',
        b'',
        u'',
        Tag(1234, 'aoeu'),
    ]

    def test_basic(self):
        if not self.testable(): return
        for ob in self.test_objects:
            self._oso(ob)

    def test_basic_bytearray(self):
        if not self.testable(): return
        xoso = self._oso
        self._oso = self._oso_bytearray
        try:
            self.test_basic()
        finally:
            self._oso = xoso

    def test_random_ints(self):
        if not self.testable(): return
        icount = self.speediterations()
        for i in _range(icount):
            v = random.randint(-4294967295, 0xffffffff)
            self._oso(v)
        oldv = []
        for i in _range(int(icount / 10)):
            v = random.randint(-1000000000000000000000, 1000000000000000000000)
            self._oso(v)
            oldv.append(v)

    def test_randobs(self):
        if not self.testable(): return
        icount = self.speediterations()
        for i in _range(icount):
            ob = _randob()
            self._oso(ob)

    def test_tuple(self):
        if not self.testable(): return
        l = [1,2,3]
        t = tuple(l)
        ser = self.dumps(t)
        o2 = self.loads(ser)
        assert l == o2

    def test_speed_vs_json(self):
        if not self.testable(): return
        # It should be noted that the python standard library has a C implementation of key parts of json encoding and decoding
        icount = self.speediterations()
        obs = [_randob_notag() for x in _range(icount)]
        st = time.time()
        bsers = [self.dumps(o) for o in obs]
        nt = time.time()
        cbor_ser_time = nt - st
        jsers = [json.dumps(o) for o in obs]
        jt = time.time()
        json_ser_time = jt - nt
        cbor_byte_count = sum(map(len, bsers))
        json_byte_count = sum(map(len, jsers))
        sys.stderr.write(
            'serialized {nobs} objects into {cb} cbor bytes in {ct:.2f} seconds ({cops:.2f}/s, {cbps:.1f}B/s) and {jb} json bytes in {jt:.2f} seconds ({jops:.2f}/s, {jbps:.1f}B/s)\n'.format(
            nobs=len(obs),
            cb=cbor_byte_count,
            ct=cbor_ser_time,
            cops=len(obs) / cbor_ser_time,
            cbps=cbor_byte_count / cbor_ser_time,
            jb=json_byte_count,
            jt=json_ser_time,
            jops=len(obs) / json_ser_time,
            jbps=json_byte_count / json_ser_time))
        bsersz = zlib.compress(b''.join(bsers))
        jsersz = zlib.compress(_join_jsers(jsers))
        sys.stderr.write('compress to {0} bytes cbor.gz and {1} bytes json.gz\n'.format(
            len(bsersz), len(jsersz)))

        st = time.time()
        bo2 = [self.loads(b) for b in bsers]
        bt = time.time()
        cbor_load_time = bt - st
        jo2 = [json.loads(b) for b in jsers]
        jt = time.time()
        json_load_time = jt - bt
        sys.stderr.write('load {nobs} objects from cbor in {ct:.2f} secs ({cops:.2f}/sec, {cbps:.1f}B/s) and json in {jt:.2f} ({jops:.2f}/sec, {jbps:.1f}B/s)\n'.format(
            nobs=len(obs),
            ct=cbor_load_time,
            cops=len(obs) / cbor_load_time,
            cbps=cbor_byte_count / cbor_load_time,
            jt=json_load_time,
            jops=len(obs) / json_load_time,
            jbps=json_byte_count / json_load_time
        ))

    def test_loads_none(self):
        if not self.testable(): return
        try:
            ob = self.loads(None)
            assert False, "expected ValueError when passing in None"
        except ValueError:
            pass

    def test_concat(self):
        "Test that we can concatenate output and retrieve the objects back out."
        if not self.testable(): return
        self._oso(self.test_objects)
        fob = StringIO()

        for ob in self.test_objects:
            self.dump(ob, fob)
        fob.seek(0)
        obs2 = []
        try:
            while True:
                obs2.append(self.load(fob))
        except EOFError:
            pass
        assert obs2 == self.test_objects

    # TODO: find more bad strings with which to fuzz CBOR
    def test_badread(self):
        if not self.testable(): return
        try:
            ob = self.loads(b'\xff')
            assert False, 'badread should have failed'
        except ValueError as ve:
            #logger.info('error', exc_info=True)
            pass
        except Exception as ex:
            logger.info('unexpected error!', exc_info=True)
            assert False, 'unexpected error' + str(ex)

    def test_datetime(self):
        if not self.testable(): return
        # right now we're just testing that it's possible to dumps()
        # Tag(0,...) because there was a bug around that.
        xb = self.dumps(Tag(0, datetime.datetime(1984,1,24,23,22,21).isoformat()))

    def test_sortkeys(self):
        if not self.testable(): return
        obytes = []
        xbytes = []
        for n in _range(2, 27):
            ob = {u'{:02x}'.format(x):x for x in _range(n)}
            obytes.append(self.dumps(ob, sort_keys=True))
            xbytes.append(self.dumps(ob, sort_keys=False))
        allOGood = True
        someXMiss = False
        for i, g in enumerate(_GOLDEN_SORTED_KEYS_BYTES):
            if g != obytes[i]:
                logger.error('bad sorted result, wanted %r got %r', g, obytes[i])
                allOGood = False
            if g != xbytes[i]:
                someXMiss = True

        assert allOGood
        assert someXMiss


_GOLDEN_SORTED_KEYS_BYTES = [
b'\xa2b00\x00b01\x01',
b'\xa3b00\x00b01\x01b02\x02',
b'\xa4b00\x00b01\x01b02\x02b03\x03',
b'\xa5b00\x00b01\x01b02\x02b03\x03b04\x04',
b'\xa6b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05',
b'\xa7b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06',
b'\xa8b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07',
b'\xa9b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08',
b'\xaab00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\t',
b'\xabb00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\n',
b'\xacb00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0b',
b'\xadb00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0c',
b'\xaeb00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\r',
b'\xafb00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\rb0e\x0e',
b'\xb0b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\rb0e\x0eb0f\x0f',
b'\xb1b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\rb0e\x0eb0f\x0fb10\x10',
b'\xb2b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\rb0e\x0eb0f\x0fb10\x10b11\x11',
b'\xb3b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\rb0e\x0eb0f\x0fb10\x10b11\x11b12\x12',
b'\xb4b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\rb0e\x0eb0f\x0fb10\x10b11\x11b12\x12b13\x13',
b'\xb5b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\rb0e\x0eb0f\x0fb10\x10b11\x11b12\x12b13\x13b14\x14',
b'\xb6b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\rb0e\x0eb0f\x0fb10\x10b11\x11b12\x12b13\x13b14\x14b15\x15',
b'\xb7b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\rb0e\x0eb0f\x0fb10\x10b11\x11b12\x12b13\x13b14\x14b15\x15b16\x16',
b'\xb8\x18b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\rb0e\x0eb0f\x0fb10\x10b11\x11b12\x12b13\x13b14\x14b15\x15b16\x16b17\x17',
b'\xb8\x19b00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\rb0e\x0eb0f\x0fb10\x10b11\x11b12\x12b13\x13b14\x14b15\x15b16\x16b17\x17b18\x18\x18',
b'\xb8\x1ab00\x00b01\x01b02\x02b03\x03b04\x04b05\x05b06\x06b07\x07b08\x08b09\tb0a\nb0b\x0bb0c\x0cb0d\rb0e\x0eb0f\x0fb10\x10b11\x11b12\x12b13\x13b14\x14b15\x15b16\x16b17\x17b18\x18\x18b19\x18\x19',
]

def gen_sorted_bytes():
    for n in _range(2, 27):
        sys.stdout.write(repr(cbor.dumps({u'{:02x}'.format(x):x for x in _range(n)}, sort_keys=True)) + ',\n')

def gen_unsorted_bytes():
    for n in _range(2, 27):
        sys.stdout.write(repr(cbor.dumps({u'{:02x}'.format(x):x for x in _range(n)}, sort_keys=False)) + ',\n')


class TestCBORPyPy(unittest.TestCase, XTestCBOR, TestPyPy):
    pass

class TestCBORCPy(unittest.TestCase, XTestCBOR, TestCPy):
    pass

class TestCBORPyC(unittest.TestCase, XTestCBOR, TestPyC):
    pass

class TestCBORCC(unittest.TestCase, XTestCBOR, TestCC):
    pass


def _randob():
    return _randob_x(_randob_probabilities, _randob_probsum, _randob)

def _randob_notag():
    return _randob_x(_randob_probabilities_notag, _randob_notag_probsum, _randob_notag)

def _randArray(randob=_randob):
    return [randob() for x in _range(random.randint(0,5))]

_chars = [chr(x) for x in _range(ord(' '), ord('~'))]

def _randStringOrBytes(randob=_randob):
    tstr = ''.join([random.choice(_chars) for x in _range(random.randint(1,10))])
    if random.randint(0,1) == 1:
        if _IS_PY3:
            # default str is unicode
            # sometimes squash to bytes
            return tstr.encode('utf8')
        else:
            # default str is bytes
            # sometimes promote to unicode string
            return tstr.decode('utf8')
    return tstr

def _randString(randob=_randob):
    return ''.join([random.choice(_chars) for x in _range(random.randint(1,10))])

def _randDict(randob=_randob):
    ob = {}
    for x in _range(random.randint(0,5)):
        ob[_randString()] = randob()
    return ob


def _randTag(randob=_randob):
    t = Tag()
    # Tags 0..36 are know standard things we might implement special
    # decoding for. This number will grow over time, and this test
    # need to be adjusted to only assign unclaimed tags for Tag<->Tag
    # encode-decode testing.
    t.tag = random.randint(37, 1000000)
    t.value = randob()
    return t

def _randInt(randob=_randob):
    return random.randint(-4294967295, 4294967295)

def _randBignum(randob=_randob):
    return random.randint(-1000000000000000000000, 1000000000000000000000)

def _randFloat(randob=_randob):
    return random.random()

_CONSTANTS = (True, False, None)
def _randConst(randob=_randob):
    return random.choice(_CONSTANTS)

_randob_probabilities = [
    (0.1, _randDict),
    (0.1, _randTag),
    (0.2, _randArray),
    (0.3, _randStringOrBytes),
    (0.3, _randInt),
    (0.2, _randBignum),
    (0.2, _randFloat),
    (0.2, _randConst),
]

_randob_probsum = sum([x[0] for x in _randob_probabilities])

_randob_probabilities_notag = [
    (0.1, _randDict),
    (0.2, _randArray),
    (0.3, _randString),
    (0.3, _randInt),
    (0.2, _randBignum),
    (0.2, _randFloat),
    (0.2, _randConst),
]

_randob_notag_probsum = sum([x[0] for x in _randob_probabilities_notag])

def _randob_x(probs=_randob_probabilities, probsum=_randob_probsum, randob=_randob):
    pos = random.uniform(0, probsum)
    for p, op in probs:
        if pos < p:
            return op(randob)
        pos -= p
    return None


if __name__ == '__main__':
    logging.basicConfig(level=logging.INFO)
    unittest.main()
