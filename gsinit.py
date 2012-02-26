import os
import gscommon as gs

for k, v in gs.setting('env', {}).iteritems():
	os.environ[k] = os.path.expandvars(os.path.expanduser(v))
