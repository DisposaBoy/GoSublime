import sublime, sublime_plugin
import gscommon as gs

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
		self.view.sel().clear()
		self.view.sel().add(sublime.Region(pt))
		self.view.show(pt)

class GsNewGoFileCommand(sublime_plugin.WindowCommand):
	def run(self):
		def cb(s):
			view = self.window.new_file()
			view.set_name(s)
			view.set_syntax_file('Packages/Go/Go.tmLanguage')
		self.window.show_input_panel("Choose File Name", 'untitled.go', cb, None, None)
