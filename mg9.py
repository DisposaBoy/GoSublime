import gscommon as gs
import gsshell
import sublime
import threading
import os
import gsq
import time
import hashlib
import base64

DOMAIN = 'MarGo9'

# customization of, or user-owned gocode and margo will no longer be supported
# so we'll hardcode the relevant paths and refer to them directly instead of relying on PATH
MARGO9_SRC = gs.dist_path('margo9')
GOCODE_SRC = gs.dist_path('gocode')
MARGO9_BIN = gs.home_path('bin', 'gosublime.margo9.exe')
GOCODE_BIN = gs.home_path('bin', 'gosublime.gocode.exe')

def _sb(s):
	bdir = gs.home_path('bin')
	if s.startswith(bdir):
		s = '~bin%s' % (s[len(bdir):])
	return s

def _tp(s):
	return (_sb(s), ('ok' if os.path.exists(s) else 'missing'))

def _so(out, err, start, end):
	out = out.strip()
	err = err.strip()
	ok = not out and not err
	if ok:
		out = 'ok %0.3fs' % (end - start)
	else:
		out = '%s\n%s' % (out, err)
	return (out.strip(), ok)

def _run(cmd, cwd=None, shell=False):
	# we don't want any interference from the user's env so clear all unnecesary vars
	nv = {
		'GOBIN': '',
		'GOPATH': '',
	}
	return gsshell.run(cmd, shell=shell, cwd=cwd, env=nv)

def maybe_install():
	if not os.path.exists(MARGO9_BIN) or not os.path.exists(GOCODE_BIN):
		install('', True)

def install(aso_tokens, force_install):
	init_start = time.time()

	try:
		os.makedirs(gs.home_path('bin'))
	except:
		pass

	if not force_install and aso_tokens == _gen_tokens():
		m_out = 'no'
		g_out = 'no'
	else:
		start = time.time()
		m_out, err, _ = _run(['go', 'build', '-o', MARGO9_BIN], cwd=MARGO9_SRC)
		m_out, m_ok = _so(m_out, err, start, time.time())

		# on windows the file cannot be replaced if it's running so close gocode first.
		# in theory, mg9 has the same issue but since it's attached to a st2 instance,
		# the only way to close it is to close st2 (which we're presumably already doing)
		start = time.time()
		if os.path.exists(GOCODE_BIN):
			_run([GOCODE_BIN, 'close'])

		g_out, err, _ = _run(['go', 'build', '-o', GOCODE_BIN], cwd=GOCODE_SRC)
		g_out, g_ok = _so(g_out, err, start, time.time())

		if m_ok and g_ok:
			def f():
				gs.aso().set('mg9_install_tokens', _gen_tokens())
				gs.save_aso()

			sublime.set_timeout(f, 0)

	a = (
		'GoSublime init (%0.3fs)' % (time.time() - init_start),
		'| install margo9: %s' % m_out,
		'| install gocode: %s' % g_out,
		'|           ~bin: %s' % gs.home_path('bin'),
		'|         margo9: %s (%s)' % _tp(MARGO9_BIN),
		'|         gocode: %s (%s)' % _tp(GOCODE_BIN),
	)
	gs.println(*a)

def _fasthash(fn):
	try:
		with open(fn) as f:
			chunk = f.read(1024*8)
		st = os.stat(fn)
		return '%s:%d' % (hashlib.sha1(chunk).hexdigest(), st.st_size)
	except Exception:
		pass
	return ''

def _read(fn):
	s = ''
	try:
		with open(fn) as f:
			s = f.read()
	except Exception:
		pass
	return s

def _token(head, bin):
	head = _read(gs.dist_path(head))
	token = '%s %s' % (head.strip(), _fasthash(bin))
	return token

def _gen_tokens():
	return '%s\n%s' % (_token('margo9.head', MARGO9_BIN), _token('gocode.head', GOCODE_BIN))

def do_init():
	aso_tokens = gs.aso().get('mg9_install_tokens', '')
	f = lambda: install(aso_tokens, False)
	gsq.do('GoSublime', f, msg='Installing MarGo9 and Gocode', set_status=True)

def _gocode(args, env={}, input=None):
	home = gs.home_path()
	# gocode should store its settings here
	nv = {
		'XDG_CONFIG_HOME': home,
	}
	nv.update(env)

	# until mg9 is in active use we'll fallback to existing gocode
	bin = GOCODE_BIN if os.path.exists(GOCODE_BIN) else 'gocode'
	cmd = gs.lst(bin, args)
	return gsshell.run(cmd, input=input, env=nv, cwd=home)

last_gopath = ''
def gocode(args, env={}, input=None):
	global last_gopath
	gopath = gs.getenv('GOPATH')
	if gopath and gopath != last_gopath:
		out, _, _ = gsshell.run(cmd=['go', 'env', 'GOOS', 'GOARCH'])
		vars = out.strip().split()
		if len(vars) == 2:
			last_gopath = gopath
			libpath = []
			osarch = '_'.join(vars)
			for p in gopath.split(os.pathsep):
				if p:
					libpath.append(os.path.join(p, 'pkg', osarch))
			_gocode(['set', 'lib-path', os.pathsep.join(libpath)])

	return _gocode(args, env=env, input=input)
def do(method, arg, shell=False):
	maybe_install()

	header, _ = gs.json_encode({'method': method, 'token': 'mg9.call'})
	body, _ = gs.json_encode(arg)
	s = '%s %s' % (header, body)
	s = 'base64:%s' % base64.b64encode(s)
	out, err, _ = gsshell.run([MARGO9_BIN, '-do', s], stderr=None, shell=shell)
	res = {'error': err}

	if out:
		try:
			for ln in out.split('\n'):
				ln = ln.strip()
				if ln:
					r, err = gs.json_decode(ln, {})
					if err:
						res = {'error': 'Invalid response %s' % err}
					else:
						if r.get('token') == 'mg9.call':
							res = r.get('data') or {}
							if gs.is_a({}, res) and r.get('error'):
								r['error'] = res['error']
							return res
						res = {'error': 'Unexpected response %s' % r}
		except Exception:
			res = {'error': gs.traceback()}

	return res

if gs.settings_obj().get('test_mg9_enabled') is True:
	sublime.set_timeout(do_init, 1000)
