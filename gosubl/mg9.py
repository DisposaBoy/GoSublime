from gosubl import about
from gosubl import ev
from gosubl import gs
from gosubl import gsq
from gosubl import sh
import atexit
import base64
import hashlib
import json
import os
import re
import sublime
import subprocess
import threading
import time
import uuid

DOMAIN = 'MarGo'
REQUEST_PREFIX = '%s.rqst.' % DOMAIN
PROC_ATTR_NAME = 'mg9.proc'
TAG = about.VERSION
INSTALL_VERSION = about.VERSION
INSTALL_EXE = about.MARGO_EXE

def gs_init(m={}):
	global INSTALL_VERSION
	global INSTALL_EXE

	atexit.register(killSrv)

	version = m.get('version')
	if version:
		INSTALL_VERSION = version

	margo_exe = m.get('margo_exe')
	if margo_exe:
		INSTALL_EXE = margo_exe

	aso_install_vesion = gs.aso().get('install_version', '')
	f = lambda: install(aso_install_vesion, False)
	gsq.do('GoSublime', f, msg='Installing MarGo', set_status=False)

class Request(object):
	def __init__(self, f, method='', token=''):
		self.f = f
		self.tm = time.time()
		self.method = method
		if token:
			self.token = token
		else:
			self.token = 'mg9.autoken.%s' % uuid.uuid4()

	def header(self):
		return {
			'method': self.method,
			'token': self.token,
		}

def _inst_state():
	return gs.attr(_inst_name(), '')

def _inst_name():
	return 'mg9.install.%s' % INSTALL_VERSION

def _margo_src():
	return gs.dist_path('margo9')

def _margo_bin(exe=''):
 	return gs.home_path('bin', exe or INSTALL_EXE)

def sanity_check_sl(sl):
	n = 0
	for p in sl:
		n = max(n, len(p[0]))

	t = '%d' % n
	t = '| %'+t+'s: %s'
	indent = '| %s> ' % (' ' * n)

	a = '~%s' % os.sep
	b = os.path.expanduser(a)

	return [t % (k, gs.ustr(v).replace(b, a).replace('\n', '\n%s' % indent)) for k,v in sl]

def sanity_check(env={}, error_log=False):
	if not env:
		env = sh.env()

	ns = '(not set)'

	sl = [
		('install state', _inst_state()),
		('sublime.version', sublime.version()),
		('sublime.channel', sublime.channel()),
		('about.ann', gs.attr('about.ann', '')),
		('about.version', gs.attr('about.version', '')),
		('version', about.VERSION),
		('platform', about.PLATFORM),
		('~bin', '%s' % gs.home_dir_path('bin')),
		('margo.exe', '%s (%s)' % _tp(_margo_bin())),
		('go.exe', '%s (%s)' % _tp(sh.which('go') or 'go')),
		('go.version', sh.GO_VERSION),
		('GOROOT', '%s' % env.get('GOROOT', ns)),
		('GOPATH', '%s' % env.get('GOPATH', ns)),
		('GOBIN', '%s (should usually be `%s`)' % (env.get('GOBIN', ns), ns)),
		('set.shell', str(gs.lst(gs.setting('shell')))),
		('env.shell', env.get('SHELL', '')),
		('shell.cmd', str(sh.cmd('${CMD}'))),
	]

	if error_log:
		try:
			with open(gs.home_path('log.txt'), 'r') as f:
				s = f.read().strip()
				sl.append(('error log', s))
		except Exception:
			pass

	return sl

def _sb(s):
	bdir = gs.home_dir_path('bin')
	if s.startswith(bdir):
		s = '~bin%s' % (s[len(bdir):])
	return s

def _tp(s):
	return (_sb(s), ('ok' if os.path.exists(s) else 'missing'))

def _bins_exist():
	return os.path.exists(_margo_bin())

def maybe_install():
	if _inst_state() == '' and not _bins_exist():
		install('', True)

def install(aso_install_vesion, force_install):
	global INSTALL_EXE

	if _inst_state() != "":
		gs.notify(DOMAIN, 'Installation aborted. Install command already called for GoSublime %s.' % INSTALL_VERSION)
		return

	INSTALL_EXE = INSTALL_EXE.replace('_%s.exe' % about.DEFAULT_GO_VERSION, '_%s.exe' % sh.GO_VERSION)
	about.MARGO_EXE = INSTALL_EXE

	is_update = about.VERSION != INSTALL_VERSION

	gs.set_attr(_inst_name(), 'busy')

	init_start = time.time()

	if not is_update and not force_install and _bins_exist() and aso_install_vesion == INSTALL_VERSION:
		m_out = 'no'
	else:
		gs.notify('GoSublime', 'Installing MarGo')
		start = time.time()

		cmd = sh.Command(['go', 'build', '-v', '-x', '-o', INSTALL_EXE, 'gosubli.me/margo'])
		cmd.wd = gs.home_dir_path('bin')
		cmd.env = {
			'CGO_ENABLED': '0',
			'GOBIN': '',
			'GOPATH': gs.dist_path(),
		}

		ev.debug('%s.build' % DOMAIN, {
			'cmd': cmd.cmd_lst,
			'cwd': cmd.wd,
		})

		cr = cmd.run()
		m_out = 'cmd: `%s`\nstdout: `%s`\nstderr: `%s`\nexception: `%s`' % (
			cr.cmd_lst,
			cr.out.strip(),
			cr.err.strip(),
			cr.exc,
		)

		if cr.ok and _bins_exist():
			def f():
				gs.aso().set('install_version', INSTALL_VERSION)
				gs.save_aso()

			sublime.set_timeout(f, 0)
		else:
			err_prefix = 'MarGo build failed'
			gs.error(DOMAIN, '%s\n%s' % (err_prefix, m_out))

			sl = [
				('GoSublime error', '\n'.join((
					err_prefix,
					'This is possibly a bug or miss-configuration of your environment.',
					'For more help, please file an issue with the following build output',
					'at: https://github.com/DisposaBoy/GoSublime/issues/new',
					'or alternatively, you may send an email to: gosublime@dby.me',
					'\n',
					m_out,
				)))
			]
			sl.extend(sanity_check({}, False))
			gs.show_output('GoSublime', '\n'.join(sanity_check_sl(sl)))

	gs.set_attr(_inst_name(), 'done')

	if is_update:
		gs.show_output('GoSublime-source', '\n'.join([
			'GoSublime source has been updated.',
			'New version: `%s`, current version: `%s`' % (INSTALL_VERSION, about.VERSION),
			'Please restart Sublime Text to complete the update.',
		]))
	else:
		e = sh.env()
		a = [
			'GoSublime init %s (%0.3fs)' % (INSTALL_VERSION, time.time() - init_start),
		]

		sl = [('install margo', m_out)]
		sl.extend(sanity_check(e))
		a.extend(sanity_check_sl(sl))
		gs.println(*a)

		missing = [k for k in ('GOROOT', 'GOPATH') if not e.get(k)]
		if missing:
			missing_message = '\n'.join([
				'Missing required environment variables: %s' % ' '.join(missing),
				'See the `Quirks` section of USAGE.md for info',
			])

			cb = lambda ok: gs.show_output(DOMAIN, missing_message, merge_domain=True, print_output=False)
			gs.error(DOMAIN, missing_message)
			gs.focus(gs.dist_path('USAGE.md'), focus_pat='^Quirks', cb=cb)

		killSrv()

		start = time.time()
		# acall('ping', {}, lambda res, err: gs.println('MarGo Ready %0.3fs' % (time.time() - start)))

		report_x = lambda: gs.println("GoSublime: Exception while cleaning up old binaries", gs.traceback())
		try:
			bin_dirs = [
				gs.home_path('bin'),
			]

			l = []
			for d in bin_dirs:
				try:
					for fn in os.listdir(d):
						if fn != INSTALL_EXE and about.MARGO_EXE_PAT.match(fn):
							l.append(os.path.join(d, fn))
				except Exception:
					pass

			for fn in l:
				try:
					gs.println("GoSublime: removing old binary: `%s'" % fn)
					os.remove(fn)
				except Exception:
					report_x()

		except Exception:
			report_x()

def calltip(fn, src, pos, quiet, f):
	tid = ''
	if not quiet:
		tid = gs.begin(DOMAIN, 'Fetching calltips')

	def cb(res, err):
		if tid:
			gs.end(tid)

		res = gs.dval(res.get('Candidates'), [])
		f(res, err)

	return acall('gocode_calltip', _complete_opts(fn, src, pos, True), cb)

def complete(fn, src, pos):
	builtins = (gs.setting('autocomplete_builtins') is True or gs.setting('complete_builtins') is True)
	res, err = bcall('gocode_complete', _complete_opts(fn, src, pos, builtins))
	res = gs.dval(res.get('Candidates'), [])
	return res, err

def _complete_opts(fn, src, pos, builtins):
	nv = sh.env()
	return {
		'Dir': gs.basedir_or_cwd(fn),
		'Builtins': builtins,
		'Fn':  fn or '',
		'Src': src or '',
		'Pos': pos or 0,
		'Home': sh.vdir(),
		'Autoinst': gs.setting('autoinst'),
		'InstallSuffix': gs.setting('installsuffix', ''),
		'Env': {
			'GOROOT': nv.get('GOROOT', ''),
			'GOPATH': nv.get('GOPATH', ''),
		},
	}

def fmt(fn, src):
	st = gs.settings_dict()
	x = st.get('fmt_cmd')
	if x:
		res, err = bcall('sh', {
			'Env': sh.env(),
			'Cmd': {
					'Name': x[0],
					'Args': x[1:],
					'Input': src or '',
			},
		})
		return res.get('out', ''), (err or res.get('err', ''))

	res, err = bcall('fmt', {
		'Fn': fn or '',
		'Src': src or '',
		'TabIndent': st.get('fmt_tab_indent'),
		'TabWidth': st.get('fmt_tab_width'),
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
		'env': sh.env(),
		'InstallSuffix': gs.setting('installsuffix', ''),
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
		'env': sh.env(),
	}, cb)

def a_pkgpaths(exclude, f):
	tid = gs.begin(DOMAIN, '')
	def cb(res, err):
		gs.end(tid)
		f(res, err)

	m = sh.env()
	acall('pkgpaths', {
		'env': {
			'GOPATH': m.get('GOPATH'),
			'GOROOT': m.get('GOROOT'),
			'_pathsep': m.get('_pathsep'),
		},
		'exclude': exclude,
	}, cb)

def declarations(fn, src, pkg_dir, f):
	tid = gs.begin(DOMAIN, 'Fetching declarations')
	def cb(res, err):
		gs.end(tid)
		f(res, err)

	return acall('declarations', {
		'fn': fn or '',
		'src': src,
		'env': sh.env(),
		'pkgDir': pkg_dir,
	}, cb)

def imports(fn, src, toggle):
	return bcall('imports', {
		'autoinst': gs.setting('autoinst'),
		'env': sh.env(),
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
		'env': sh.env(),
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
	gs.mg9_send_q.put((method, arg, cb))

def bcall(method, arg):
	if _inst_state() != "done":
		return {}, 'Blocking call(%s) aborted: Install is not done' % method

	q = gs.queue.Queue()
	acall(method, arg, lambda r,e: q.put((r, e)))
	try:
		res, err = q.get(True, gs.setting('ipc_timeout', 1))
		return res, err
	except:
		return {}, 'Blocking Call(%s): Timeout' % method

def expand_jdata(v):
	if gs.is_a(v, {}):
		for k in v:
			v[k] = expand_jdata(v[k])
	elif gs.is_a(v, []):
		v = [expand_jdata(e) for e in v]
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
					tag = r.get('tag', '')
					k = REQUEST_PREFIX+token
					req = gs.attr(k, {})
					gs.del_attr(k)
					if req and req.f:
						if tag != TAG:
							gs.notice(DOMAIN, "\n".join([
								"GoSublime/MarGo appears to be out-of-sync.",
								"Maybe restart Sublime Text.",
								"Received tag `%s', expected tag `%s'. " % (tag, TAG),
							]))

						err = r.get('error', '')

						ev.debug(DOMAIN, "margo response: %s" % {
							'method': req.method,
							'tag': tag,
							'token': token,
							'dur': '%0.3fs' % (time.time() - req.tm),
							'err': err,
							'size': '%0.1fK' % (len(ln)/1024.0),
						})

						dat = expand_jdata(r.get('data', {}))
						try:
							keep = req.f(dat, err) is True
							if keep:
								req.tm = time.time()
								gs.set_attr(k, req)
						except Exception:
							gs.error_traceback(DOMAIN)
					else:
						ev.debug(DOMAIN, 'Ignoring margo: token: %s' % token)
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

					if _inst_state() != "busy":
						maybe_install()

					while _inst_state() == "busy":
						time.sleep(0.100)

					mg_bin = _margo_bin()
					cmd = [
						mg_bin,
						'-oom', gs.setting('margo_oom', 0),
						'-poll', 30,
						'-tag', TAG,
					]

					c = sh.Command(cmd)
					c.stderr = gs.LOGFILE
					c.env = {
						'GOGC': 10,
						'XDG_CONFIG_HOME': gs.home_path(),
					}

					pr = c.proc()
					if pr.ok:
						proc = pr.p
						err = ''
					else:
						proc = None
						err = 'Exception: %s' % pr.exc

					if err or not proc or proc.poll() is not None:
						killSrv()
						_call(cb, {}, 'Abort. Cannot start MarGo: %s' % err)

						continue

					gs.set_attr(PROC_ATTR_NAME, proc)
					gsq.launch(DOMAIN, lambda: _read_stdout(proc))

				req = Request(f=cb, method=method)
				gs.set_attr(REQUEST_PREFIX+req.token, req)

				header, err = gs.json_encode(req.header())
				if err:
					_cb_err(cb, 'Failed to construct ipc header: %s' % err)
					continue

				body, err = gs.json_encode(arg)
				if err:
					_cb_err(cb, 'Failed to construct ipc body: %s' % err)
					continue

				ev.debug(DOMAIN, 'margo request: %s ' % header)

				ln = '%s %s\n' % (header, body)

				try:
					if gs.PY3K:
						proc.stdin.write(bytes(ln, 'UTF-8'))
					else:
						proc.stdin.write(ln)

				except Exception as ex:
					_cb_err(cb, 'Cannot talk to MarGo: %s' % err)
					killSrv()
					gs.println(gs.traceback())

			except Exception:
				killSrv()
				gs.println(gs.traceback())
		except Exception:
			gs.println(gs.traceback())
			break

def _call(cb, res, err):
	try:
		cb(res, err)
	except Exception:
		gs.error_traceback(DOMAIN)

def _cb_err(cb, err):
	gs.error(DOMAIN, err)
	_call(cb, {}, err)


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

def on(token, cb):
	req = Request(f=cb, token=token)
	gs.set_attr(REQUEST_PREFIX+req.token, req)

def _dump(res, err):
	gs.println(json.dumps({
		'res': res,
		'err': err,
	}, sort_keys=True, indent=2))

if not gs.checked(DOMAIN, 'launch ipc threads'):
	gsq.launch(DOMAIN, _send)
	gsq.launch(DOMAIN, _recv)

def on_mg_msg(res, err):
	msg = res.get('message', '')
	if msg:
		print('GoSublime: MarGo: %s' % msg)
		gs.notify('MarGo', msg)

	return True

on('margo.message', on_mg_msg)
