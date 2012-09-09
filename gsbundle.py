# Sublime modelines - https://github.com/SublimeText/Modelines
# sublime: translate_tabs_to_spaces false; rulers [100,120]

import gsshell, gscommon as gs
import os, re
import sublime

DOMAIN = 'GsBundle'
INSTALL_CMD = ['go', 'install', '-v','margo', 'gocode']
ENV_PATH = re.compile(r'(?P<name>\w+)=["\']?(?P<value>.+?)["\']?$')

def print_install_log(cmd, s):
	e = gs.env()
	dur = cmd.ended - cmd.started
	gs.println(
		'GoSublime: %s done %0.3fs' % (DOMAIN, dur),
		'|  Bundle GOPATH: %s' % BUNDLE_GOPATH,
		'|   Bundle GOBIN: %s' % BUNDLE_GOBIN,
		'|  Bundle Gocode: %s (exists: %s)' % (BUNDLE_GOCODE, os.path.exists(BUNDLE_GOCODE)),
		'|   Bundle MarGo: %s (exists: %s)' % (BUNDLE_MARGO, os.path.exists(BUNDLE_MARGO)),
		'|    User GOROOT: %s' % e.get('GOROOT', '(NOT SET)'),
		'|    User GOPATH: %s' % e.get('GOPATH', '(NOT SET)'),
		'|     User GOBIN: %s (should usually be `NOT SET\')' % e.get('GOBIN', '(NOT SET)'),
		'| Output:\n%s\n' % s
	)

	CRITICAL_ENV_VARS = ('GOROOT', 'GOPATH')
	unset_vars = [var for var in CRITICAL_ENV_VARS if not e.get(var)]
	if unset_vars:
		tpl = 'check the console for error messages: the following environment variables are not set: %s'
		gs.notice(DOMAIN, tpl % ', '.join(unset_vars))

def on_env_done(cmd):
	l = cmd.consume_outq()
	e = {}
	for i in l:
		pair = getattr(ENV_PATH.search(i), "groupdict", dict)()
		if pair:
			k, v = str(pair['name']), str(pair['value'])
			if k in ('GOROOT', 'GOPATH'):
				e[k] = v

	os.environ.update(e)

	x = cmd.exception()
	if x or not e:
		s = '\n>    '.join(l)
		heading = 'Possible error while attempting to get environment variables:'
		tpl = '%s\n|    Command: %s\n|    Exception: %s\n|    Output:\n>    %s'
		gs.show_output(DOMAIN, tpl % (heading, cmd.cmd, x, s), merge_domain=True)

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

def on_gocode_done(c):
	s = '\n'.join(c.consume_outq())
	x = c.exception()
	if x:
		gs.notice(DOMAIN, 'Gocode Error: %s\nOutput: %s' % (x, s))
	else:
		gsshell.Command(cmd=[BUNDLE_GOCODE], cwd=BUNDLE_GOBIN).start()

def on_margo_done(c):
	s = '\n'.join(c.consume_outq())
	x = c.exception()
	if x:
		gs.notice(DOMAIN, 'MarGo Error: %s\nOutput: %s' % (x, s))
	else:
		gs.println('%s: MarGo: %s' % (DOMAIN, s))

def on_install_done(c):
	s = output_str(c)
	x = c.exception()
	if x:
		tpl = 'Error while installing MarGo and Gocode\nCommand: %s\nException: %s\nOutput: %s'
		gs.show_output(DOMAIN, tpl % (c.cmd, x, s), merge_domain=True)
	print_install_log(c, s)

	c = gsshell.Command(cmd=[
		BUNDLE_MARGO,
		"-d",
		"-call", "replace",
		"-addr", gs.setting('margo_addr', '')
	])
	c.on_done = on_margo_done
	c.start()

	c = gsshell.Command(cmd=[BUNDLE_GOCODE, 'close'])
	c.on_done = on_gocode_done
	c.start()

enabled = True

try:
	# We have to build absolute paths so that some os/exec.Command calls work as expected on
	# Windows. When calling subprocesses, Go always completes partial names with PATHEXT values
	# (unlike CreateProcess). If there is a margo.* executable in the current directory and it isn't
	# the expected margo.exe binary, MarGo.exe will throw an error or behave unexpectedly.
	# See: https://github.com/DisposaBoy/GoSublime/issues/126 (#126)
	BUNDLE_GOPATH = os.path.join(sublime.packages_path(), 'GoSublime', '9')
	BUNDLE_GOBIN = os.path.join(BUNDLE_GOPATH, 'bin')
	BUNDLE_GOCODE = os.path.join(BUNDLE_GOBIN, 'gocode')
	BUNDLE_MARGO = os.path.join(BUNDLE_GOBIN, 'margo')
	if gs.os_is_windows():
		BUNDLE_GOCODE = '%s.exe' % BUNDLE_GOCODE
		BUNDLE_MARGO = '%s.exe' % BUNDLE_MARGO
	os.environ['PATH'] = '%s%s%s' % (BUNDLE_GOBIN, os.pathsep, os.environ.get('PATH', ''))

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