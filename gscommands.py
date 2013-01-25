import datetime
import gscommon as gs
import mg9
import os
import sublime
import sublime_plugin

class GsCommentForwardCommand(sublime_plugin.TextCommand):
	def run(self, edit):
		self.view.run_command("toggle_comment", {"block": False})
		self.view.run_command("move", {"by": "lines", "forward": True})

class GsFmtSaveCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		return gs.is_go_source_view(self.view)

	def run(self, edit):
		self.view.run_command("gs_fmt")
		sublime.set_timeout(lambda: self.view.run_command("save"), 0)

class GsFmtPromptSaveAsCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		return gs.is_go_source_view(self.view)

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
		try:
			basedir = gs.basedir_or_cwd(view and view.file_name())
			for fn in os.listdir(basedir):
				if fn.endswith('.go'):
					name, _ = mg9.pkg_name(os.path.join(basedir, fn), '')
					if name:
						pkg_name = name
						break
		except Exception:
			gs.error_traceback('GsNewGoFile')

		view = self.window.new_file()
		view.set_name(default_file_name)
		view.set_syntax_file('Packages/GoSublime/GoSublime.tmLanguage')
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
		m = {}
		try:
			tasks = gs.task_list()
			ents.insert(0, ['', '%d active task(s)' % len(tasks)])
			for tid, t in tasks:
				cancel_text = ''
				if t['cancel']:
					cancel_text = ' (cancel task)'
					m[len(ents)] = tid

				ents.append([
					'#%s %s%s' % (tid, t['domain'], cancel_text),
					t['message'],
					'started: %s' % t['start'],
					'elapsed: %s' % (now - t['start']),
				])
		except:
			ents = [['', 'Failed to gather active tasks']]

		def cb(i):
			gs.cancel_task(m.get(i, ''))

		self.window.show_quick_panel(ents, cb)
