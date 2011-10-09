import sublime, sublime_plugin

class GsCommentForwardCommand(sublime_plugin.TextCommand):
	def run(self, edit):
		self.view.run_command("toggle_comment", {"block": False})
		if self.view.score_selector(self.view.sel()[0].begin(), 'source.go') > 0:
			self.view.run_command("move", {"by": "lines", "forward": True})

class GsFmtSaveCommand(sublime_plugin.TextCommand):
	def run(self, edit):
		if self.view.score_selector(self.view.sel()[0].begin(), 'source.go') > 0:
			self.view.run_command("gs_fmt")
		sublime.set_timeout(lambda: self.view.run_command("save"), 0)

class GsFmtPromptSaveAsCommand(sublime_plugin.TextCommand):
	def run(self, edit):
		if self.view.score_selector(self.view.sel()[0].begin(), 'source.go') > 0:
			self.view.run_command("gs_fmt")
		sublime.set_timeout(lambda: self.view.run_command("prompt_save_as"), 0)
