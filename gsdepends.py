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

class GsDependsOnLoad(sublime_plugin.EventListener):
	def on_load(self, view):
		global dep_check_done
		sublime.set_timeout(gs.sync_settings, 0)
		if not dep_check_done and gs.is_go_source_view(view):
			dep_check_done = True
			margo.dispatch(check_depends, 'checking dependencies')

def split_changes(s):
	changes = []
	for m in CHANGES_SPLIT_PAT.split(s):
		m = CHANGES_MATCH_PAT.match(m)
		if m:
			changes.append((m.group(1), m.group(2)))
	changes.sort(reverse=True)
	return changes

def call_cmd(cmd):
	_, _, exc = gsshell.run(cmd)
	return not exc

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

def check_depends():
	changelog_fn = gs.dist_path("CHANGELOG.md")
	try:
		with open(changelog_fn) as f:
			s = f.read()
	except IOError:
		gs.notice(DOMAIN, traceback.format_exc())
		return

	changes = split_changes(s)
	if changes:
		def cb():
			aso = gs.aso()
			old_rev = aso.get('changelog.rev', '')
			new_rev = changes[0][0]
			if new_rev > old_rev:
				aso.set('changelog.rev', new_rev)
				gs.save_aso()

				new_changes = [
					'GoSublime: Recent Updates (you may need to restart Sublime Text for changes to take effect)',
					'------------------------------------------------------------------------------------------',
				]

				for change in changes:
					rev, msg = change
					if rev > old_rev:
						new_changes.append('\n%s\n\t%s' % (rev, msg))
					else:
						break

				new_changes.append('\nSee %s for the full CHANGELOG\n' % changelog_fn)
				new_changes = '\n'.join(new_changes)
				gs.show_output(DOMAIN, new_changes, print_output=False)
		sublime.set_timeout(cb, 0)
	else:
		margo.call(
			path='/',
			args='hello',
			message='hello MarGo'
		)

def dispatch(f, msg=''):
	gsq.dispatch(DOMAIN, f, msg)
