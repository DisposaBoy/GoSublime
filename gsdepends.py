import gscommon as gs
import margo
import gsq
import threading
import traceback
import os
import re
import sublime
import sublime_plugin
import mg9
import gsshell

DOMAIN = 'GsDepends'
CHANGES_SPLIT_PAT = re.compile(r'^##', re.MULTILINE)
CHANGES_MATCH_PAT = re.compile(r'^\s*(r[\d.]+[-]\d+)\s+(.+?)\s*$', re.DOTALL)
GOCODE_REPO = 'github.com/nsf/gocode'
MARGO_REPO = 'github.com/DisposaBoy/MarGo'

dep_check_done = False

def do_hello():
	global hello_sarting
	if hello_sarting:
		return
	hello_sarting = True

	margo_cmd = [
		mg9.MARGO0_BIN,
		"-d",
		"-call", "replace",
		"-addr", gs.setting('margo_addr', '')
	]

	tid = gs.begin(DOMAIN, 'Starting MarGo', False)
	out, err, _ = gsshell.run(margo_cmd)
	gs.end(tid)

	out = out.strip()
	err = err.strip()
	if err:
		gs.notice(DOMAIN, err)
	elif out:
		gs.println(DOMAIN, 'MarGo started %s' % out)
	hello_sarting = False

hello_sarting = False
def hello():
	_, err = margo.post('/', 'hello', {}, True, False)
	if err:
		dispatch(do_hello, 'Starting MarGo and gocode...')

def dispatch(f, msg=''):
	gsq.dispatch(DOMAIN, f, msg)
