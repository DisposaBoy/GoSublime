import sublime, sublime_plugin
import gscommon as gs, margo
import os, datetime

class GsCommentForwardCommand(sublime_plugin.TextCommand):
	def run(self, edit):
		self.view.run_command("toggle_comment", {"block": False})
		self.view.run_command("move", {"by": "lines", "forward": True})

class GsFmtSaveCommand(sublime_plugin.TextCommand):
	def run(self, edit):
		self.view.run_command("gs_fmt")
		sublime.set_timeout(lambda: self.view.run_command("save"), 0)

class GsFmtPromptSaveAsCommand(sublime_plugin.TextCommand):
	def run(self, edit):
		self.view.run_command("gs_fmt")
		sublime.set_timeout(lambda: self.view.run_command("prompt_save_as"), 0)

class GsGotoRowColCommand(sublime_plugin.TextCommand):
	def run(self, edit, row, col=0):
		pt = self.view.text_point(row, col)
		r = sublime.Region(pt, pt)
		self.view.sel().clear()
		self.view.sel().add(r)
		self.view.show(pt)
		dmn = 'gs.focus.%s:%s:%s' % (gs.view_fn(self.view), row, col)
		flags = sublime.DRAW_EMPTY_AS_OVERWRITE
		show = lambda: self.view.add_regions(dmn, [r], 'comment', 'bookmark', flags)
		hide = lambda: self.view.erase_regions(dmn)

		for i in range(3):
			m = 300
			s = i * m * 2
			h = s + m
			sublime.set_timeout(show, s)
			sublime.set_timeout(hide, h)

class GsNewGoFileCommand(sublime_plugin.WindowCommand):
	def run(self):
		default_file_name = 'untitled.go'
		pkg_name = 'main'
		view = gs.active_valid_go_view()
		basedir = gs.basedir_or_cwd(view and view.file_name())
		for fn in os.listdir(basedir):
			if fn.endswith('.go'):
				name, _ = margo.package(os.path.join(basedir, fn), '')
				if name and name.get('Name'):
					pkg_name = name.get('Name')
					break

		view = self.window.new_file()
		view.set_name(default_file_name)
		view.set_syntax_file('Packages/Go/Go.tmLanguage')
		edit = view.begin_edit()
		try:
			view.replace(edit, sublime.Region(0, view.size()), 'package %s\n' % pkg_name)
			view.sel().clear()
			view.sel().add(view.find(pkg_name, 0, sublime.LITERAL))
		finally:
			view.end_edit(edit)

class GsShowTasksCommand(sublime_plugin.WindowCommand):
	def run(self):
		ents = []
		now = datetime.datetime.now()
		with gs.sm_lck:
			tasks = gs.sm_tasks.values()

		try:
			for t in gs.sm_tasks.values():
				delta = (now - t['start'])
				ents.append([
					t['domain'],
					t['message'],
					'duration: %s' % delta,
				])
			ents.sort(key=lambda t: t[2], reverse=True)
		except:
			ents = [['', 'Failed to gather runnning tasks']]

		if len(ents) == 0:
			ents = [['', 'No task currently runnning']]

		self.window.show_quick_panel(ents, None)