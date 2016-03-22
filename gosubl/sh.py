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
		nv.update(nv0)
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

	cmdl = []
	for k in vars:
		cmdl.append('[[[$'+k+']]'+k+'[[%'+k+'%]]]')
	cmd_str = 'echo "%s"' % ' '.join(cmdl)

	cr = ShellCommand(cmd_str).run()
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

	if not _env_ext.get('GOROOT'):
		m = re.search(r'\bGOROOT=(.+)', go('env'))
		if m:
			_env_ext['GOROOT'] = m.group(1).strip('"')

	for k in _env_ext:
		v = os.environ.get(k)
		if v:
			_env_ext[k] = v

	cr_go = go_cmd(['version']).run()
	cr_go_out = cr_go.out + cr_go.err
	m = about.GO_VERSION_OUTPUT_PAT.search(cr_go_out)
	if m:
		GO_VERSION = about.GO_VERSION_NORM_PAT.sub('', m.group(1))
		VDIR_NAME = '%s_%s' % (about.VERSION, GO_VERSION)

	dur = (time.time() - start)

	ev.debug('sh.init', {
		'cr.init': cr,
		'cr.go': cr_go,
		'go_version': GO_VERSION,
		'env': _env_ext,
		'dur': dur,
	})

	cmd_lst = []
	for v in cr.cmd_lst:
		v = v.replace(cmd_str, 'echo "..."')
		cmd_lst.append(v)

	_print('load env vars %s: go version: %s -> `%s` -> `%s`: %0.3fs' % (
		cmd_lst,
		cr_go.cmd_lst,
		cr_go_out,
		(GO_VERSION if GO_VERSION != about.DEFAULT_GO_VERSION else cr_go),
		dur,
	))

	init_done = True

def _print(s):
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
	e = os.environ.copy()
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

	add_path = [bin_dir()]

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

	e['PATH'] = psep.join(add_path)

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
	return _which(cmd, getenv('PATH', ''))

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

init_done = False
GO_VERSION = about.DEFAULT_GO_VERSION
VDIR_NAME = '%s_%s' % (about.VERSION, GO_VERSION)
_env_ext = {}
