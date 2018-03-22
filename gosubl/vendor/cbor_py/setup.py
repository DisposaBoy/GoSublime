#! /usr/bin/env python
# Copyright 2014 Brian Olson
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
#     http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Thanks!
# to Mic Bowman for a bunch of work and impetus on dumps(,sort_keys=)

from distutils.command.build_ext import build_ext
from distutils.errors import (CCompilerError, DistutilsExecError,
    DistutilsPlatformError)
import sys

from setuptools import setup, Extension


build_errors = (CCompilerError, DistutilsExecError, DistutilsPlatformError)
if sys.platform == 'win32' and sys.version_info > (2, 6):
    # 2.6's distutils.msvc9compiler can raise an IOError when failing to
    # find the compiler
    build_errors += (IOError,)


class BuildError(Exception):
    """Raised if compiling extensions failed."""


class optional_build_ext(build_ext):
    """build_ext implementation with optional C speedups."""

    def run(self):
        try:
            build_ext.run(self)
        except DistutilsPlatformError:
            raise BuildError()

    def build_extension(self, ext):
        try:
            build_ext.build_extension(self, ext)
        except build_errors as be:
            raise BuildError(be)
        except ValueError as ve:
            # this can happen on Windows 64 bit, see Python issue 7511
            if "'path'" in str(sys.exc_info()[1]): # works with Python 2 and 3
                raise BuildError(ve)
            raise


VERSION = eval(open('cbor/VERSION.py','rb').read())


setup_options = dict(
    name='cbor',
    version=VERSION,
    description='RFC 7049 - Concise Binary Object Representation',
    long_description="""
An implementation of RFC 7049 - Concise Binary Object Representation (CBOR).

CBOR is comparable to JSON, has a superset of JSON's ability, but serializes to a binary format which is smaller and faster to generate and parse.

The two primary functions are cbor.loads() and cbor.dumps().

This library includes a C implementation which runs 3-5 times faster than the Python standard library's C-accelerated implementanion of JSON. This is also includes a 100% Python implementation.
""",
    author='Brian Olson',
    author_email='bolson@bolson.org',
    url='https://bitbucket.org/bodhisnarkva/cbor',
    packages=['cbor'],
    package_dir={'cbor':'cbor'},
    ext_modules=[
        Extension(
            'cbor._cbor',
            include_dirs=['c/'],
            sources=['c/cbormodule.c'],
            headers=['c/cbor.h'],
        )
    ],
    license='Apache',
    classifiers=[
	'Development Status :: 5 - Production/Stable',
        'Intended Audience :: Developers',
        'License :: OSI Approved :: Apache Software License',
        'Operating System :: OS Independent',
        'Programming Language :: Python :: 2.7',
        'Programming Language :: Python :: 3.4',
        'Programming Language :: Python :: 3.5',
        'Programming Language :: C',
        'Topic :: Software Development :: Libraries :: Python Modules',
    ],
    cmdclass={'build_ext': optional_build_ext},
)


def main():
    """ Perform setup with optional C speedups.

    Optional extension compilation stolen from markupsafe, which again stole
    it from simplejson. Creds to Bob Ippolito for the original code.
    """
    is_jython = 'java' in sys.platform
    is_pypy = hasattr(sys, 'pypy_translation_info')

    if is_jython or is_pypy:
        del setup_options['ext_modules']

    try:
        setup(**setup_options)
    except BuildError as be:
        sys.stderr.write('''
BUILD ERROR:
  %s
RETRYING WITHOUT C EXTENSIONS
''' % (be,))
        del setup_options['ext_modules']
        setup(**setup_options)


if __name__ == '__main__':
    main()
