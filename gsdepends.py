import gscommon as gs, margo, gsq
import threading, traceback, os, re
import sublime, sublime_plugin

DOMAIN = 'GsDepends'
CHANGES_SPLIT_PAT = re.compile(r'^##', re.MULTILINE)
CHANGES_MATCH_PAT = re.compile(r'^\s*(r[\d.]+[-]\d+)\s+(.+?)\s*$', re.DOTALL)
GOCODE_REPO = 'github.com/nsf/gocode'
MARGO_REPO = 'github.com/DisposaBoy/MarGo'

dep_check_done = False

class GsDependsOnActivated(sublime_plugin.EventListener):
	def on_activated(self, view):
		if not dep_check_done:
			sublime.set_timeout(lambda: check_depends(view), 0)

class GsUpdateDeps(sublime_plugin.TextCommand):
	def run(self, edit):
		run_go_get(self.view)

def split_changes(s):
	changes = []
	for m in CHANGES_SPLIT_PAT.split(s):
		m = CHANGES_MATCH_PAT.match(m)
		if m:
			changes.append((m.group(1), m.group(2)))
	changes.sort()
	return changes

def call_cmd(cmd):
	_, _, exc = gs.runcmd(cmd)
	return not exc

hello_sarting = False
def hello():
	def cb():
		global hello_sarting
		if hello_sarting:
			return
		hello_sarting = True
		print 'starting gocode'
		call_cmd(['gocode'])
		margo_cmd = list(gs.setting('margo_cmd', []))
		if not margo_cmd:
			err = 'Missing `margo_cmd`'
			gs.notice("MarGo", err)
			hello_sarting = False
			return

		margo_cmd.extend([
			"-d",
			"-call", "replace",
			"-addr", gs.setting('margo_addr', '')
		])
		print 'starting margo'
		out, err, _ = gs.runcmd(margo_cmd)
		out = out.strip()
		err = err.strip()
		if err:
			gs.notice(DOMAIN, err)
		elif out:
			gs.notice(DOMAIN, 'MarGo started %s' % out)
		hello_sarting = False

	_, err = margo.post('/', 'hello', {}, True)
	if err:
		dispatch(cb, 'Starting MarGo and gocode...')
	else:
		call_cmd(['gocode'])

def run_go_get(view):
	msg = 'Installing/updating gocode and MarGo...'
	def f():
		out, err, _ = gs.runcmd(['go', 'get', '-u', '-v', GOCODE_REPO, MARGO_REPO])
		margo.bye_ni()
		call_cmd(['gocode', 'close'])
		gs.notice(DOMAIN, '%s done\n%s%s' % (msg, out, err))
	dispatch(f, msg, view)

def check_depends(view):
	global dep_check_done
	if dep_check_done:
		return

	if not view or not view.window():
		sublime.set_timeout(lambda: check_depends(view), 1000)
		return

	if not gs.is_go_source_view(view):
		return

	dep_check_done = True

	e = gs.env()
	if not (e.get('GOROOT') and e.get('GOPATH')):
		gs.notice(DOMAIN, "GOPATH and/or GOROOT appear to be unset")

	if not call_cmd(['go', '--help']):
		gs.notice(DOMAIN, 'The `go` command cannot be found')
		return

	missing = []
	cmds = [
		['gocode', '--help'],
		['MarGo', '--help'],
	]
	for cmd in cmds:
		if not call_cmd(cmd):
			missing.append(cmd[0])

	if missing:
		def cb(i):
			if i == 0:
				run_go_get(view)
		items = [[
			'GoSublime depends on gocode and MarGo',
			'Install %s (using `go get`)' % ', '.join(missing),
			'gocode repo: %s' % GOCODE_REPO,
			'MarGo repo: %s' % MARGO_REPO,
		]]
		view.window().show_quick_panel(items, cb)
		return

	changelog_fn = os.path.join(sublime.packages_path(), 'GoSublime', "CHANGELOG.md")
	try:
		with open(changelog_fn) as f:
			s = f.read()
	except IOError:
		gs.notice(DOMAIN, traceback.format_exc())
		return

	changes = split_changes(s)
	if changes:
		win = sublime.active_window()
		if win:
			settings_fn = 'GoSublime-GsDepends.sublime-settings'
			settings = sublime.load_settings(settings_fn)
			new_rev = changes[-1][0]
			old_rev = settings.get('tracking_rev', '')

			def on_panel_close(i):
				if i == 1 or i == 2:
					view = win.open_file(changelog_fn)
					if i == 1:
						run_go_get(view)
						settings.set('tracking_rev', new_rev)
						sublime.save_settings(settings_fn)


			if new_rev > old_rev:
				items = [
					[
						" ",
						"GoSublime updated to %s" % new_rev,
						" ",
					],
					[
						"Install/Update dependencies: Gocode, MarGo",
						"go get -u %s" % GOCODE_REPO,
						"go get -u %s" % MARGO_REPO,
					],
					[
						"View changelog",
						"Packages/GoSublime/CHANGELOG.md"
						" ",
					]
				]
				win.show_quick_panel(items, on_panel_close)
				return
	dispatch(hello)



Q = None

def dispatch(f, msg='', view=None, p=0):
	global Q
	if not Q:
		Q = gsq.GsQ(DOMAIN)
		Q.start()

	def cb(v):
		if v is None:
			win = sublime.active_window()
			if win:
				v = win.active_view()
		Q.dispatch(f, msg, v, v is not None, p)
	sublime.set_timeout(lambda: cb(view), 0)