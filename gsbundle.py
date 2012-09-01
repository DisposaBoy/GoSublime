import gsshell, gscommon as gs
import os
import sublime

DOMAIN = 'GsBundle'
INSTALL_CMD = ['go', 'install', '-v','margo', 'gocode']

def print_install_log(c, s):
	e = gs.env()
	dur = c.ended - c.started
	gs.println(
		'GoSublime: %s done %0.3fs' % (DOMAIN, dur),
		'| Bundle GOPATH: %s' % BUNDLE_GOPATH,
		'|  Bundle GOBIN: %s' % BUNDLE_GOBIN,
		'|   User GOROOT: %s' % e.get('GOROOT', '(NOT SET)'),
		'|   User GOPATH: %s' % e.get('GOPATH', '(NOT SET)'),
		'|    User GOBIN: %s (should usually be `NOT SET\')' % e.get('GOBIN', '(NOT SET)'),
		'| Output:\n%s\n' % s
	)

	unset = []
	if not e.get('GOROOT'):
		unset.append('GOROOT')
	if not e.get('GOPATH'):
		unset.append('GOPATH')
	if unset:
		tpl = 'check the console for error messages: the following environment variables are not set: %s'
		gs.notice(DOMAIN, tpl % ', '.join(unset))

def on_env_done(c):
	l = c.consume_outq()
	e = {}
	for i in l:
		i = i.strip().split('=', 2)
		if len(i) == 2 and i[0] in ('GOROOT', 'GOPATH'):
			e[str(i[0])] = str(i[1].strip('\'"'))

	os.environ.update(e)

	x = c.exception()
	if x or not e:
		s = '\n>    '.join(l)
		heading = 'Possible error while attempting to get environment variables:'
		tpl = '%s\n|    Command: %s\n|    Exception: %s\n|    Output:\n>    %s'
		gs.show_output(DOMAIN, tpl % (heading, c.cmd, x, s), merge_domain=True)

	do_install()

def output_str(c):
	return '\n'.join(['>    %s' % ln for ln in c.consume_outq()])

def do_install():
	c = gsshell.Command(
		cmd=INSTALL_CMD,
		cwd=BUNDLE_GOPATH,
		env={
			'GOPATH': BUNDLE_GOPATH,
			'GOBIN': BUNDLE_GOBIN,
		})
	c.on_done = on_install_done
	c.start()

def on_install_done(c):
	s = output_str(c)
	x = c.exception()
	if x:
		tpl = 'Error while installing MarGo and Gocode\nCommand: %s\nException: %s\nOutput: %s'
		gs.show_output(DOMAIN, tpl % (c.cmd, x, s), merge_domain=True)
	print_install_log(c, s)

enabled = False

try:
	BUNDLE_GOPATH = os.path.join(sublime.packages_path(), 'GoSublime', '9')
	BUNDLE_GOBIN = os.path.join(BUNDLE_GOPATH, 'bin')

	if enabled:
		e = gs.env()
		if e.get('GOROOT') and e.get('GOPATH'):
			do_install()
		else:
			gs.println('%s: attempting to set GOROOT and/or GOPATH' % DOMAIN)
			if gs.os_is_windows():
				cmd = 'go env & echo GOPATH=%GOPATH%'
			else:
				cmd = 'go env; echo GOPATH=$GOPATH'
			c = gsshell.Command(cmd=cmd, shell=True)
			c.on_done = on_env_done
			c.start()
except Exception:
	gs.show_traceback(DOMAIN)