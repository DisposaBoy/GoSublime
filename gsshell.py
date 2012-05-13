import sublime, sublime_plugin
import gscommon as gs
import re

GO_RUN_PAT = re.compile(r'^go\s+run$', re.IGNORECASE)

class Prompt(object):
	def __init__(self, view):
		self.view = view
		self.panel = None
		self.subcommands = [
			'go run', 'go build', 'go clean', 'go fix',
			'go install', 'go test', 'go fmt', 'go vet', 'go tool'
		]
		self.settings = sublime.load_settings('GoSublime-GsShell.sublime-settings')

	def on_done(self, s):
		s = s.strip()
		if s:
			self.settings.set('last_command', s)
			sublime.save_settings('GoSublime-GsShell.sublime-settings')

		if GO_RUN_PAT.match(s):
			s = 'go run *.go'

		self.view.window().run_command("exec", { 'kill': True })
		self.view.window().run_command("exec", {
			'shell': True,
			'env': gs.env(),
			'cmd': [s],
			'file_regex': '^(.+\.go):([0-9]+):(?:([0-9]+):)?\s*(.*)',
		})

	def on_change(self, s):
		if self.panel:
			size = self.view.size()
			if s.endswith('\t'):
				lc = self.settings.get('last_command', 'go ')
				s = s.strip()
				if s and s not in ('', 'go'):
					l = []
					for i in self.subcommands:
						if i.startswith(s):
							l.append(i)
					if len(l) == 1:
						s = '%s ' % l[0]
				elif lc:
					s = '%s ' % lc
				edit = self.panel.begin_edit()
				try:
					self.panel.replace(edit, sublime.Region(0, self.panel.size()), s)
				finally:
					self.panel.end_edit(edit)

class GsShellCommand(sublime_plugin.WindowCommand):
	def run(self):
		view = gs.active_valid_go_view(self.window)
		if not view:
			gs.notice("GsShell", "this not a source.go view")
			return

		p = Prompt(view)
		p.panel = self.window.show_input_panel("GsShell", "go ", p.on_done, p.on_change, None)
