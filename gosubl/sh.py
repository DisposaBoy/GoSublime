from . import about
from . import gs
from collections import namedtuple
import os
import re
import string
import subprocess
import time

try:
	STARTUP_INFO = subprocess.STARTUPINFO()
	STARTUP_INFO.dwFlags |= subprocess.STARTF_USESHOWWINDOW
	STARTUP_INFO.wShowWindow = subprocess.SW_HIDE
except (AttributeError):
	STARTUP_INFO = None

Result = namedtuple('Result', 'out err ok exc')

class Command(object):
	def __init__(self, cmd_str):
		self.cmd_str = gs.astr(cmd_str)
		self.input = None
		self.stdin = subprocess.PIPE
		self.stdout = subprocess.PIPE
		self.stderr = subprocess.PIPE
		self.startupinfo = STARTUP_INFO
		self.wd = None
		self.env = {}

	def run(self):
		if self.wd:
			wd = self.wd
			try:
				os.makedirs(wd)
			except Exception:
				pass
		else:
			wd = None

		if self.input:
			input = gs.astr(self.input)
		else:
			input = None

		try:
			setsid = os.setsid
		except Exception:
			setsid = None

		out = ''
		err = ''
		exc = None

		nv = env(self.env)

		try:
			out, err = subprocess.Popen(
				_cmd(self.cmd_str, nv),
				stdout=self.stdout,
				stderr=self.stderr,
				stdin=self.stdin,
				startupinfo=gs.STARTUP_INFO,
				shell=False,
				env=nv,
				cwd=wd,
				preexec_fn=setsid,
				bufsize=0
			).communicate(input=input)
		except Exception as e:
			exc = e

		return Result(
			out=gs.ustr(out),
			err=gs.ustr(err),
			ok=(not exc),
			exc=exc
		)

def _exists(fn):
	return os.path.exists(os.path.expanduser(fn))

def shl(m={}):
	return _shl(env(m))

def _shl(e):
	l = gs.setting('shell', [])
	if not l:
		fn = e.get('SHELL') or e.get('COMSPEC')
		if fn:
			name, _ = os.path.splitext(os.path.basename(fn))
			f = globals().get('_shl_%s' % name)
			if f:
				l = f(fn)

	if not l:
		if gs.os_is_windows():
			l = _shl_cmd('cmd')
		else:
			l = _shl_sh('sh')

	return l

def _shl_cmd(fn):
	return [fn, '/C', '${CMD}']

def _shl_sh(fn):
	return [fn, '-c', '${CMD}']

def _shl_fish(fn):
	return [fn, '-i', '-c', '${CMD}']

def _shl_bash(fn):
	if _exists('~/.bashrc'):
		return [fn, '-i', '-c', '${CMD}']

	return [fn, '-l', '-c', '${CMD}']

def _shl_zsh(fn):
	if _exists('~/.zshrc'):
		return [fn, '-i', '-c', '${CMD}']

	return [fn, '-l', '-c', '${CMD}']

def cmd(cmd_str, m={}):
	return _cmd(cmd_str, env(m))

def _cmd(cmd_str, e):
	cmdm = {'CMD': cmd_str}
	cmdl = []
	for s in _shl(e):
		s = string.Template(s).safe_substitute(cmdm)
		s = gs.astr(s)
		if s:
			cmdl.append(s)

	return cmdl

def init():
	global _env_ext

	start = time.time()

	vars = [
		'PATH',
		'GOARCH',
		'GOBIN',
		'GOCHAR',
		'GOEXE',
		'GOGCCFLAGS',
		'GOHOSTARCH',
		'GOHOSTOS',
		'GOOS',
		'GOPATH',
		'GOROOT',
		'GOTOOLDIR',
		'CGO_ENABLED',
	]

	cmdl = ['echo']
	for k in vars:
		cmdl.append('[[[$'+k+']]'+k+'[[%'+k+'%]]]')
	cmd_str = ' '.join(cmdl)

	cr = Command(cmd_str).run()
	if cr.exc:
		_print('error loading env vars: %s' % cr.exc)

	out = cr.out + cr.err

	mats = re.findall(r'\[\[\[(.*?)\]\](%s)\[\[(.*?)\]\]\]' % '|'.join(vars), out)
	for m in mats:
		a, k, b = m
		v = ''
		if a != '$'+k:
			v = a
		elif b != '%'+k+'%':
			v = b

		if v:
			_env_ext[k] = v

	_print('load env vars: %0.3fs' % (time.time() - start))

def _print(s):
	print('GoSblime %s sh: %s' % (about.VERSION, s))

def getenv(name, default='', m={}):
	return env(m).get(name, default)

def env(m={}):
	"""
	Assemble environment information needed for correct operation. In particular,
	ensure that directories containing binaries are included in PATH.
	"""
	e = os.environ.copy()
	e.update(_env_ext)
	e.update(m)

	roots = gs.lst(e.get('GOPATH', '').split(os.pathsep), e.get('GOROOT', ''))
	lfn = gs.attr('last_active_go_fn', '')

	comps = lfn.split(os.sep)
	gs_gopath = []
	for i, s in enumerate(comps):
		if s.lower() == "src":
			p = os.sep.join(comps[:i])
			if p not in roots:
				gs_gopath.append(p)
	gs_gopath.reverse()
	e['GS_GOPATH'] = os.pathsep.join(gs_gopath)

	uenv = gs.setting('env', {})
	for k in uenv:
		try:
			uenv[k] = string.Template(uenv[k]).safe_substitute(e)
		except Exception as ex:
			gs.println('%s: Cannot expand env var `%s`: %s' % (NAME, k, ex))

	e.update(uenv)
	e.update(m)

	# For custom values of GOPATH, installed binaries via go install
	# will go into the "bin" dir of the corresponding GOPATH path.
	# Therefore, make sure these paths are included in PATH.

	add_path = [gs.home_dir_path('bin')]

	for s in gs.lst(e.get('GOROOT', ''), e.get('GOPATH', '').split(os.pathsep)):
		if s:
			s = os.path.join(s, 'bin')
			if s not in add_path:
				add_path.append(s)

	gobin = e.get('GOBIN', '')
	if gobin and gobin not in add_path:
		add_path.append(gobin)

	if gs.os_is_windows():
		l = [
			'~\\bin',
			'~\\go\\bin',
			'C:\\Go\\bin',
		]
	else:
		l = [
			'~/bin',
			'~/go/bin',
			'/usr/local/go/bin',
			'/usr/local/opt/go/bin',
			'/usr/local/bin',
			'/usr/bin',
		]

	for s in l:
		s = os.path.expanduser(s)
		if s not in add_path:
			add_path.append(s)

	psep = gs.setting('shell_path_sep') or os.pathsep

	for s in e.get('PATH', '').split(psep):
		if s and s not in add_path:
			add_path.append(s)


	e['PATH'] = psep.join(add_path)

	# Ensure no unicode objects leak through. The reason is twofold:
	# 	* On Windows, Python 2.6 (used by Sublime Text) subprocess.Popen
	# 	  can only take bytestrings as environment variables in the
	#	  "env"	parameter. Reference:
	# 	  https://github.com/DisposaBoy/GoSublime/issues/112
	# 	  http://stackoverflow.com/q/12253014/1670
	#   * Avoids issues with networking too.
	clean_env = {}
	for k, v in e.items():
		try:
			clean_env[gs.astr(k)] = gs.astr(v)
		except Exception as ex:
			gs.println('%s: Bad env: %s' % (NAME, ex))

	return clean_env

def which_ok(fn):
	try:
		return os.path.isfile(fn) and os.access(fn, os.X_OK)
	except Exception:
		return False

def which(cmd):
	if os.path.isabs(cmd):
		return cmd if which_ok(cmd) else ''

	# not supporting PATHEXT. period.
	if gs.os_is_windows():
		cmd = '%s.exe' % cmd

	psep = gs.setting('shell_path_sep') or os.pathsep

	seen = {}
	for p in getenv('PATH', '').split(psep):
		p = os.path.join(p, cmd)
		if p not in seen and which_ok(p):
			return p

		seen[p] = True

	return ''

def go(subcmd_str):
	cr = Command('go '+subcmd_str).run()
	out = cr.out.strip() + '\n' + cr.err.strip()
	return out.strip()

_env_ext = {}
init()
