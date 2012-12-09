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
import json
import atexit

DOMAIN = 'MarGo9'
REQUEST_PREFIX = '%s.rqst.' % DOMAIN
PROC_ATTR_NAME = 'mg9.proc'

# customization of, or user-owned margo will no longer be supported
# so we'll hardcode the relevant paths and refer to them directly instead of relying on PATH
MARGO0_SRC = gs.dist_path('something_borrowed/margo0')
MARGO9_SRC = gs.dist_path('margo9')
MARGO0_EXE = 'gosublime.margo0.exe'
MARGO9_EXE = 'gosublime.margo9.exe'
MARGO0_BIN = gs.home_path('bin', MARGO0_EXE)
MARGO9_BIN = gs.home_path('bin', MARGO9_EXE)

if not gs.checked(DOMAIN, '_vars'):
	_send_q = Queue.Queue()
	_recv_q = Queue.Queue()

class Request(object):
	def __init__(self, f, method='', token=''):
		self.f = f
		self.tm = time.time()
		self.method = method
		if token:
			self.token = token
		else:
			self.token = 'mg9.autoken.%s' % uuid.uuid4()

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
	return os.path.exists(MARGO9_BIN) and os.path.exists(MARGO0_BIN)

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
	else:
		gs.notify('GoSublime', 'Installing MarGo0')
		start = time.time()
		m0_out, err, _ = _run(['go', 'build', '-o', MARGO0_BIN], cwd=MARGO0_SRC)
		m0_out, m0_ok = _so(m0_out, err, start, time.time())

		if os.path.exists(MARGO0_BIN):
			margo.bye_ni()

		gs.notify('GoSublime', 'Installing MarGo9')
		start = time.time()
		m_out, err, _ = _run(['go', 'build', '-o', MARGO9_BIN], cwd=MARGO9_SRC)
		m_out, m_ok = _so(m_out, err, start, time.time())

		if m_ok and m0_ok:
			def f():
				gs.aso().set('mg9_install_tokens', _gen_tokens())
				gs.save_aso()

			sublime.set_timeout(f, 0)

	gs.notify('GoSublime', 'Syncing environment variables')
	out, err, _ = gsshell.run([MARGO9_EXE, '-env'], cwd=gs.home_path(), shell=True)

	# notify this early so we don't mask any notices below
	gs.notify('GoSublime', 'Ready')

	if err:
		gs.notice(DOMAIN, 'Cannot run get env vars: %s' % (MARGO9_EXE, err))
	else:
		env, err = gs.json_decode(out, {})
		if err:
			gs.notice(DOMAIN, 'Cannot load env vars: %s\nenv output: %s' % (err, out))
		else:
			gs.environ9.update(env)

	e = gs.env()
	a = (
		'GoSublime init (%0.3fs)' % (time.time() - init_start),
		'| install margo0: %s' % m0_out,
		'| install margo9: %s' % m_out,
		'|           ~bin: %s' % gs.home_path('bin'),
		'|         margo0: %s (%s)' % _tp(MARGO0_BIN),
		'|         margo9: %s (%s)' % _tp(MARGO9_BIN),
		'|         GOROOT: %s' % e.get('GOROOT', '(not set)'),
		'|         GOPATH: %s' % e.get('GOPATH', '(not set)'),
		'|          GOBIN: %s (should usually be (not set))' % e.get('GOBIN', '(not set)'),
	)
	gs.println(*a)

	missing = [k for k in ('GOROOT', 'GOPATH') if not e.get(k)]
	if missing:
		gs.notice(DOMAIN, "Missing environment variable(s): %s" % ', '.join(missing))

	killSrv()
	start = time.time()
	acall('ping', {}, lambda res, err: gs.println('MarGo Ready %0.3fs' % (time.time() - start)))

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
	])

def do_init():
	aso_tokens = gs.aso().get('mg9_install_tokens', '')
	f = lambda: install(aso_tokens, False)
	gsq.do('GoSublime', f, msg='Installing MarGo', set_status=True)

def completion_options(m={}):
	res, err = bcall('gocode_options', {})
	res = gs.dval(res.get('options'), {})
	return res, err

def complete(fn, src, pos):
	home = gs.home_path()
	builtins = (gs.setting('autocomplete_builtins') is True or gs.setting('complete_builtins') is True)
	res, err = bcall('gocode_complete', {
		'Dir': gs.basedir_or_cwd(fn),
		'Builtins': builtins,
		'Fn':  fn or '',
		'Src': src or '',
		'Pos': pos or 0,
		'Home': home,
		'Env': gs.env({
			'XDG_CONFIG_HOME': home,
		}),
	})

	res = gs.dval(res.get('completions'), [])
	return res, err

def fmt(fn, src):
	res, err = bcall('fmt', {
		'fn': fn or '',
		'src': src or '',
		'tabIndent': gs.setting('fmt_tab_indent'),
		'tabWidth': gs.setting('fmt_tab_width'),
	})
	return res.get('src', ''), err

def acall(method, arg, cb):
	if not gs.checked(DOMAIN, 'launch _send'):
		gsq.launch(DOMAIN, _send)

	_send_q.put((method, arg, cb))

def bcall(method, arg):
	q = Queue.Queue()
	acall(method, arg, lambda r,e: q.put((r, e)))
	try:
		res, err = q.get(True, 1)
		return res, err
	except:
		return {}, 'Blocking Call: Timeout'

def _recv():
	while True:
		try:
			ln = _recv_q.get()
			try:
				ln = ln.strip()
				if ln:
					r, _ = gs.json_decode(ln, {})
					token = r.get('token', '')
					k = REQUEST_PREFIX+token
					req = gs.attr(k)
					gs.del_attr(k)
					if req and req.f:
						gs.debug(DOMAIN, "margo response: method: %s, token: %s, dur: %0.3fs, err: `%s'" % (
							req.method,
							req.token,
							(time.time() - req.tm),
							r.get('error', ''),
						))
						keep = req.f(r.get('data', {}), r.get('error', '')) is not True
						if keep:
							req.tm = time.time()
							gs.set_attr(k, req)
					else:
						gs.debug(DOMAIN, 'Ignoring margo: token: %s' % token)

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

				proc = gs.attr(PROC_ATTR_NAME)
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

					proc, _, err = gsshell.proc([MARGO9_BIN, '-poll=30'], stderr=gs.LOGFILE)
					gs.set_attr(PROC_ATTR_NAME, proc)

					if not proc:
						gs.notice(DOMAIN, 'Cannot MarGo: %s' % err)
						continue

					gsq.launch(DOMAIN, lambda: _read_stdout(proc))

				req = Request(f=cb, method=method)
				gs.set_attr(REQUEST_PREFIX+req.token, req)

				gs.debug(DOMAIN, 'margo request: method: %s, token: %s' % (req.method, req.token))

				header, _ = gs.json_encode({'method': method, 'token': req.token})
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

def killSrv():
	p = gs.del_attr(PROC_ATTR_NAME)
	if p:
		print p
		try:
			p.stdout.close()
		except Exception:
			pass

		try:
			p.stdin.close()
		except Exception:
			pass

		try:
			p.kill()
		except Exception:
			pass

def _dump(res, err):
	gs.println(json.dumps({
		'res': res,
		'err': err,
	}, sort_keys=True, indent=2))

if not gs.checked(DOMAIN, 'do_init'):
	atexit.register(killSrv)
	sublime.set_timeout(do_init, 0)
