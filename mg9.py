import gscommon as gs
import gsshell
import sublime
import threading
import os
import gsq
import time
import hashlib
import base64
import Queue
import uuid
import margo

DOMAIN = 'MarGo9'

# customization of, or user-owned gocode and margo will no longer be supported
# so we'll hardcode the relevant paths and refer to them directly instead of relying on PATH
MARGO0_SRC = gs.dist_path('something_borrowed/margo0')
MARGO9_SRC = gs.dist_path('margo9')
GOCODE_SRC = gs.dist_path('something_borrowed/gocode')
MARGO0_BIN = gs.home_path('bin', 'gosublime.margo0.exe')
MARGO9_BIN = gs.home_path('bin', 'gosublime.margo9.exe')
GOCODE_BIN = gs.home_path('bin', 'gosublime.gocode.exe')

if not gs.checked(DOMAIN, '_vars'):
	_send_q = Queue.Queue()
	_recv_q = Queue.Queue()
	_stash = {}

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
	nv = {
		'GOBIN': '',
		'GOPATH': gs.dist_path('something_borrowed'),
	}
	return gsshell.run(cmd, shell=shell, cwd=cwd, env=nv)

def _bins_exist():
	return os.path.exists(MARGO9_BIN) and os.path.exists(MARGO0_BIN) and os.path.exists(GOCODE_BIN)

def maybe_install():
	if not _bins_exist():
		install('', True)

def install(aso_tokens, force_install):
	init_start = time.time()

	try:
		os.makedirs(gs.home_path('bin'))
	except:
		pass

	if not force_install and _bins_exist() and aso_tokens == _gen_tokens():
		m0_out = 'no'
		m_out = 'no'
		g_out = 'no'
	else:
		start = time.time()
		m0_out, err, _ = _run(['go', 'build', '-o', MARGO0_BIN], cwd=MARGO0_SRC)
		m0_out, m0_ok = _so(m0_out, err, start, time.time())

		if os.path.exists(GOCODE_BIN):
			margo.bye_ni()

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

		if m_ok and m0_ok and g_ok:
			def f():
				gs.aso().set('mg9_install_tokens', _gen_tokens())
				gs.save_aso()

			sublime.set_timeout(f, 0)

	a = (
		'GoSublime init (%0.3fs)' % (time.time() - init_start),
		'| install margo0: %s' % m0_out,
		'| install margo9: %s' % m_out,
		'| install gocode: %s' % g_out,
		'|           ~bin: %s' % gs.home_path('bin'),
		'|         margo0: %s (%s)' % _tp(MARGO0_BIN),
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
	return '\n'.join([
		_token('something_borrowed/margo0.head', MARGO9_BIN),
		_token('margo9.head', MARGO9_BIN),
		_token('something_borrowed/gocode.head', GOCODE_BIN),
	])

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

def gocode(args, env={}, input=None):
	last_propose = gs.attr('gocode.last_propose_builtins', False)
	propose = gs.setting('complete_builtins', False)
	if last_propose != propose:
		gs.set_attr('gocode.last_propose_builtins', propose)
		_gocode(['set', 'propose-builtins', 'true' if propose else 'false'])

	last_gopath = gs.attr('gocode.last_gopath')
	gopath = gs.getenv('GOPATH')
	if gopath and gopath != last_gopath:
		out, _, _ = gsshell.run(cmd=['go', 'env', 'GOOS', 'GOARCH'])
		vars = out.strip().split()
		if len(vars) == 2:
			gs.set_attr('gocode.last_gopath', gopath)
			libpath = []
			osarch = '_'.join(vars)
			for p in gopath.split(os.pathsep):
				if p:
					libpath.append(os.path.join(p, 'pkg', osarch))
			_gocode(['set', 'lib-path', os.pathsep.join(libpath)])

	return _gocode(args, env=env, input=input)

def acall(method, arg, cb):
	if not gs.checked(DOMAIN, 'launch _send'):
		gsq.launch(DOMAIN, _send)

	_send_q.put((method, arg, cb))

def bcall(method, arg, shell=False):
	maybe_install()

	header, _ = gs.json_encode({'method': method, 'token': 'mg9.call'})
	body, _ = gs.json_encode(arg)
	s = '%s %s' % (header, body)
	s = 'base64:%s' % base64.b64encode(s)
	out, err, _ = gsshell.run([MARGO9_BIN, '-do', s], stderr=gs.LOGFILE, shell=shell)
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


def _recv():
	while True:
		try:
			ln = _recv_q.get()
			try:
				ln = ln.strip()
				if ln:
					r, _ = gs.json_decode(ln, {})
					token = r.get('token', '')
					f = _stash.get(token)
					if f:
						del _stash[token]
						f(r.get('data', {}), r.get('error', ''))
			except Exception:
				gs.println(gs.traceback())
		except Exception:
			gs.println(gs.traceback())
			break

def _send():
	while True:
		try:
			try:
				method, arg, cb = _send_q.get()

				proc = gs.attr('mg9.proc')
				if not proc or proc.poll() is not None:
					if proc:
						try:
							proc.kill()
							proc.stdout.close()
						except:
							pass

					maybe_install()

					if not gs.checked(DOMAIN, 'launch _recv'):
						gsq.launch(DOMAIN, _recv)

					# ideally the env should be setup before-hand with a bcall
					# so we won't run this through the shell
					proc, _, err = gsshell.proc([MARGO9_BIN, '-poll=5'], stderr=gs.LOGFILE)
					gs.set_attr('mg9.proc', proc)

					if not proc:
						gs.notice(DOMAIN, 'Cannot start MarGo9: %s' % err)
						continue

					gsq.launch(DOMAIN, lambda: _read_stdout(proc))

				token = 'mg9.autoken.%s' % uuid.uuid4()
				_stash[token] = cb

				header, _ = gs.json_encode({'method': method, 'token': token})
				body, _ = gs.json_encode(arg)
				ln = '%s %s\n' % (header, body)
				proc.stdin.write(ln)
			except Exception:
				gs.println(gs.traceback())
		except Exception:
			gs.println(gs.traceback())
			break

def _read_stdout(proc):
	try:
		while True:
			ln = proc.stdout.readline()
			if not ln:
				break

			_recv_q.put(ln)
	except Exception:
		gs.println(gs.traceback())

		proc.stdout.close()
		proc.wait()
		proc = None

if not gs.checked(DOMAIN, 'do_init'):
	sublime.set_timeout(do_init, 0)

