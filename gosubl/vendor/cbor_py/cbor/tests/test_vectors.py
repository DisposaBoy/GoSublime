#!/usr/bin/env python

"""
Test CBOR implementation against common "test vectors" set from
https://github.com/cbor/test-vectors/
"""

import base64
import json
import logging
import math
import os
import sys
import unittest


_IS_PY3 = sys.version_info[0] >= 3


logger = logging.getLogger(__name__)


#from cbor.cbor import dumps as pydumps
from cbor.cbor import loads as pyloads
try:
    #from cbor._cbor import dumps as cdumps
    from cbor._cbor import loads as cloads
except ImportError:
    # still test what we can without C fast mode
    logger.warn('testing without C accelerated CBOR', exc_info=True)
    #cdumps, cloads = None, None
    cloads = None
from cbor import Tag


# Accomodate several test vectors that have diagnostic descriptors but not JSON
_DIAGNOSTIC_TESTS = {
    'Infinity': lambda x: x == float('Inf'),
    '-Infinity': lambda x: x == float('-Inf'),
    'NaN': math.isnan,
    'undefined': lambda x: x is None,

    # TODO: parse into datetime.datetime()
    '0("2013-03-21T20:04:00Z")': lambda x: isinstance(x, Tag) and (x.tag == 0) and (x.value == '2013-03-21T20:04:00Z'),

    "h''": lambda x: x == b'',
    "(_ h'0102', h'030405')": lambda x: x == b'\x01\x02\x03\x04\x05',
    '{1: 2, 3: 4}': lambda x: x == {1: 2, 3: 4},
    "h'01020304'": lambda x: x == b'\x01\x02\x03\x04',
}


# We expect these to raise exception because they encode reserved/unused codes in the spec.
# ['hex'] values of tests we expect to raise
_EXPECT_EXCEPTION = set(['f0', 'f818', 'f8ff'])


def _check(row, decoded):
    cbdata = base64.b64decode(row['cbor'])
    if cloads is not None:
        cb = cloads(cbdata)
        if cb != decoded:
            anyerr = True
            sys.stderr.write('expected {0!r} got {1!r} c failed to decode cbor {2}\n'.format(decoded, cb, base64.b16encode(cbdata)))

    cb = pyloads(cbdata)
    if cb != decoded:
        anyerr = True
        sys.stderr.write('expected {0!r} got {1!r} py failed to decode cbor {2}\n'.format(decoded, cb, base64.b16encode(cbdata)))


def _check_foo(row, checkf):
    cbdata = base64.b64decode(row['cbor'])
    if cloads is not None:
        cb = cloads(cbdata)
        if not checkf(cb):
            anyerr = True
            sys.stderr.write('expected {0!r} got {1!r} c failed to decode cbor {2}\n'.format(decoded, cb, base64.b16encode(cbdata)))

    cb = pyloads(cbdata)
    if not checkf(cb):
        anyerr = True
        sys.stderr.write('expected {0!r} got {1!r} py failed to decode cbor {2}\n'.format(decoded, cb, base64.b16encode(cbdata)))


class TestVectors(unittest.TestCase):
        def test_vectors(self):
            here = os.path.dirname(__file__)
            jf = os.path.abspath(os.path.join(here, '../../../test-vectors/appendix_a.json'))
            if not os.path.exists(jf):
                logging.warning('cannot find test-vectors/appendix_a.json, tried: %r', jf)
                return

            if _IS_PY3:
                testfile = open(jf, 'r')
                tv = json.load(testfile)
            else:
                testfile = open(jf, 'rb')
                tv = json.load(testfile)
            anyerr = False
            for row in tv:
                rhex = row.get('hex')
                if 'decoded' in row:
                    decoded = row['decoded']
                    _check(row, decoded)
                    continue
                elif 'diagnostic' in row:
                    diag = row['diagnostic']
                    checkf = _DIAGNOSTIC_TESTS.get(diag)
                    if checkf is not None:
                        _check_foo(row, checkf)
                        continue

                # variously verbose log of what we're not testing:
                cbdata = base64.b64decode(row['cbor'])
                try:
                    pd = pyloads(cbdata)
                except:
                    if rhex and (rhex in _EXPECT_EXCEPTION):
                        pass
                    else:
                        logging.error('failed to py load hex=%s diag=%r', rhex, row.get('diagnostic'), exc_info=True)
                    pd = ''
                cd = None
                if cloads is not None:
                    try:
                        cd = cloads(cbdata)
                    except:
                        if rhex and (rhex in _EXPECT_EXCEPTION):
                            pass
                        else:
                            logging.error('failed to c load hex=%s diag=%r', rhex, row.get('diagnostic'), exc_info=True)
                        cd = ''
                logging.warning('skipping hex=%s diag=%r py=%s c=%s', rhex, row.get('diagnostic'), pd, cd)
            testfile.close()

            assert not anyerr


if __name__ == '__main__':
    logging.basicConfig(level=logging.DEBUG)
    unittest.main()
