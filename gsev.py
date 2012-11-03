import sublime
import sublime_plugin
import gscommands
import gscommon as gs

DOMAIN = 'GsEV'

class EV(sublime_plugin.EventListener):
	def on_post_save(self, view):
		sublime.set_timeout(lambda: do_post_save(view), 0)

	def on_activated(self, view):
		sublime.set_timeout(lambda: do_sync_active_view(view), 0)

def do_post_save(view):
	if not gs.is_pkg_view(view):
		return

	for c in gs.setting('on_save', []):
		cmd = c.get('cmd', '')
		args = c.get('args', {})
		msg = 'running on_save command %s' % cmd
		tid = gs.begin(DOMAIN, msg, set_status=False)
		gs.println(msg)
		try:
			view.run_command(cmd, args)
		except Exception as ex:
			gs.notice(DOMAIN, 'Error %s' % ex)
		finally:
			gs.end(tid)

def do_sync_active_view(view):
	if not gs.is_pkg_view(view):
		return

	gs.set_attr('last_active_fn', view.file_name())
