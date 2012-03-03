import subprocess, socket, json, traceback, os
import sublime
import gsinit, gscommon as gs

def isinst(v, base):
	return isinstance(v, type(base))

def send(sck_addr, header, body):
	sck = socket.socket()
	sck.connect(sck_addr)
	sf = sck.makefile()
	json.dump(header, sf)
	sf.write("\r")
	json.dump(body, sf)
	sf.write("\n")
	sf.flush()
	return json.load(sf)

def call(method, a, default):
	h = {'method': method}
	resp = None
	try:
		if not gs.setting('margo_enabled', False):
			return (default, '')

		margo_cmd = list(gs.setting('margo_cmd', []))
		if not margo_cmd:
			err = 'Missing `margo_cmd`'
			gs.notice("MarGo", err)
			return (default, err)

		margo_addr = gs.setting('margo_addr')
		if not margo_addr:
			err = 'Missing `margo_addr`'
			gs.notice("MarGo", err)
			return (default, err)

		sck_addr = margo_addr.split(':')
		if len(sck_addr) != 2:
			err = 'Invalid `margo_addr`... must be in the format: `host:port`'
			gs.notice("MarGo", err)
			return (default, err)
		sck_addr = (sck_addr[0], int(sck_addr[1]))

		try:
			resp = send(sck_addr, h, a)
		except socket.error:
			margo_cmd.extend(["-d", "-addr", margo_addr])
			out, err = gs.runcmd(margo_cmd)
			
			out = out.strip()
			if out:
				print('MarGo: %s' % out)
			
			err = err.strip()
			if err:
				gs.notice('MarGo', err)
			else:
				resp = send(sck_addr, h, a)
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

def exit():
	call('exit', {}, None)

def hello(arg=None):
	return call('hello', arg, None)

def declarations(filename, src):
	return call('declarations', {
		'fn': filename,
		'src': src
	}, [])

def package_name(filename, src):
	return call('package_name', {
		'fn': filename,
		'src': src
	}, u'')

def imports(filename, src, import_paths, toggle):
	return call('imports', {
		'fn': filename,
		'src': src,
		'env': gsinit.env,
		'import_paths': import_paths,
		'toggle': toggle,
	}, {})
