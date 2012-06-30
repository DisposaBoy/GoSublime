import gscommon as gs, margo
import sublime, sublime_plugin

DOMAIN = 'GsDoc'

class GsDocCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		return gs.is_go_source_view(self.view)

	def show_output(self, s):
		gs.show_output(DOMAIN+'-output', s, False, 'GsDoc')

	def run(self, _, mode=''):
		view = self.view
		if (not gs.is_go_source_view(view)) or (mode not in ['goto', 'hint']):
			return

		pt = view.sel()[0].begin()
		src = view.substr(sublime.Region(0, view.size()))
		docs, err = margo.doc(view.file_name(), src, pt)
		if err:
			self.show_output('// Error: %s' % err)
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
					self.show_output("%s: cannot find definition" % DOMAIN)
			elif mode == "hint":
				s = []
				for d in docs:
					doc = '// %s %s\n// ...\n' % (d.get('kind', ''), d.get('name', ''))
					src = d.get('src', '').strip()
					if src:
						doc = '%s%s' % (doc, src)

					s.append(doc)
				s = '\n\n\n'.join(s)
				self.show_output(s)
		else:
			self.show_output("%s: no docs found" % DOMAIN)

