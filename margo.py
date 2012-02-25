import subprocess, httplib, urllib, json, traceback
import gscommon as gs

def isinst(v, base):
	return isinstance(v, type(base))

def sendreq(margo_addr, path, a, h):
	conn = httplib.HTTPConnection(margo_addr)
	conn.request("POST", path, a, h)
	return conn

def request(path, args, default):
	if not gs.setting('margo_enabled', False):
		return (default, '')

	margo_cmd = list(gs.setting('margo_cmd', []))
	margo_addr = gs.setting('margo_addr')

	if not margo_cmd or not margo_addr:
		err = 'Missing `margo_cmd` or `margo_addr`'
		gs.notice("MarGo", err)
		return (default, err)

	resp = None
	try:
		args = urllib.urlencode(args)
		h = {'Content-type': 'application/x-www-form-urlencoded'}
		try:
			conn = sendreq(margo_addr, path, args, h)
		except:
			margo_cmd.extend(["-d", "-http", margo_addr])
			gs.notice('MarGo', 'Attempting to start MarGo: `%s`' % ' '.join(margo_cmd))
			_, err = gs.runcmd(margo_cmd)
			if err:
				gs.notice('MarGo', err)
			conn = sendreq(margo_addr, path, args, h)
		resp = json.load(conn.getresponse())
	except:
		err = traceback.format_exc()
		gs.notice("MarGo", err)
		return (default, err)

	if not isinst(resp, {}):
		resp = {}
	if not isinst(resp.get("error"), ""):
		resp["error"] = "Invalid Response"
	if not isinst(resp.get("data"), default):
		resp["data"] = default
		if not resp["error"]:
			resp["error"] = "Invalid Data"
	return (resp["data"], resp["error"])

