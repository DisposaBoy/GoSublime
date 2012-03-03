import os
import gscommon as gs
import margo
import sublime

env = {}

for k, v in gs.setting('env', {}).iteritems():
	os.environ[k] = os.path.expandvars(os.path.expanduser(v))

for i in os.environ.iteritems():
	env[i[0]] = i[1]

sublime.set_timeout(lambda: margo.hello("hello world"), 0)
