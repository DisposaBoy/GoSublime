import gscommon as gs, margo
import sublime, sublime_plugin
import os

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

		doc = ''
		pt = view.sel()[0].begin()
		src = view.substr(sublime.Region(0, view.size()))
		docs, err = margo.doc(view.file_name(), src, pt)
		if err:
			self.show_output('// Error: %s' % err)
		elif docs:
			if mode == "goto":
				fn = ''
				flags = 0
				if len(docs) > 0:
					d = docs[0]
					fn = d.get('fn', '')
					row = d.get('row', 0)
					col = d.get('col', 0)
					if fn:
						gs.println('opening %s:%s:%s' % (fn, row, col))
						gs.focus(fn, row, col)
						return
				self.show_output("%s: cannot find definition" % DOMAIN)
			elif mode == "hint":
				s = []
				for d in docs:
					src = d.get('src', '').strip()
					if src:
						doc = '// %s %s\n//\n' % (d.get('kind', ''), d.get('name', ''))
						doc = '%s%s' % (doc, src)

					s.append(doc)
				doc = '\n\n\n'.join(s).strip()
		self.show_output(doc or "// %s: no docs found" % DOMAIN)

class GsBrowseDeclarationsCommand(sublime_plugin.WindowCommand):
	def is_enabled(self):
		return gs.is_go_source_view(self.window.active_view())

	def run(self, dir=''):
		win, view = gs.win_view(None, self.window)
		if view is None:
			return

		current = "Current Package"

		im, _ = margo.import_paths('', '')
		paths = im.get('paths', [])
		paths.sort()
		paths.insert(0, current)

		def cb(i):
			if i == 0:
				vfn = gs.view_fn(view)
				src = gs.view_src(view)
				pkg_dir = ''
				if view.file_name():
					pkg_dir = os.path.dirname(view.file_name())
				self.present(vfn, src, pkg_dir)
			elif i > 0:
				self.present('', '', paths[i])

		if paths:
			if dir == '.':
				cb(0)
			elif dir:
				self.present('', '', dir)
			else:
				win.show_quick_panel(paths, cb)
		else:
			win.show_quick_panel([['', 'No package paths found']], lambda x: None)


	def present(self, vfn, src, pkg_dir):
		win = self.window
		if win is None:
			return

		res, err = margo.declarations(vfn, src, pkg_dir)
		if err:
			gs.notice(DOMAIN, err)
			return

		decls = res.get('file_decls', [])
		for d in res.get('pkg_decls', []):
			if not vfn or d['fn'] != vfn:
				decls.append(d)

		for d in decls:
			d['ent'] = '%s %s' % (d['kind'], (d['repr'] or d['name']))

		ents = []
		decls.sort(key=lambda d: d['ent'])
		for d in decls:
			ents.append(d['ent'])

		def cb(i):
			if i >= 0:
				d = decls[i]
				gs.focus(d['fn'], d['row'], d['col'], win)

		if ents:
			win.show_quick_panel(ents, cb)
		else:
			win.show_quick_panel([['', 'No declarations found']], lambda x: None)


class GsBrowsePackagesCommand(sublime_plugin.WindowCommand):
	def run(self):
		win = self.window
		res, err = margo.pkgdirs()
		if err:
			gs.notice(DOMAIN, err)
			return

		m = {}
		for root, dirs in res.iteritems():
			for dir, fn in dirs.iteritems():
				if not m.get(dir):
					m[dir] = fn
		ents = sorted(m.keys())
		if ents:
			def cb(i):
				if i >= 0:
					fn = m[ents[i]]
					gs.focus(fn, 0, 0, win)
			win.show_quick_panel(ents, cb)
		else:
			win.show_quick_panel([['', 'No source directories found']], lambda x: None)


