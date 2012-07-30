import subprocess, httplib, urllib, json, traceback, os
import sublime
import gscommon as gs, gsdepends, gsq

DOMAIN = 'MarGo'

class Conn(object):
	def __init__(self):
		self.c = None

	def init(self):
		if self.c:
			try:
				self.c.close()
			except:
				pass
		self.c = httplib.HTTPConnection(gs.setting('margo_addr', ''), timeout=5)

	def post(self, path, p, h):
		if not self.c:
			self.init()
		try:
			self.c.request('POST', path, p, h)
		except:
			self.init()
			self.c.request('POST', path, p, h)
		return json.loads(self.c.getresponse().read())

conn = Conn()

def isinst(v, base):
	return isinstance(v, type(base))

def post(path, a, default, fail_early=False, can_block=False):
	resp = None

	try:
		params = urllib.urlencode({ 'data': json.dumps(a) })
		headers = {
			"Content-type": "application/x-www-form-urlencoded",
			"Accept": "application/json; charset=utf-8"
		}
	except Exception as ex:
		return (default, ('MarGo: %s' % ex))

	try:
		resp = conn.post(path, params, headers)
	except Exception as ex:
		if can_block:
			gsdepends.do_hello()
			try:
				resp = conn.post(path, params, headers)
			except Exception as ex:
				return (default, ('MarGo: %s' % ex))
		else:
			# gsdepeds.hello calls us...
			if not fail_early:
				gsdepends.dispatch(gsdepends.hello)
			return (default, ('MarGo: %s' % ex))

	if not isinst(resp, {}):
		resp = {}
	if not isinst(resp.get("error"), u""):
		resp["error"] = "Invalid Response"
	if default is not None and not isinst(resp.get("data"), default):
		resp["data"] = default
		if not resp["error"]:
			resp["error"] = "Invalid Data"
	return (resp["data"], resp["error"])

def declarations(filename, src, pkg_dir=''):
	return post('/declarations', {
		'fn': filename or '',
		'src': src,
		'env': gs.env(),
		'pkg_dir': pkg_dir,
	}, {})

def fmt(filename, src):
	return post('/fmt', {
		'fn': filename or '',
		'src': src,
		'tab_indent': gs.setting('fmt_tab_indent'),
		'tab_width': gs.setting('fmt_tab_width'),
	}, u"")

def hello(motd):
	return post('/', motd, {})

def bye_ni():
	return post('/', 'bye ni', {}, True)

def package(filename, src):
	return post('/package', {
		'fn': filename or '',
		'src': src
	}, {})

def lint(filename, src):
	return post('/lint', {
		'fn': filename or '',
		'src': src
	}, [])

def imports(filename, src, toggle):
	return post('/imports', {
		'fn': filename or '',
		'src': src,
		'toggle': toggle,
		'tab_indent': gs.setting('fmt_tab_indent'),
		'tab_width': gs.setting('fmt_tab_width'),
	}, {})

def call(path='/', args={}, default={}, cb=None, message=''):
	try:
		if args is None:
			a = ''
		elif isinst(args, {}):
			a = {
				'env': gs.env(),
				'tab_indent': gs.setting('fmt_tab_indent'),
				'tab_width': gs.setting('fmt_tab_width'),
			}
			for k, v in args.iteritems():
				if v is None:
					v = ''
				a[k] = v
		else:
			a = args
	except:
		a = args

	def f():
		res, err = post(path, a, default, False, True)
		if cb:
			sublime.set_timeout(lambda: cb(res, err), 0)

	dispatch(f, 'call %s: %s' % (path, message))

def dispatch(f, msg):
	gsq.dispatch(DOMAIN, f, msg)

