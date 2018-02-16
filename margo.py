from .gosubl import gs
from .gosubl.margo import mg
from .gosubl.margo_render import render_src
import sublime_plugin

class MargoEvents(sublime_plugin.EventListener):
	def on_query_completions(self, view, prefix, locations):
		return mg.event('query_completions', view, mg.on_query_completions, [view, prefix, locations])

	def on_hover(self, view, point, hover_zone):
		return mg.event('hover', view, mg.on_hover, [view, point, hover_zone])

	def on_activated_async(self, view):
		return mg.event('activated', view, mg.on_activated, [view])

	def on_modified_async(self, view):
		return mg.event('modified', view, mg.on_modified, [view])

	def on_selection_modified_async(self, view):
		return mg.event('selection_modified', view, mg.on_selection_modified, [view])

	def on_pre_save(self, view):
		return mg.event('pre_save', view, mg.on_pre_save, [view])

	def on_post_save_async(self, view):
		return mg.event('post_save', view, mg.on_post_save, [view])

	def on_load_async(self, view):
		return mg.event('load', view, mg.on_load, [view])

	def on_close(self, view):
		return mg.event('close', view, mg.on_close, [view])

class MargoRenderSrcCommand(sublime_plugin.TextCommand):
	def run(self, edit, src):
		render_src(self.view, edit, src)

class MargoFmtCommand(sublime_plugin.TextCommand):
	def run(self, edit):
		if mg.enabled(self.view):
			mg.fmt(self.view)
		else:
			self.view.run_command('gs_fmt')

class MargoRestartAgentCommand(sublime_plugin.WindowCommand):
	def run(self):
		mg.restart()

class MargoOpenExtensionCommand(sublime_plugin.WindowCommand):
	def run(self):
		fn = mg.extension_file(True)
		if fn:
			gs.focus(fn, focus_pat='func Margo')

