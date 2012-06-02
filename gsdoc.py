import gscommon as gs, margo
import sublime, sublime_plugin

DOMAIN = 'GsDoc'

class GsDocCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		return gs.is_go_source_view(self.view)

	def run(self, _, mode=''):
		view = self.view
		if (not gs.is_go_source_view(view)) or (mode not in ['goto', 'hint']):
			return

		pt = view.sel()[0].begin()
		r = view.word(pt)
		r2 = None
		if r.begin() > 1 and view.substr(sublime.Region(r.begin()-1, r.begin())) == ".":
			r2 = view.word(r.begin()-2)
		if not r2 and view.substr(sublime.Region(r.end(), r.end()+1)) == ".":
			r2 = view.word(r.end()+2)

		if r2:
			r = sublime.Region(min(r.begin(), r2.begin()), max(r.end(), r2.end()))
			expr = view.substr(r)
			src = view.substr(sublime.Region(0, view.size()))
			docs, err = margo.doc(view.file_name(), src, expr)
			if err:
				gs.notice(DOMAIN, err)
			elif docs:
				if mode == "goto":
					fn = ''
					flags = 0
					for d in docs:
						fn = d.get('fn', '')
						row = d.get('row', 0)
						col = d.get('col', 0)
						if row > 0:
							flags = sublime.ENCODED_POSITION
							fn = '%s:%d:%d' % (fn, row+1, col+1)
					if fn:
						view.window().open_file(fn,	flags)
					else:
						gs.notice(DOMAIN, "cannot find definition for `%s'" % expr)
				elif mode == "hint":
					s = []
					for d in docs:
						s.append('%s\n\n%s' % (d.get('decl'), d.get('doc')))
					s = '\n\n\n\n'.join(s)

					panel = view.window().get_output_panel(DOMAIN)
					edit = panel.begin_edit()
					try:
						panel.set_read_only(False)
						panel.sel().clear()
						panel.replace(edit, sublime.Region(0, panel.size()), s)
						panel.set_read_only(True)
					finally:
						panel.end_edit(edit)
					view.window().run_command("show_panel", {"panel": "output.%s" % DOMAIN})
			else:
				gs.notice(DOMAIN, "no docs found for `%s'" % expr)
		else:
			gs.notice(DOMAIN, 'cannot find a valid name: currently supports only pkg.Func')

