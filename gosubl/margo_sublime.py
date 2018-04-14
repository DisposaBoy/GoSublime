from . import _dbg
from . import gs
from .margo import mg
from .margo_render import render_src
from .margo_state import actions, ViewPathName
import os
import sublime
import sublime_plugin

class MargoEvents(sublime_plugin.EventListener):
	def on_query_completions(self, view, prefix, locations):
		return mg.event('query_completions', view, mg.on_query_completions, [view, prefix, locations])

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

class MargoRenderSrcCommand(sublime_plugin.TextCommand):
	def run(self, edit, src):
		render_src(self.view, edit, src)

class MargoUserCmdsCommand(sublime_plugin.TextCommand):
	def enabled(self):
		return mg.enabled(self.view)

	def run(self, edit):
		mg.send(view=self.view, actions=[actions.QueryUserCmds], cb=self._cb)

	def _cb(self, rs):
		win = self.view.window() or sublime.active_window()
		selected = 0
		flags = sublime.MONOSPACE_FONT
		items = []
		cmds = rs.state.user_cmds

		for c in cmds:
			desc = c.desc or '`%s`' % ' '.join([c.name] + c.args)
			items.append([c.title, desc])

		def on_done(i):
			if i < 0 or i >= len(cmds):
				return

			cmd = cmds[i]
			win.run_command('gs9o_win_open', {
				'run': [cmd.name] + cmd.args,
				'save_hist': False,
				'focus_view': False,
				'show_view': True,
			})

		def on_highlight(i):
			pass

		win.show_quick_panel(items or ['No User Commands'], on_done, flags, selected, on_highlight)

class MargoIssuesCommand(sublime_plugin.TextCommand):
	def run(self, edit, **action):
		if mg.enabled(self.view):
			self._run()
		else:
			self.view.run_command('gs_palette', {
				'palette': 'errors', 'direct': True,
			})

	def _run(self):
		mg.send(view=self.view, actions=[actions.QueryIssues], cb=self._cb)

	def _cb(self, rs):
		show_issues(self.view, rs.state.issues)

def issues_to_items(view, issues):
	vp = ViewPathName(view)
	dir = os.path.dirname(vp.path)
	index = []

	for isu in issues:
		if isu.message:
			index.append(isu)

	if not index:
		return ([], [], -1)

	def sort_key(isu):
		if vp.match(isu):
			return (-1, '', isu.row)

		return (1, isu.relpath(dir), isu.row)

	index.sort(key=sort_key)

	row, _ = gs.rowcol(view)
	items = []
	selected = []
	for idx, isu in enumerate(index):
		if vp.match(isu):
			title = 'Line %d' % (isu.row + 1)
			selected.append((abs(isu.row - row), idx))
		else:
			title = '%s:%d' % (isu.relpath(dir) or isu.name, isu.row + 1)
			selected.append((999999999, -1))

		rows = [title]
		rows.extend(s.strip() for s in isu.message.split('\n'))
		rows.append(' '.join(
			'[%s]' % s for s in filter(bool, (isu.tag, isu.label))
		))

		# hack: ST sometimes decide to truncate the message because it's longer
		# than the top row... and we don't want the message up there
		rows[0] = rows[0].ljust(max(len(s) for s in rows))
		items.append(rows)

	# hack: if the items don't have the same length, ST throws an exception
	n = max(len(l) for l in items)
	for l in items:
		l += [''] * (n - len(l))

	return (items, index, min(selected)[1])

def show_issues(view, issues):
	orig_row, orig_col = gs.rowcol(view)
	flags = sublime.MONOSPACE_FONT
	items, index, selected = issues_to_items(view, issues)

	def on_done(i):
		if not index or i >= len(index):
			return

		if i < 0:
			vp = ViewPathName(view)
			fn = vp.path or vp.name
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

