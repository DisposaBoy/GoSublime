from . import _dbg
from . import gs
from .margo import mg
from .margo_render import render_src
from .margo_state import actions, view_path, view_name
import os
import sublime
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

class MargoIssuesCommand(sublime_plugin.TextCommand):
	def run(self, edit, **action):
		if mg.enabled(self.view):
			self._run()
		else:
			self.view.run_command('gs_palette', {
				'palette': 'errors', 'direct': True,
			})

	def _run(self):
		mg.send(view=self.view, action=actions.QueryIssues, cb=self._cb)

	def _cb(self, rs):
		show_issues(self.view, rs.state.issues)

def issues_to_items(view, issues):
	path = view_path(view)
	dir = os.path.dirname(path)
	name = view_name(view)
	index = []

	def in_view(isu):
		return isu.path == path or isu.name == name or (not isu.path and not isu.name)

	for isu in issues:
		if isu.message:
			index.append(isu)

	if not index:
		return ([], [], -1)

	def sort_key(isu):
		if in_view(isu):
			return (-1, '', isu.row)

		return (1, isu.relpath(dir), isu.row)

	index.sort(key=sort_key)

	row, _ = gs.rowcol(view)
	items = []
	selected = []
	for idx, isu in enumerate(index):
		if in_view(isu):
			title = 'Line %d' % (isu.row + 1)
			selected.append((abs(isu.row - row), idx))
		else:
			title = '%s:%d' % (isu.relpath(dir) or isu.name, isu.row + 1)
			selected.append((999999999, -1))

		message = '  %s%s' % (isu.message, ' [' + isu.label + ']' if isu.label else '')
		items.append([title, message])

	return (items, index, min(selected)[1])

def show_issues(view, issues):
	orig_row, orig_col = gs.rowcol(view)
	flags = sublime.MONOSPACE_FONT
	items, index, selected = issues_to_items(view, issues)

	def on_done(i):
		if i < 0 or i >= len(items):
			fn = view_path(view) or view_name(view)
			gs.focus(fn, row=orig_row, col=orig_col, win=view.window())
			return

		isu = index[i]
		gs.focus(isu.path or isu.name, row=isu.row, col=isu.col, win=view.window())

	def on_highlight(i):
		on_done(i)

	view.window().show_quick_panel(items or ['No Issues'], on_done, flags, selected, on_highlight)

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

