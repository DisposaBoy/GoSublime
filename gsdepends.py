import gscommon as gs, margo, gsq
import threading, traceback, os, re
import sublime, sublime_plugin

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

class GsUpdateDeps(sublime_plugin.TextCommand):
	def run(self, edit):
		run_go_get()

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

def do_hello():
	global hello_sarting
	if hello_sarting:
		return
	hello_sarting = True

	tid = gs.begin(DOMAIN, 'Starting Gocode', False)
	call_cmd(['gocode'])
	gs.end(tid)

	margo_cmd = list(gs.setting('margo_cmd', []))
	if margo_cmd:
		margo_cmd.extend([
			"-d",
			"-call", "replace",
			"-addr", gs.setting('margo_addr', '')
		])

		tid = gs.begin(DOMAIN, 'Starting MarGo', False)
		out, err, _ = gs.runcmd(margo_cmd)
		gs.end(tid)

		out = out.strip()
		err = err.strip()
		if err:
			gs.notice(DOMAIN, err)
		elif out:
			gs.notice(DOMAIN, 'MarGo started %s' % out)
		hello_sarting = False
	else:
		err = 'Missing `margo_cmd`'
		gs.notice("MarGo", err)
		hello_sarting = False

hello_sarting = False
def hello():
	_, err = margo.post('/', 'hello', {}, True, False)
	if err:
		dispatch(do_hello, 'Starting MarGo and gocode...')
	else:
		call_cmd(['gocode'])

def run_go_get():
	msg = 'Installing/updating gocode and MarGo...'
	def f():
		out, err, _ = gs.runcmd(['go', 'get', '-u', '-v', GOCODE_REPO, MARGO_REPO])
		margo.bye_ni()
		call_cmd(['gocode', 'close'])
		gs.notice(DOMAIN, '%s done' % msg)
		gs.println(DOMAIN, '%s done\n%s%s' % (msg, out, err))
		do_hello()
	dispatch(f, msg)

def check_depends():
	gr = gs.go_env_goroot()
	if not gr:
		gs.notice(DOMAIN, 'The `go` command cannot be found')
		return

	e = gs.env()
	if not e.get('GOROOT'):
		os.environ['GOROOT'] = gr
	elif not e.get('GOPATH'):
		gs.notice(DOMAIN, "GOPATH and/or GOROOT appear to be unset")

	gs.println(
		'GoSublime: checking dependencies',
		('\tGOROOT is: %s' % e.get('GOROOT', gr)),
		('\tGOPATH is: %s' % e.get('GOPATH', ''))
	)

	missing = []
	for cmd in ('gocode', 'MarGo'):
		if not call_cmd([cmd, '--help']):
			missing.append(cmd)

	if missing:
		def cb(i, _):
			if i == 0:
				run_go_get()

		items = [[
			'GoSublime depends on gocode and MarGo',
			'Install %s (using `go get`)' % ', '.join(missing),
			'gocode repo: %s' % GOCODE_REPO,
			'MarGo repo: %s' % MARGO_REPO,
		]]

		gs.show_quick_panel(items, cb)
		gs.println(DOMAIN, '\n'.join(items[0]))
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
		def cb():
			settings_fn = 'GoSublime-GsDepends.sublime-settings'
			settings = sublime.load_settings(settings_fn)
			new_rev = changes[-1][0]
			old_rev = settings.get('tracking_rev', '')

			def on_panel_close(i, win):
				if i > 0:
					settings.set('tracking_rev', new_rev)
					sublime.save_settings(settings_fn)
					win.open_file(changelog_fn)
					if i == 1:
						run_go_get()

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
				gs.show_quick_panel(items, on_panel_close)
		sublime.set_timeout(cb, 0)
	else:
		margo.call(
			path='/',
			args='hello',
			message='hello MarGo'
		)



def dispatch(f, msg=''):
	gsq.dispatch(DOMAIN, f, msg)
