import sublime
import sublime_plugin
import gscommon as gs
import gsshell
import uuid

DOMAIN = "GsCommander"
AC_OPTS = sublime.INHIBIT_WORD_COMPLETIONS | sublime.INHIBIT_EXPLICIT_COMPLETIONS

try:
	stash
except:
	stash = {}

class EV(sublime_plugin.EventListener):
	def on_query_completions(self, view, prefix, locations):
			return []

class GsCommanderInitCommand(sublime_plugin.TextCommand):
	def run(self, edit, wd=None):
		v = self.view

		n = v.size()
		if n > 0 and v.substr(sublime.Region(n, n)) != '\n':
			v.insert(edit, n, '\n# ')
		else:
			v.insert(edit, n, '# ')

		if not wd:
			win = self.view.window()
			if win is not None:
				av = win.active_view()
				if av is not None:
					wd = gs.basedir_or_cwd(av.file_name())

		v.sel().clear()
		n = v.size()
		v.sel().add(sublime.Region(n, n))
		vs = v.settings()
		vs.set("gscommander.wd", wd)
		vs.set("rulers", [])
		vs.set("fold_buttons", True)
		vs.set("fade_fold_buttons", False)
		vs.set("gutter", False)
		vs.set("word_wrap", True)
		vs.set("indent_subsequent_lines", True)
		vs.set("line_numbers", False)
		v.set_syntax_file('Packages/GoSublime/GsCommander.tmLanguage')
		v.show(v.size()-1)


class GsCommanderOpenCommand(sublime_plugin.WindowCommand):
	def run(self):
		win = self.window
		wid = win.id()
		v = stash.get(wid)
		if v is None:
			v = win.get_output_panel('gscommander')
			stash[wid] = v

		win.run_command("show_panel", {"panel": "output.gscommander"})
		win.focus_view(v)
		v.run_command('gs_commander_init')

class GsCommanderExecCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		v = self.view
		return v is not None and v.score_selector(v.sel()[0].begin(), 'text.gscommander') > 0

	def run(self, edit):
		v = self.view
		pos = v.sel()[0].begin()
		line = v.line(pos)
		cmd = v.substr(line).lstrip()
		if cmd.startswith('#'):
			cmd = cmd.lstrip('#')
			out, err, _ = gsshell.run(cmd, shell=True)
			wd = v.settings().get('gscommander.wd')
			c = gsshell.ViewCommand(cmd=cmd, shell=True, view=v, cwd=wd)
			def on_done(c):
				def cb():
					win = sublime.active_window()
					if win is not None:
						win.run_command("gs_commander_open")
				sublime.set_timeout(cb, 0)

			c.done.append(on_done)
			c.start()
