from . import about
from . import ev
from . import gs
from collections import namedtuple
import os
import re
import string
import sublime
import subprocess
import time

st_environ = os.environ.copy()

try:
	STARTUPINFO = subprocess.STARTUPINFO()
	STARTUPINFO.dwFlags |= subprocess.STARTF_USESHOWWINDOW
	STARTUPINFO.wShowWindow = subprocess.SW_HIDE
except (AttributeError):
	STARTUPINFO = None

Proc = namedtuple('Proc', 'p input orig_cmd cmd_lst env wd ok exc')
Result = namedtuple('Result', 'out cmd_lst err ok exc')
psep = os.pathsep

class _command(object):
	def __init__(self):
		self.input = None
		self.stdin = subprocess.PIPE
		self.stdout = subprocess.PIPE
		self.stderr = subprocess.PIPE
		self.startupinfo = STARTUPINFO
		self.wd = None
		self.env = {}

	def proc(self):
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

		nv0 = {}
		for k in self.env:
			nv0[gs.astr(k)] = gs.astr(self.env[k])

		nv = env(nv0)
		# this line used to be `nv.update(nv0)` but I think it's a mistake
		# because e.g. if you set self.env['PATH'], sh.env() will merge it
		# and then we  go ahead and overwrite it again
		cmd_lst = self.cmd(nv)
		orig_cmd = cmd_lst[0]
		cmd_lst[0] = _which(orig_cmd, nv.get('PATH'))

		try:
			if not cmd_lst[0]:
				raise Exception('Cannot find command `%s`' % orig_cmd)

			p = subprocess.Popen(
				cmd_lst,
				stdout=self.stdout,
				stderr=self.stderr,
				stdin=self.stdin,
				startupinfo=self.startupinfo,
				shell=False,
				env=nv,
				cwd=wd,
				preexec_fn=setsid,
				bufsize=0
			)
		except Exception as e:
			exc = e
			p = None

		return Proc(
			p=p,
			input=input,
			orig_cmd=orig_cmd,
			cmd_lst=cmd_lst,
			env=nv,
			wd=wd,
			ok=(not exc),
			exc=exc
		)

	def run(self):
		out = ''
		err = ''
		exc = None

		pr = self.proc()
		if pr.ok:
			ev.debug('sh.run', pr)

			try:
				out, err = pr.p.communicate(input=pr.input)
			except Exception as e:
				exc = e
		else:
			exc = pr.exc

		return Result(
			out=gs.ustr(out),
			err=gs.ustr(err),
			cmd_lst=pr.cmd_lst,
			ok=(not exc),
			exc=exc
		)

class ShellCommand(_command):
	def __init__(self, cmd_str):
		_command.__init__(self)
		self.cmd_str = gs.astr(cmd_str)

	def cmd(self, e):
		return _cmd(self.cmd_str, e)

class Command(_command):
	def __init__(self, cmd_lst):
		_command.__init__(self)
		self.cmd_lst = [gs.astr(s) for s in cmd_lst]

	def cmd(self, e):
		return self.cmd_lst

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
	return [fn, '-l', '-c', '${CMD}']

_shl_fish = _shl_sh
_shl_bash = _shl_sh
_shl_zsh = _shl_sh
_shl_rc = _shl_sh

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

def gs_init(_={}):
	global _env_ext
	global GO_VERSION
	global VDIR_NAME
	global init_done

	start = time.time()

	vars = [
		'PATH',
		'GOBIN',
		'GOPATH',
		'GOROOT',
		'CGO_ENABLED',
	]

	cmd = ShellCommand('go run sh-bootstrap.go')
	cmd.wd = gs.dist_path('gosubl')
	cr = cmd.run()
	raw_ver = ''
	ver = ''
	if cr.exc or cr.err:
		_print('error running sh-bootstrap.go: %s' % (cr.exc or cr.err))

	for ln in cr.out.split('\n'):
		ln = ln.strip()
		if not ln:
			continue

		v, err = gs.json_decode(ln, {})
		if err:
			_print('cannot decode sh-bootstrap.go output: `%s`' % (ln))
			continue

		if not gs.is_a(v, {}):
			_print('cannot decode sh-bootstrap.go output: `%s`. value: `%s` is not a dict' % (ln, v))
			continue

		env = v.get('Env')
		if not gs.is_a(env, {}):
			_print('cannot decode sh-bootstrap.go output: `%s`. Env: `%s` is not a dict' % (ln, env))
			continue

		ver = v.get('Version') or ver
		raw_ver = v.get('RawVersion') or raw_ver
		_env_ext.update(env)

	if ver:
		GO_VERSION = ver
		VDIR_NAME = '%s_%s' % (about.VERSION, GO_VERSION)


	_env_keys = sorted(_env_ext.keys())

	for k in _env_keys:
		v = _env_ext[k]
		x = st_environ.get(k)
		if k != 'PATH' and x and x != v:
			_print('WARNING: %s is different between your shell and ST/GUI environments' % (k))
			_print('     shell.%s: %s' % (k, v))
			_print('    st/gui.%s: %s' % (k, x))

	for k in _env_keys:
		_print('using shell env %s=%s' % (k, _env_ext[k]))

	_print('go version: `%s` (raw version string `%s`)' % (
		(GO_VERSION if GO_VERSION != about.DEFAULT_GO_VERSION else ver),
		raw_ver
	))


	dur = (time.time() - start)
	_print('shell bootstrap took %0.3fs' % (dur))

	ev.debug('sh.init', {
		'cr': cr,
		'go_version': GO_VERSION,
		'env': _env_ext,
		'dur': dur,
	})

	export_env()
	gs.sync_settings_callbacks.append(export_env)

	init_done = True

_print_log = []
def _print(s):
	_print_log.append(s)
	print('GoSublime %s sh: %s' % (about.VERSION, s))

def getenv(name, default='', m={}):
	return env(m).get(name, default)

def gs_gopath(fn, roots=[]):
	comps = fn.split(os.sep)
	l = []
	for i, s in enumerate(comps):
		if s.lower() == "src":
			p = os.path.normpath(os.sep.join(comps[:i]))
			if p not in roots:
				l.append(p)
	l.reverse()
	return psep.join(l)

def env(m={}):
	"""
	Assemble environment information needed for correct operation. In particular,
	ensure that directories containing binaries are included in PATH.
	"""

	# TODO: cleanup this function, it just keeps growing crap

	add_path = []
	
	if 'PATH' in m:
		for s in m['PATH'].split(psep):
			if s and s not in add_path:
				add_path.append(s)

		# remove PATH so we don't overwrite the `e[PATH]` below
		m = m.copy()
		del m['PATH']

	add_path.append(bin_dir())

	e = st_environ.copy()
	e.update(_env_ext)
	e.update(m)

	roots = [os.path.normpath(s) for s in gs.lst(e.get('GOPATH', '').split(psep), e.get('GOROOT', ''))]
	e['GS_GOPATH'] = gs_gopath(gs.getwd(), roots) or gs_gopath(gs.attr('last_active_go_fn', ''), roots)

	uenv = gs.setting('env', {})
	for k in uenv:
		try:
			uenv[k] = string.Template(uenv[k]).safe_substitute(e)
		except Exception as ex:
			gs.println('%s: Cannot expand env var `%s`: %s' % (NAME, k, ex))

	e.update(uenv)
	e.update(m)

	if e['GS_GOPATH'] and gs.setting('use_gs_gopath') is True:
		e['GOPATH'] = e['GS_GOPATH']

	# For custom values of GOPATH, installed binaries via go install
	# will go into the "bin" dir of the corresponding GOPATH path.
	# Therefore, make sure these paths are included in PATH.
	for s in gs.lst(e.get('GOROOT', ''), e.get('GOPATH', '').split(psep)):
		if s:
			s = os.path.join(s, 'bin')
			if s not in add_path:
				add_path.append(s)

	gobin = e.get('GOBIN', '')
	if gobin and gobin not in add_path:
		add_path.append(gobin)

	for s in e.get('PATH', '').split(psep):
		if s and s not in add_path:
			add_path.append(s)

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

	e['PATH'] = psep.join(filter(bool, add_path))

	fn = gs.attr('active_fn', '')
	wd =  gs.getwd()

	e.update({
		'PWD': wd,
		'_wd': wd,
		'_dir': os.path.dirname(fn),
		'_fn': fn,
		'_vfn': gs.attr('active_vfn', ''),
		'_nm': fn.replace('\\', '/').split('/')[-1],
	})

	if not e.get('GOPATH'):
		gp = os.path.expanduser('~/go')
		e['GOPATH'] = gp
		_print('GOPATH is not set... setting it to the default: %s' % gp)

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

def which(cmd, m={}):
	return _which(cmd, getenv('PATH', '', m=m))

def _which(cmd, env_path):
	if os.path.isabs(cmd):
		return cmd if which_ok(cmd) else ''

	# not supporting PATHEXT. period.
	if gs.os_is_windows() and not cmd.endswith('.exe'):
		cmd = '%s.exe' % cmd

	seen = {}
	for p in env_path.split(psep):
		p = os.path.join(p, cmd)
		if p not in seen and which_ok(p):
			return p

		seen[p] = True

	return ''

def go_cmd(cmd_lst):
	go = which('go')
	if go:
		return Command(gs.lst(go, cmd_lst))
	return ShellCommand('go %s' % (' '.join(cmd_lst)))

def go(cmd_lst):
	cr = go_cmd(cmd_lst).run()
	out = cr.out.strip() + '\n' + cr.err.strip()
	return out.strip()

def vdir():
	return gs.home_dir_path(VDIR_NAME)

def bin_dir():
	if not init_done:
		# bootstrapping issue:
		#	* gs_init useds ShellCommand to run the go command in order to init GO_VERSION
		#	* ShellCommand calls env()
		#	* env() calls bin_dir()
		#	* we(bin_dir()) use GO_VERSION
		return ''

	return gs.home_dir_path(VDIR_NAME, 'bin')

def exe(nm):
	if gs.os_is_windows():
		nm = '%s.exe' % nm

	return os.path.join(bin_dir(), nm)

def export_env():
	e = env()
	for k in gs.setting('export_env_vars'):
		v = e.get(k)
		if v:
			# note-to-self: don't get any clever ideas about changing this to os.putenv()! because Python...
			os.environ[k] = v

init_done = False
GO_VERSION = about.DEFAULT_GO_VERSION
VDIR_NAME = '%s_%s' % (about.VERSION, GO_VERSION)
_env_ext = {}
