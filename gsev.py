from gosubl import gs
from . import gstest
import sublime
import sublime_plugin

DOMAIN = 'GsEV'

class EV(sublime_plugin.EventListener):
	def on_pre_save(self, view):
		view.run_command('gs_fmt')
		sublime.set_timeout(lambda: do_set_gohtml_syntax(view), 0)

	def on_post_save(self, view):
		sublime.set_timeout(lambda: do_post_save(view), 0)

	def on_activated(self, view):
		win = view.window()
		if win is not None:
			active_view = win.active_view()
			if active_view is not None:
				sublime.set_timeout(lambda: do_sync_active_view(active_view), 0)

		sublime.set_timeout(lambda: do_set_gohtml_syntax(view), 0)

	def on_load(self, view):
		sublime.set_timeout(lambda: do_set_gohtml_syntax(view), 0)

class GsOnLeftClick(sublime_plugin.TextCommand):
	def run(self, edit):
		view = self.view
		if gs.is_go_source_view(view):
			if not gstest.handle_action(view, 'left-click'):
				view.run_command('gs_doc', {"mode": "goto"})
		elif view.score_selector(gs.sel(view).begin(), "text.9o") > 0:
			view.window().run_command("gs9o_open_selection")

class GsOnRightClick(sublime_plugin.TextCommand):
	def run(self, edit):
		view = self.view
		if gs.is_go_source_view(view):
			if not gstest.handle_action(view, 'right-click'):
				view.run_command('gs_doc', {"mode": "hint"})

def do_post_save(view):
	if not gs.is_pkg_view(view):
		return

	for c in gs.setting('on_save', []):
		cmd = c.get('cmd', '')
		args = c.get('args', {})
		msg = 'running on_save command %s' % cmd
		tid = gs.begin(DOMAIN, msg, set_status=False)
		try:
			view.run_command(cmd, args)
		except Exception as ex:
			gs.notice(DOMAIN, 'Error %s' % ex)
		finally:
			gs.end(tid)

def do_sync_active_view(view):
	fn = view.file_name() or ''
	gs.set_attr('active_fn', fn)

	if fn:
		gs.set_attr('last_active_fn', fn)
		if fn.lower().endswith('.go'):
			gs.set_attr('last_active_go_fn', fn)

	win = view.window()
	if win is not None and view in win.views():
		m = {}
		psettings = view.settings().get('GoSublime')
		if psettings and gs.is_a(psettings, {}):
			m = gs.mirror_settings(psettings)
		gs.set_attr('last_active_project_settings', gs.dval(m, {}))

		gs.sync_settings()

def do_set_gohtml_syntax(view):
	fn = view.file_name()
	xl = gs.setting('gohtml_extensions', [])
	if xl and fn and fn.lower().endswith(tuple(xl)):
		view.set_syntax_file(gs.tm_path('gohtml'))

