import gscommon as gs, gsshell, margo, gsq
import threading, traceback, os, re
import sublime

DOMAIN = 'GsDepends'
CHANGES_SPLIT_PAT = re.compile(r'^##', re.MULTILINE)
CHANGES_MATCH_PAT = re.compile(r'^\s*(r[\d.]+[-]\d+)\s+(.+?)\s*$', re.DOTALL)

def split_changes(s):
	changes = []
	for m in CHANGES_SPLIT_PAT.split(s):
		m = CHANGES_MATCH_PAT.match(m)
		if m:
			changes.append((m.group(1), m.group(2)))
	changes.sort()
	return changes

def hello():
	margo.hello("hello world")
	gs.runcmd(['gocode'])

def check_depends():
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
			gocode_repo = 'github.com/nsf/gocode'
			margo_repo = 'github.com/DisposaBoy/MarGo'
			settings_fn = 'GoSublime-GsDepends.sublime-settings'
			settings = sublime.load_settings(settings_fn)
			new_rev = changes[0][0]
			old_rev = settings.get('tracking_rev', '')

			def on_panel_close(i):
				if i == 1 or i == 2:
					view = win.open_file(changelog_fn)
					if i == 1:
						prompt = gsshell.Prompt(view)
						prompt.on_done('go get -u -v %s %s' % (gocode_repo, margo_repo))
						margo.bye_ni()
						gs.runcmd('gocode close')
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
						"go get -u %s" % gocode_repo,
						"go get -u %s" % margo_repo,
					],
					[
						"View changelog",
						"Packages/GoSublime/CHANGELOG.md"
						" ",
					]
				]
				win.show_quick_panel(items, on_panel_close)
	else:
		gsq.dispatch(hello)

sublime.set_timeout(check_depends, 1000)
