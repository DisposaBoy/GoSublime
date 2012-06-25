import sublime, sublime_plugin
import gscommon as gs
import re, os, httplib

DOMAIN = "GsShell"
GO_RUN_PAT = re.compile(r'^go\s+(run|play)$', re.IGNORECASE)
GO_SHARE_PAT = re.compile(r'^go\s+share$', re.IGNORECASE)
GO_PLAY_PAT = re.compile(r'(\b)go\s+play(\b)', re.IGNORECASE)

class Prompt(object):
	def __init__(self, view, change_history=True):
		self.view = view
		self.panel = None
		self.subcommands = [
			'go run', 'go build', 'go clean', 'go fix',
			'go install', 'go test', 'go fmt', 'go vet', 'go tool',
			'go share', 'go play',
		]
		self.settings = sublime.load_settings('GoSublime-GsShell.sublime-settings')
		self.change_history = change_history

	def on_done(self, s):
		fn = self.view.file_name()
		win = self.view.window()
		if fn and win:
			basedir = os.path.dirname(fn)
			for v in win.views():
				vfn = v.file_name()
				if vfn and os.path.dirname(vfn) == basedir and vfn.endswith('.go'):
					v.run_command('gs_fmt_save')

		# above we do some saves - thus creating a race so push this back to the end of the queue
		def cb(s):
			file_name = self.view.file_name()
			s = GO_PLAY_PAT.sub(r'\1go run\2', s)
			s = s.strip()
			if s and self.change_history:
				self.settings.set('last_command', s)
				sublime.save_settings('GoSublime-GsShell.sublime-settings')

			if GO_SHARE_PAT.match(s):
				s = ''
				host = "play.golang.org"
				warning = 'Are you sure you want to share this file. It will be public on %s' % host
				if not sublime.ok_cancel_dialog(warning):
					return

				try:
					c = httplib.HTTPConnection(host)
					src = self.view.substr(sublime.Region(0, self.view.size()))
					c.request('POST', '/share', src, {'User-Agent': 'GoSublime'})
					s = 'http://%s/p/%s' % (host, c.getresponse().read())
				except Exception as ex:
					s = 'Error: %s' % ex

				self.show_output(s)
				return

			if GO_RUN_PAT.match(s):
				if not file_name:
					# todo: clean this up after the command runs
					f, err = gs.temp_file(suffix='.go', prefix=DOMAIN+'-play.', delete=False)
					if err:
						self.show_output(err)
						return
					else:
						try:
							src = self.view.substr(sublime.Region(0, self.view.size()))
							if isinstance(src, unicode):
								src = src.encode('utf-8')
							f.write(src)
							f.close()
						except Exception as ex:
							self.show_output('Error: %s' % ex)
							return
						file_name = f.name
				s = 'go run "%s"' % file_name
			else:
				gpat = ' *.go'
				if gpat in s:
					fns = []
					for fn in os.listdir(os.path.dirname(self.view.file_name())):
						if fn.endswith('.go') and fn[0] not in ('.', '_') and not fn.endswith('_test.go'):
							fns.append('"%s"' % fn)
					fns = ' '.join(fns)
					if fns:
						s = s.replace(gpat, ' '+fns)
			self.view.window().run_command("exec", { 'kill': True })
			self.view.window().run_command("exec", {
				'shell': True,
				'env': gs.env(),
				'cmd': [s],
				'file_regex': '^(.+\.go):([0-9]+):(?:([0-9]+):)?\s*(.*)',
			})

		sublime.set_timeout(lambda: cb(s), 0)

	def show_output(self, s):
		panel_name = DOMAIN+'-share'
		panel = self.view.window().get_output_panel(panel_name)
		edit = panel.begin_edit()
		try:
			panel.set_read_only(False)
			panel.sel().clear()
			panel.replace(edit, sublime.Region(0, panel.size()), s)
			panel.sel().add(sublime.Region(0, panel.size()))
			panel.set_read_only(True)
		finally:
			panel.end_edit(edit)
		print('%s output: %s' % (DOMAIN, s))
		self.view.window().run_command("show_panel", {"panel": "output.%s" % panel_name})

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
	def is_enabled(self):
		view = gs.active_valid_go_view(self.window)
		return bool(view)

	def run(self, prompt="go ", run=""):
		view = gs.active_valid_go_view(self.window)
		if not view:
			gs.notice(DOMAIN, "this not a source.go view")
			return

		run = run.strip()
		p = Prompt(view, run == "")
		if run:
			p.on_done(run)
		else:
			p.panel = self.window.show_input_panel("GsShell", prompt, p.on_done, p.on_change, None)
