# Sublime modelines - https://github.com/SublimeText/Modelines
# sublime: translate_tabs_to_spaces false; rulers [100,120]

import gsshell
import gscommon as gs
import os
import sublime
import json

DOMAIN = 'GsBundle'
INSTALL_CMD = ['go', 'install', '-v', 'gosublime9', 'margo', 'gocode']

def print_install_log(c, s):
	e = gs.env()
	dur = c.ended - c.started
	pkgdir = sublime.packages_path()
	subl9_status = (BUNDLE_GOSUBLIME9.replace(pkgdir, 'Packages'), os.path.exists(BUNDLE_GOSUBLIME9))
	margo_status = (BUNDLE_GOCODE.replace(pkgdir, 'Packages'), os.path.exists(BUNDLE_GOCODE))
	gocode_status = (BUNDLE_MARGO.replace(pkgdir, 'Packages'), os.path.exists(BUNDLE_MARGO))
	gs.println(
		'GoSublime: %s done %0.3fs' % (DOMAIN, dur),
		'|      Bundle GOPATH: %s' % BUNDLE_GOPATH.replace(pkgdir, 'Packages'),
		'|       Bundle GOBIN: %s' % BUNDLE_GOBIN.replace(pkgdir, 'Packages'),
		'|      Bundle Gocode: %s (exists: %s)' % gocode_status,
		'|  Bundle GoSublime9: %s (exists: %s)' % subl9_status,
		'|       Bundle MarGo: %s (exists: %s)' % margo_status,
		'|        User GOROOT: %s' % e.get('GOROOT', '(NOT SET)'),
		'|        User GOPATH: %s' % e.get('GOPATH', '(NOT SET)'),
		'|         User GOBIN: %s (should usually be `NOT SET\')' % e.get('GOBIN', '(NOT SET)'),
		s
	)

	CRITICAL_ENV_VARS = ('GOROOT', 'GOPATH')
	unset_vars = [var for var in CRITICAL_ENV_VARS if not e.get(var)]
	if unset_vars:
		tpl = 'check the console for error messages: the following environment variables are not set: %s'
		gs.notice(DOMAIN, tpl % ', '.join(unset_vars))

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
		tpl = 'Error while installing dependencies\nCommand: %s\nException: %s\nOutput: %s'
		gs.show_output(DOMAIN, tpl % (c.cmd, x, s), merge_domain=True)

	js, _, _ = gsshell.run(cmd=BUNDLE_GOSUBLIME9, shell=True)
	js = json.loads(js)
	for k,v in js.iteritems():
		if v:
			gs.environ9[k] = v

	print_install_log(c, s)

	c = gsshell.Command(cmd=[
		BUNDLE_MARGO,
		"-d",
		"-call", "replace",
		"-addr", gs.setting('margo_addr', '')
	])
	c.on_done = on_margo_done
	c.start()

	gsshell.run(cmd=[BUNDLE_GOCODE, 'close'])

def init():
	try:
		if gs.settings_obj().get('gsbundle_enabled') is True:
			do_install()
	except Exception:
		gs.show_traceback(DOMAIN)


try:
	# We have to build absolute paths so that some os/exec.Command calls work as expected on
	# Windows. When calling subprocesses, Go always completes partial names with PATHEXT values
	# (unlike CreateProcess). If there is a margo.* executable in the current directory and it isn't
	# the expected margo.exe binary, MarGo.exe will throw an error or behave unexpectedly.
	# See: https://github.com/DisposaBoy/GoSublime/issues/126 (#126)
	ext = '.exe' if gs.os_is_windows() else ''
	BUNDLE_GOPATH = os.path.join(sublime.packages_path(), 'GoSublime', '9')
	BUNDLE_GOBIN = os.path.join(sublime.packages_path(), 'User', 'GoSublime', '9', 'bin')
	BUNDLE_GOSUBLIME9 = os.path.join(BUNDLE_GOBIN, 'gosublime9%s' % ext)
	BUNDLE_GOCODE = os.path.join(BUNDLE_GOBIN, 'gocode%s' % ext)
	BUNDLE_MARGO = os.path.join(BUNDLE_GOBIN, 'margo%s' % ext)
	os.environ['PATH'] = '%s%s%s' % (BUNDLE_GOBIN, os.pathsep, os.environ.get('PATH', ''))

	sublime.set_timeout(init, 1000)
except Exception:
	gs.show_traceback(DOMAIN)
