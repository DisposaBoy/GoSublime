import os
import gscommon as gs, margo, gsq
import sublime

DOMAIN = 'GoSublime'

def margo_dep(try_install):
	motd = "hello world"
	resp, err = margo.hello(motd)
	m = resp.get('motd')

	att_msg = 'Attempting to install MarGo'
	if (not 'motd' in resp or not 'actions' in resp) and try_install:
		gs.notice(DOMAIN, att_msg)
		def cb():
			out, err = gs.runcmd(['go', 'get', '-u', 'github.com/DisposaBoy/MarGo'])
			err = '%s\n%s' % (out, err)
			err = err.strip()
			if err:
				gs.notice(DOMAIN, err)
			else:
				gs.notice(DOMAIN, '%s: done...' % att_msg)
			sublime.set_timeout(lambda: margo_dep(False), 0)
		sublime.set_timeout(cb, 0)
		return

	if not err and m != motd:
		err = "Invalid response when calling MarGo. Expected `%s` got `%s`" % (motd, m)

	if err:
		gs.notice(DOMAIN, err)

def sync_env():
	for k, v in gs.setting('env', {}).iteritems():
		os.environ[k] = os.path.expandvars(os.path.expanduser(v))

gs.settings_obj().add_on_change('env', sync_env)

gsq.dispatch(sync_env)
gsq.dispatch(lambda: margo_dep(False))
