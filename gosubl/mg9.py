from gosubl import about
from gosubl import gs
from gosubl import gsq
from gosubl import gsshell
import atexit
import base64
import hashlib
import json
import os
import re
import sublime
import threading
import time
import uuid

DOMAIN = 'MarGo'
REQUEST_PREFIX = '%s.rqst.' % DOMAIN
PROC_ATTR_NAME = 'mg9.proc'

def gs_init():
	atexit.register(killSrv)

	aso_tokens = gs.aso().get('mg9_install_tokens', '')
	f = lambda: install(aso_tokens, False)
	gsq.do('GoSublime', f, msg='Installing MarGo', set_status=True)

class Request(object):
	def __init__(self, f, method='', token=''):
		self.f = f
		self.tm = time.time()
		self.method = method
		if token:
			self.token = token
		else:
			self.token = 'mg9.autoken.%s' % uuid.uuid4()

def _margo_src():
	return gs.dist_path('margo9')

def _margo_bin():
	return gs.home_path('bin', about.MARGO_EXE)

def sanity_check(env={}):
	if not env:
		env = gs.env()

	ns = '(not set)'

	return [
		('version', about.VERSION),
		('platform', about.PLATFORM),
		('~bin', '%s' % gs.home_path('bin')),
		('MarGo', '%s (%s)' % _tp(_margo_bin())),
		('GOROOT', '%s' % env.get('GOROOT', ns)),
		('GOPATH', '%s' % env.get('GOPATH', ns)),
		('GOBIN', '%s (should usually be `%s`)' % (env.get('GOBIN', ns), ns)),
	]

def _check_changes():
	def cb():
		aso = gs.aso()
		old_version = aso.get('version', '')
		if about.VERSION > old_version:
			aso.set('version', about.VERSION)
			gs.save_aso()
			gs.focus(gs.dist_path('CHANGELOG.md'))

	sublime.set_timeout(cb, 0)

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
	return os.path.exists(_margo_bin())

def maybe_install():
	if not _bins_exist():
		install('', True)

def install(aso_tokens, force_install):
	k = 'mg9.install.%s' % about.VERSION
	if gs.attr(k, False):
		gs.error(DOMAIN, 'Installation aborted. Install command already called for GoSublime %s.' % about.VERSION)
		return

	gs.set_attr(k, True)

	init_start = time.time()

	try:
		os.makedirs(gs.home_path('bin'))
	except:
		pass

	if not force_install and _bins_exist() and aso_tokens == _gen_tokens():
		m_out = 'no'
	else:
		gs.notify('GoSublime', 'Installing MarGo')
		start = time.time()
		m_out, err, _ = _run(['go', 'build', '-o', _margo_bin()], cwd=_margo_src())
		m_out, m_ok = _so(m_out, err, start, time.time())

		if m_ok:
			def f():
				gs.aso().set('mg9_install_tokens', _gen_tokens())
				gs.save_aso()

			sublime.set_timeout(f, 0)

	gs.notify('GoSublime', 'Syncing environment variables')
	out, err, _ = gsshell.run([about.MARGO_EXE, '-env'], cwd=gs.home_path(), shell=True)

	# notify this early so we don't mask any notices below
	gs.notify('GoSublime', 'Ready')
	_check_changes()

	if err:
		gs.notice(DOMAIN, 'Cannot run get env vars: %s' % (err))
	else:
		env, err = gs.json_decode(out, {})
		if err:
			gs.notice(DOMAIN, 'Cannot load env vars: %s\nenv output: %s' % (err, out))
		else:
			gs.environ9.update(env)

	e = gs.env()
	a = [
		'GoSublime init (%0.3fs)' % (time.time() - init_start),
		'| install margo: %s' % m_out,
	]
	a.extend(['| %14s: %s' % ln for ln in sanity_check(e)])
	gs.println(*a)

	missing = [k for k in ('GOROOT', 'GOPATH') if not e.get(k)]
	if missing:
		gs.notice(DOMAIN, "Missing environment variable(s): %s" % ', '.join(missing))

	killSrv()
	start = time.time()
	# acall('ping', {}, lambda res, err: gs.println('MarGo Ready %0.3fs' % (time.time() - start)))

	report_x = lambda: gs.println("GoSublime: Exception while cleaning up old binaries", gs.traceback())
	try:
		d = gs.home_path('bin')
		old_pat = re.compile(r'^gosublime.r\d{2}.\d{2}.\d{2}-\d+.margo.exe$')
		for fn in os.listdir(d):
			try:
				if fn != about.MARGO_EXE and (about.MARGO_EXE_PAT.match(fn) or old_pat.match(fn)):
					fn = os.path.join(d, fn)
					gs.println("GoSublime: removing old binary: %s" % fn)
					os.remove(fn)
			except Exception:
				report_x()
	except Exception:
		report_x()

def _gen_tokens():
	return about.VERSION

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

def import_paths(fn, src, f):
	tid = gs.begin(DOMAIN, 'Fetching import paths')
	def cb(res, err):
		gs.end(tid)
		f(res, err)

	acall('import_paths', {
		'fn': fn or '',
		'src': src or '',
		'env': gs.env(),
	}, cb)

def pkg_name(fn, src):
	res, err = bcall('pkg', {
		'fn': fn or '',
		'src': src or '',
	})
	return res.get('name'), err

def pkg_dirs(f):
	tid = gs.begin(DOMAIN, 'Fetching pkg dirs')
	def cb(res, err):
		gs.end(tid)
		f(res, err)

	acall('pkg_dirs', {
		'env': gs.env(),
	}, cb)

def declarations(fn, src, pkg_dir, f):
	tid = gs.begin(DOMAIN, 'Fetching declarations')
	def cb(res, err):
		gs.end(tid)
		f(res, err)

	return acall('declarations', {
		'fn': fn or '',
		'src': src,
		'env': gs.env(),
		'pkgDir': pkg_dir,
	}, cb)

def imports(fn, src, toggle):
	return bcall('imports', {
		'fn': fn or '',
		'src': src or '',
		'toggle': toggle or [],
		'tabIndent': gs.setting('fmt_tab_indent'),
		'tabWidth': gs.setting('fmt_tab_width'),
	})

def doc(fn, src, offset, f):
	tid = gs.begin(DOMAIN, 'Fetching doc info')
	def cb(res, err):
		gs.end(tid)
		f(res, err)

	acall('doc', {
		'fn': fn or '',
		'src': src or '',
		'offset': offset or 0,
		'env': gs.env(),
		'tabIndent': gs.setting('fmt_tab_indent'),
		'tabWidth': gs.setting('fmt_tab_width'),
	}, cb)

def share(src, f):
	warning = 'Are you sure you want to share this file. It will be public on play.golang.org'
	if sublime.ok_cancel_dialog(warning):
		acall('share', {'Src': src or ''}, f)
	else:
		f({}, 'Share cancelled')

def acall(method, arg, cb):
	if not gs.checked(DOMAIN, 'launch _send'):
		gsq.launch(DOMAIN, _send)

	gs.mg9_send_q.put((method, arg, cb))

def bcall(method, arg):
	q = gs.queue.Queue()
	acall(method, arg, lambda r,e: q.put((r, e)))
	try:
		res, err = q.get(True, 1)
		return res, err
	except:
		return {}, 'Blocking Call: Timeout'

def expand_jdata(v):
	if gs.is_a(v, {}):
		for k in v:
			v[k] = expand_jdata(v[k])
	else:
		if gs.PY3K and isinstance(v, bytes):
			v = gs.ustr(v)

		if gs.is_a_string(v) and v.startswith('base64:'):
			try:
				v = gs.ustr(base64.b64decode(v[7:]))
			except Exception:
				v = ''
				gs.error_traceback(DOMAIN)
	return v

def _recv():
	while True:
		try:
			ln = gs.mg9_recv_q.get()
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

						dat = expand_jdata(r.get('data', {}))
						err = r.get('error', '')
						try:
							keep = req.f(dat, err) is not True
							if keep:
								req.tm = time.time()
								gs.set_attr(k, req)
						except Exception:
							gs.error_traceback(DOMAIN)
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
				method, arg, cb = gs.mg9_send_q.get()

				proc = gs.attr(PROC_ATTR_NAME)
				if not proc or proc.poll() is not None:
					killSrv()
					maybe_install()

					if not gs.checked(DOMAIN, 'launch _recv'):
						gsq.launch(DOMAIN, _recv)

					proc, _, err = gsshell.proc([_margo_bin(), '-poll=30'], stderr=gs.LOGFILE ,env={
						'XDG_CONFIG_HOME': gs.home_path(),
					})
					gs.set_attr(PROC_ATTR_NAME, proc)

					if not proc:
						gs.notice(DOMAIN, 'Cannot start MarGo: %s' % err)
						try:
							cb({}, 'Abort. Cannot start MarGo')
						except:
							pass
						continue

					gsq.launch(DOMAIN, lambda: _read_stdout(proc))

				req = Request(f=cb, method=method)
				gs.set_attr(REQUEST_PREFIX+req.token, req)

				gs.debug(DOMAIN, 'margo request: method: %s, token: %s' % (req.method, req.token))

				header, _ = gs.json_encode({'method': method, 'token': req.token})
				body, _ = gs.json_encode(arg)
				ln = '%s %s\n' % (header, body)

				if gs.PY3K:
					proc.stdin.write(bytes(ln, 'UTF-8'))
				else:
					proc.stdin.write(ln)
			except Exception:
				killSrv()
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

			gs.mg9_recv_q.put(gs.ustr(ln))
	except Exception:
		gs.println(gs.traceback())

		proc.stdout.close()
		proc.wait()
		proc = None

def killSrv():
	p = gs.del_attr(PROC_ATTR_NAME)
	if p:
		try:
			p.stdout.close()
		except Exception:
			pass

		try:
			p.stdin.close()
		except Exception:
			pass

def _dump(res, err):
	gs.println(json.dumps({
		'res': res,
		'err': err,
	}, sort_keys=True, indent=2))
