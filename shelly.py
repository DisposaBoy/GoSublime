import sublime, sublime_plugin
import gscommon as gs, margo, gsshell
import datetime, os, time

DOMAIN = "Shelly"
AC_OPTS = sublime.INHIBIT_WORD_COMPLETIONS | sublime.INHIBIT_EXPLICIT_COMPLETIONS

def is_enabled(view):
	return True

lwd = ''

try:
	_shelly_views
except Exception:
	# cache the views so they don't get destroyed
	_shelly_views = {}

def shelly_view(win):
	v = None
	if win is not None:
		wid = win.id()
		v = _shelly_views.get(wid, None)
		if v is None:
			v = win.get_output_panel("shelly")
			_shelly_views[wid] = v
			vs = v.settings()
			vs.set('shelly.view', True)
			vs.set("highlight_line", True)
			vs.set("gutter", False)
			vs.set("word_wrap", False)
			vs.set("detect_indentation", False)
			vs.set("draw_indent_guides", True)
			vs.set("indent_guide_options", ["draw_normal"])
			vs.set("tab_size", 1)
			vs.set("scroll_past_end", False)
			vs.set("rulers", [])
			v.set_read_only(False)
			v.set_syntax_file('Packages/GoSublime/shelly/shelly-output.tmLanguage')
	return v

class EV(sublime_plugin.EventListener):
	def on_query_completions(self, view, prefix, locations):
		pos = locations[0]
		if view.score_selector(pos, 'text.shelly-prompt') == 0:
			return []

		cl = []

		subcommands = [
			'go run', 'go build', 'go clean', 'go fix',
			'go install', 'go test', 'go fmt', 'go vet', 'go tool',
			'#share', '#play', '#clear'
		]

		for s in subcommands:
			if s.startswith('#'):
				cl.append((s, s))
			else:
				cl.append((s, s+' '))

		for s in hist:
			s = s.strip()
			if s and (s, s+' ') not in cl:
				cl.append((s, s))

		return (cl, AC_OPTS)

class ShellyViewCommand(gsshell.ViewCommand):
	def __init__(self, cmd=[], shell=False, env={}, cwd='', view=None):
		super(ShellyViewCommand, self).__init__(cmd=cmd, shell=shell, env=env, cwd=cwd, view=view)

	def run(self):
		if not self.cmd:
			return

		super(ShellyViewCommand, self).on_output(self, ('\n[run `%s`]' % self.cmd))
		super(ShellyViewCommand, self).run()

	def on_output(self, c, line):
		super(ShellyViewCommand, self).on_output(c, '\t'+line)

def settings():
	return sublime.load_settings('GoSublime-GsShell.sublime-settings')

class Prompt(object):
	def __init__(self, win, wd):
		self.window = win
		self.wd = wd
		self.view = win.show_input_panel('[ %s ] $' % wd, '', self.on_done, self.on_change, self.on_cancel)
		self.view.set_syntax_file('Packages/GoSublime/shelly/shelly-prompt.tmLanguage')
		vs = self.view.settings()
		vs.set("gutter", False)
		vs.set("rulers", [])
		vs.set("word_separators", "\\ .()$/=")
		vs.set("scroll_past_end", False)
		vs.set("line_padding_top", 1)
		vs.set("line_padding_bottom", 1)

	def on_done(self, s):
		v = shelly_view(self.window)
		if v is None:
			return

		s = s.strip()
		if s:
			c = ShellyViewCommand(cmd=s, shell=True, cwd=self.wd, view=v)
			c.start()

			try:
				hist = settings().get('hist', [])
				try:
					hist.remove(s)
				except Exception:
					pass
				hist.append(s)
				settings().set('hist', hist)
			except Exception as ex:
				gs.notice(DOMAIN, 'Error: %s' % ex)
		self.window.run_command('show_panel', {'panel': 'output.shelly'})

	def on_change(self, s):
		if hasattr(self, 'view') and self.view is not None and s and not s[-1].isspace():
			self.view.run_command('auto_complete', {
				"disable_auto_insert": True,
				"api_completions_only": True,
				"next_completion_if_showing": False
			})

	def on_cancel(self):
		pass

class ShellyHistPrevCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		return self.view.score_selector(self.view.sel()[0].begin(), 'text.shelly-prompt') > 0

	def run(self, edit):
		hist = settings().get('hist', [])
		try:
			i = self.view.settings().get('shelly.hist.index', len(hist)) - 1
			s = hist[i]
			self.view.replace(edit, sublime.Region(0, self.view.size()), s)
		except Exception:
			i = len(hist)-1
		self.view.settings().set('shelly.hist.index', i)

class ShellyHistNextCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		return self.view.score_selector(self.view.sel()[0].begin(), 'text.shelly-prompt') > 0

	def run(self, edit):
		hist = settings().get('hist', [])
		try:
			i = self.view.settings().get('shelly.hist.index', -1) + 1
			s = hist[i]
			self.view.replace(edit, sublime.Region(0, self.view.size()), s)
		except Exception:
			i = -1
		self.view.settings().set('shelly.hist.index', i)

class ShellyPromptCommand(sublime_plugin.WindowCommand):
	def run(self, _=None):
		av = self.window.active_view()
		wd = gs.basedir_or_cwd(av.file_name() if av is not None else '')
		Prompt(self.window, wd)

