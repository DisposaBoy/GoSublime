import sublime, sublime_plugin
import gscommon as gs, margo, gsq
import os, datetime

def do_post_save(view):
	if not gs.is_pkg_view(view):
		return

	domain = 'GoSublime-On-Save'
	for c in gs.setting('on_save', []):
		cmd = c.get('cmd', '')
		args = c.get('args', {})
		msg = 'running on_save command %s' % cmd
		tid = gs.begin(domain, msg, set_status=False)
		gs.println(msg)
		try:
			view.run_command(cmd, args)
		except Exception as ex:
			gs.notice(domain, 'Error %s' % ex)
		finally:
			gs.end(tid)

class EV(sublime_plugin.EventListener):
	def on_post_save(self, view):
		sublime.set_timeout(lambda: do_post_save(view), 0)

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
					name, _ = margo.package(os.path.join(basedir, fn), '')
					if name and name.get('Name'):
						pkg_name = name.get('Name')
						break
		except Exception:
			pass

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

		m = {}
		try:
			tasks = []

			for tid, t in gs.sm_tasks.items():
				tasks.append((tid, t))
			tasks.sort(key=lambda a: a[1]['start'], reverse=True)

			ents.insert(0, ['', '%d active task(s)' % len(tasks)])
			for a in tasks:
				tid, t = a
				delta = (now - t['start'])
				cancel_text = ''
				if t['cancel']:
					cancel_text = ' (cancel task)'
					m[len(ents)] = tid

				ents.append([
					'%s%s' % (t['domain'], cancel_text),
					t['message'],
					'started: %s' % t['start'],
					'elapsed: %s' % delta
				])
		except:
			ents = [['', 'Failed to gather active tasks']]

		def cb(i):
			t = gs.task(m.get(i, ''))
			if t and t['cancel']:
				s = 'are you sure you want to end task: %s: %s' % (t['domain'], t['message'])
				if sublime.ok_cancel_dialog(s):
					t['cancel']()

		self.window.show_quick_panel(ents, cb)
