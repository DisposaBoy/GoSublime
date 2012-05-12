import subprocess, httplib, urllib, json, traceback, os
import sublime
import gscommon as gs

class Conn(object):
	def __init__(self):
		self.c = None

	def init(self):
		if self.c:
			try:
				self.c.close()
			except:
				pass
		self.c = httplib.HTTPConnection(gs.setting('margo_addr', ''))

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

def post(path, a, default, fail_early=False):
	resp = None
	try:
		params = urllib.urlencode({ 'data': json.dumps(a) })
		headers = {
			"Content-type": "application/x-www-form-urlencoded",
			"Accept": "application/json; charset=utf-8"
		}

		try:
			resp = conn.post(path, params, headers)
		except Exception:
			if fail_early:
				return (default, traceback.format_exc())

			margo_cmd = list(gs.setting('margo_cmd', []))
			if not margo_cmd:
				err = 'Missing `margo_cmd`'
				gs.notice("MarGo", err)
				return (default, err)
			margo_cmd.extend(["-d", "-addr", gs.setting('margo_addr', '')])
			out, err, _ = gs.runcmd(margo_cmd)

			out = out.strip()
			if out:
				print('MarGo: started: %s' % out)

			err = err.strip()
			if err:
				gs.notice('MarGo', err)
			else:
				resp = conn.post(path, params, headers)
	except:
		err = traceback.format_exc()
		gs.notice("MarGo", err)
		return (default, err)
	if not isinst(resp, {}):
		resp = {}
	if not isinst(resp.get("error"), u""):
		resp["error"] = "Invalid Response"
	if default is not None and not isinst(resp.get("data"), default):
		resp["data"] = default
		if not resp["error"]:
			resp["error"] = "Invalid Data"
	return (resp["data"], resp["error"])

def declarations(filename, src):
	return post('/declarations', {
		'fn': filename,
		'src': src
	}, [])

def fmt(filename, src):
	return post('/fmt', {
		'fn': filename,
		'src': src
	}, u"")

def hello(motd):
	return post('/', motd, {})

def bye_ni():
	return post('/', 'bye ni', {}, True)

def package(filename, src):
	return post('/package', {
		'fn': filename,
		'src': src
	}, {})

def imports(filename, src, import_paths, toggle):
	return post('/imports', {
		'fn': filename,
		'src': src,
		'env': gs.env(),
		'import_paths': import_paths,
		'toggle': toggle,
	}, {})
