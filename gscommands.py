import sublime, sublime_plugin

class GsCommentForwardCommand(sublime_plugin.TextCommand):
	def run(self, edit):
		if self.view.score_selector(self.view.sel()[0].begin(), 'source.go') > 0:
			self.view.run_command("toggle_comment", {"block": False})
			self.view.run_command("move", {"by": "lines", "forward": True})
		else:
			self.view.run_command("toggle_comment", {"block": False})
