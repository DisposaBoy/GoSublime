import re
import sublime

ANN = 'a13.03.28-2'
VERSION = 'r13.03.28-2'
VERSION_PAT = re.compile(r'r\d{2}.\d{2}.\d{2}-\d+', re.IGNORECASE)
PLATFORM = '%s-%s' % (sublime.platform(), sublime.arch())
MARGO_EXE_PREFIX = 'gosublime.margo'
MARGO_EXE_SUFFIX = 'exe'
_sfx = '%s.%s' % (PLATFORM, MARGO_EXE_SUFFIX)
MARGO_EXE = '%s.%s.%s' % (MARGO_EXE_PREFIX, VERSION, _sfx)
MARGO_EXE_PAT = re.compile(r'^%s.%s.%s$' % (MARGO_EXE_PREFIX, VERSION_PAT.pattern, _sfx), re.IGNORECASE)
