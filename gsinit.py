import os
import gscommon as gs
import margo

env = {}

for k, v in gs.setting('env', {}).iteritems():
	os.environ[k] = os.path.expandvars(os.path.expanduser(v))

for i in os.environ.iteritems():
	env[i[0]] = i[1]
