import subprocess, httplib, urllib, json, traceback
import gscommon as gs

def request(path, a={}):
	try:
		a = urllib.urlencode(a)
		h = {'Content-type': 'application/x-www-form-urlencoded'}
		conn = httplib.HTTPConnection(gs.setting('margo_addr'))
		conn.request("POST", path, a, h)
		resp = json.load(conn.getresponse())
		return resp
	except:
		gs.notice("Margo", traceback.format_exc())
	return {}
