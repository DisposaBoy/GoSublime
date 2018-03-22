#!python
from __future__ import absolute_import
from __future__ import division  # / => float
import gc
import logging
import os
import resource
import sys
import tempfile
import unittest

from cbor.tests.test_cbor import _randob


logger = logging.getLogger(__name__)


try:
    from cbor._cbor import dumps as cdumps
    from cbor._cbor import loads as cloads
    from cbor._cbor import dump as cdump
    from cbor._cbor import load as cload
except ImportError:
    # still test what we can without C fast mode
    logger.warn('testing without C accelerated CBOR', exc_info=True)
    cdumps, cloads, cdump, cload = None, None, None, None



_TEST_COUNT = 100000
_TEST_OUTER = 5


_IS_PY3 = sys.version_info[0] >= 3


if _IS_PY3:
    _range = range
    from io import BytesIO as StringIO
else:
    _range = xrange
    from cStringIO import StringIO


class TestUsage(unittest.TestCase):
    def test_dumps_usage(self):
        '''
        repeatedly serialize, check that usage doesn't go up
        '''
        if cdumps is None:
            logger.warn('no C dumps(), skipping test_dumps_usage')
            return
        start_usage = resource.getrusage(resource.RUSAGE_SELF)
        usage_history = [start_usage]
        for o in _range(_TEST_OUTER):
            for i in _range(_TEST_COUNT):
                ob = _randob()
                blob = cdumps(ob)
                # and silently drop the result. I hope the garbage collector works!
            t_usage = resource.getrusage(resource.RUSAGE_SELF)
            usage_history.append(t_usage)
        end_usage = usage_history[-1]
        dmaxrss = end_usage.ru_maxrss - start_usage.ru_maxrss
        didrss = end_usage.ru_idrss - start_usage.ru_idrss
        dmaxrsspct = ((end_usage.ru_maxrss != 0) and (dmaxrss / end_usage.ru_maxrss)) or 0
        didrsspct = ((end_usage.ru_idrss != 0) and (didrss / end_usage.ru_idrss)) or 0

        sys.stderr.write('maxrss: {} - {}, d={} ({:.2f}%)\n'.format(start_usage.ru_maxrss, end_usage.ru_maxrss, dmaxrss, dmaxrsspct * 100.0))
        sys.stderr.write('idrss: {} - {}, d={} ({:.2f}%)\n'.format(start_usage.ru_idrss, end_usage.ru_idrss, didrss, didrsspct * 100.0))

        assert (dmaxrsspct) < 0.05, [x.ru_maxrss for x in usage_history]
        assert (didrsspct) < 0.05, [x.ru_idrss for x in usage_history]

    def test_loads_usage(self):
        '''
        repeatedly serialize, check that usage doesn't go up
        '''
        if (cdumps is None) or (cloads is None):
            logger.warn('no C fast CBOR, skipping test_loads_usage')
            return
        ## Just a string passes!
        #ob = 'sntaoheusnatoheusnaotehuasnoetuhaosentuhaoesnth'
        ## Just an array passes!
        #ob = [1,2,3,4,5,6,7,8,9,12,12,13]
        ## Just a dict passes!
        #ob = {'a':'b', 'c':'d', 'e':'f', 'g':'h'}
        # dict of dict is doom!
        #ob = {'a':{'b':'c', 'd':'e', 'f':'g'}, 'x':'p'}
        ob = {'aoeu':[1,2,3,4],'foo':'bar','pants':{'foo':0xb44, 'pi':3.14}, 'flubber': [{'x':'y', 'z':[None, 2, []]}, 2, 'hello']}
        blob = cdumps(ob)
        start_usage = resource.getrusage(resource.RUSAGE_SELF)
        usage_history = [start_usage]
        for o in _range(_TEST_OUTER):
            for i in _range(_TEST_COUNT):
                dob = cloads(blob)
                # and silently drop the result. I hope the garbage collector works!
            t_usage = resource.getrusage(resource.RUSAGE_SELF)
            usage_history.append(t_usage)
        end_usage = usage_history[-1]
        dmaxrss = end_usage.ru_maxrss - start_usage.ru_maxrss
        didrss = end_usage.ru_idrss - start_usage.ru_idrss
        dmaxrsspct = ((end_usage.ru_maxrss != 0) and (dmaxrss / end_usage.ru_maxrss)) or 0
        didrsspct = ((end_usage.ru_idrss != 0) and (didrss / end_usage.ru_idrss)) or 0

        sys.stderr.write('maxrss: {} - {}, d={} ({:.2f}%)\n'.format(start_usage.ru_maxrss, end_usage.ru_maxrss, dmaxrss, dmaxrsspct * 100.0))
        sys.stderr.write('idrss: {} - {}, d={} ({:.2f}%)\n'.format(start_usage.ru_idrss, end_usage.ru_idrss, didrss, didrsspct * 100.0))

        assert (dmaxrsspct) < 0.05, [x.ru_maxrss for x in usage_history]
        assert (didrsspct) < 0.05, [x.ru_idrss for x in usage_history]

    def test_tempfile(self):
        '''repeatedly seralize to temp file, then repeatedly deserialize from
        it, checking usage all along the way.
        '''
        if cdump is None:
            logger.warn('no C dump(), skipping test_tempfile')
            return
        with tempfile.NamedTemporaryFile() as ntf:
            # first, write a bunch to temp file
            with open(ntf.name, 'wb') as fout:
                sys.stderr.write('write {!r} {}\n'.format(ntf.name, fout))
                start_usage = resource.getrusage(resource.RUSAGE_SELF)
                usage_history = [start_usage]
                for o in _range(_TEST_OUTER):
                    for i in _range(_TEST_COUNT):
                        ob = _randob()
                        cdump(ob, fout)
                    t_usage = resource.getrusage(resource.RUSAGE_SELF)
                    usage_history.append(t_usage)
                end_usage = usage_history[-1]
                dmaxrss = end_usage.ru_maxrss - start_usage.ru_maxrss
                didrss = end_usage.ru_idrss - start_usage.ru_idrss
                dmaxrsspct = ((end_usage.ru_maxrss != 0) and (dmaxrss / end_usage.ru_maxrss)) or 0
                didrsspct = ((end_usage.ru_idrss != 0) and (didrss / end_usage.ru_idrss)) or 0

                sys.stderr.write('maxrss: {} - {}, d={} ({:.2f}%)\n'.format(start_usage.ru_maxrss, end_usage.ru_maxrss, dmaxrss, dmaxrsspct * 100.0))
                sys.stderr.write('idrss: {} - {}, d={} ({:.2f}%)\n'.format(start_usage.ru_idrss, end_usage.ru_idrss, didrss, didrsspct * 100.0))

                assert (dmaxrsspct) < 0.05, [x.ru_maxrss for x in usage_history]
                assert (didrsspct) < 0.05, [x.ru_idrss for x in usage_history]

            sys.stderr.write('{!r} is {} bytes\n'.format(ntf.name, os.path.getsize(ntf.name)))

            # now, read a bunch back from temp file.
            with open(ntf.name, 'rb') as fin:
                sys.stderr.write('read {!r} {}\n'.format(ntf.name, fin))
                start_usage = resource.getrusage(resource.RUSAGE_SELF)
                usage_history = [start_usage]
                for o in _range(_TEST_OUTER):
                    for i in _range(_TEST_COUNT):
                        dob = cload(fin)
                        # and silently drop the result. I hope the garbage collector works!
                    gc.collect()
                    t_usage = resource.getrusage(resource.RUSAGE_SELF)
                    usage_history.append(t_usage)
                end_usage = usage_history[-1]
                dmaxrss = end_usage.ru_maxrss - start_usage.ru_maxrss
                didrss = end_usage.ru_idrss - start_usage.ru_idrss
                dmaxrsspct = ((end_usage.ru_maxrss != 0) and (dmaxrss / end_usage.ru_maxrss)) or 0
                didrsspct = ((end_usage.ru_idrss != 0) and (didrss / end_usage.ru_idrss)) or 0

                sys.stderr.write('maxrss: {} - {}, d={} ({:.2f}%)\n'.format(start_usage.ru_maxrss, end_usage.ru_maxrss, dmaxrss, dmaxrsspct * 100.0))
                sys.stderr.write('idrss: {} - {}, d={} ({:.2f}%)\n'.format(start_usage.ru_idrss, end_usage.ru_idrss, didrss, didrsspct * 100.0))

                assert (dmaxrsspct) < 0.05, [x.ru_maxrss for x in usage_history]
                assert (didrsspct) < 0.05, [x.ru_idrss for x in usage_history]

    def test_stringio_usage(self):
        '''serialize data to StringIO, read it back'''
        if cdump is None:
            logger.warn('no C dump(), skipping test_tempfile')
            return

        # warmup the rusage, allocate everything!
        fout = StringIO()
        sys.stderr.write('write 1 to StringIO\n')
        oblist = []
        for o in _range(_TEST_OUTER):
            for i in _range(_TEST_COUNT):
                ob = _randob()
                oblist.append(ob)
                cdump(ob, fout)

        # position at start to overwrite, but leave allocated
        fout.seek(0)

        sys.stderr.write('write 2 to StringIO\n')
        start_usage = resource.getrusage(resource.RUSAGE_SELF)
        usage_history = [start_usage]
        pos = 0
        for o in _range(_TEST_OUTER):
            for i in _range(_TEST_COUNT):
                ob = oblist[pos]
                pos += 1
                cdump(ob, fout)
            gc.collect()
            t_usage = resource.getrusage(resource.RUSAGE_SELF)
            usage_history.append(t_usage)
        end_usage = usage_history[-1]
        dmaxrss = end_usage.ru_maxrss - start_usage.ru_maxrss
        didrss = end_usage.ru_idrss - start_usage.ru_idrss
        dmaxrsspct = ((end_usage.ru_maxrss != 0) and (dmaxrss / end_usage.ru_maxrss)) or 0
        didrsspct = ((end_usage.ru_idrss != 0) and (didrss / end_usage.ru_idrss)) or 0

        sys.stderr.write('maxrss: {} - {}, d={} ({:.2f}%)\n'.format(start_usage.ru_maxrss, end_usage.ru_maxrss, dmaxrss, dmaxrsspct * 100.0))
        sys.stderr.write('idrss: {} - {}, d={} ({:.2f}%)\n'.format(start_usage.ru_idrss, end_usage.ru_idrss, didrss, didrsspct * 100.0))

        assert (dmaxrsspct) < 0.05, [x.ru_maxrss for x in usage_history]
        assert (didrsspct) < 0.05, [x.ru_idrss for x in usage_history]

        sys.stderr.write('StringIO is {} bytes\n'.format(fout.tell()))
        fout.seek(0)

        fin = fout
        sys.stderr.write('read StringIO\n')
        start_usage = resource.getrusage(resource.RUSAGE_SELF)
        usage_history = [start_usage]
        for o in _range(_TEST_OUTER):
            for i in _range(_TEST_COUNT):
                dob = cload(fin)
                # and silently drop the result. I hope the garbage collector works!
            gc.collect()
            t_usage = resource.getrusage(resource.RUSAGE_SELF)
            usage_history.append(t_usage)
        end_usage = usage_history[-1]
        dmaxrss = end_usage.ru_maxrss - start_usage.ru_maxrss
        didrss = end_usage.ru_idrss - start_usage.ru_idrss
        dmaxrsspct = ((end_usage.ru_maxrss != 0) and (dmaxrss / end_usage.ru_maxrss)) or 0
        didrsspct = ((end_usage.ru_idrss != 0) and (didrss / end_usage.ru_idrss)) or 0

        sys.stderr.write('maxrss: {} - {}, d={} ({:.2f}%)\n'.format(start_usage.ru_maxrss, end_usage.ru_maxrss, dmaxrss, dmaxrsspct * 100.0))
        sys.stderr.write('idrss: {} - {}, d={} ({:.2f}%)\n'.format(start_usage.ru_idrss, end_usage.ru_idrss, didrss, didrsspct * 100.0))

        assert (dmaxrsspct) < 0.05, [x.ru_maxrss for x in usage_history]
        assert (didrsspct) < 0.05, [x.ru_idrss for x in usage_history]



if __name__ == '__main__':
    logging.basicConfig(level=logging.DEBUG)
    unittest.main()
