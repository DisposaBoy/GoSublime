import os
import gscommon as gs
import margo
import sublime

env = {}

for k, v in gs.setting('env', {}).iteritems():
	os.environ[k] = os.path.expandvars(os.path.expanduser(v))

for i in os.environ.iteritems():
	env[i[0]] = i[1]

def margo_dep():
	motd = "hello world"
	resp, err = margo.hello(motd)
	m = resp.get('motd')

	if not err and m != motd:
		err = "Invalid response when calling MarGo. Expected `%s` got `%s`" % (motd, m)

	if err:
		gs.notice('GsInit', err)

sublime.set_timeout(margo_dep, 3)
