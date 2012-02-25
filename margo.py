import subprocess, socket, json, traceback
import gscommon as gs

def isinst(v, base):
	return isinstance(v, type(base))

def send(sck_addr, a):
	sck = socket.socket()
	sck.connect(sck_addr)
	sf = sck.makefile()
	json.dump(a, sf)
	sf.flush()
	return json.load(sf)

def do(a, default):
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
			resp = send(sck_addr, a)
		except:
			margo_cmd.extend(["-d", "-addr", margo_addr])
			gs.notice('MarGo', 'Attempting to start MarGo: `%s`' % ' '.join(margo_cmd))
			_, err = gs.runcmd(margo_cmd)
			if err:
				gs.notice('MarGo', err)
			resp = send(sck_addr, a)
	except:
		err = traceback.format_exc()
		gs.notice("MarGo", err)
		return (default, err)

	if not isinst(resp, {}):
		resp = {}
	if not isinst(resp.get("error"), u""):
		resp["error"] = "Invalid Response"
	if not isinst(resp.get("data"), default):
		resp["data"] = default
		if not resp["error"]:
			resp["error"] = "Invalid Data"
	return (resp["data"], resp["error"])

do({'call': 'hello'}, {}) # start MarGo
