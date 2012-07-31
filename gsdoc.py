import gscommon as gs, margo
import sublime, sublime_plugin
import os, re

DOMAIN = 'GsDoc'

GOOS_PAT = re.compile(r'_(%s)' % '|'.join(gs.GOOSES))
GOARCH_PAT = re.compile(r'_(%s)' % '|'.join(gs.GOARCHES))

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
		def f(docs, err):
			doc = ''
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
						name = d.get('name', '')
						if name:
							kind = d.get('kind', '')
							pkg = d.get('pkg', '')
							if pkg:
								name = '%s.%s' % (pkg, name)
							src = d.get('src', '')
							if src:
								src = '\n//\n%s' % src
							doc = '// %s %s%s' % (name, kind, src)

						s.append(doc)
					doc = '\n\n\n'.join(s).strip()
			self.show_output(doc or "// %s: no docs found" % DOMAIN)

		margo.call(
			path='/doc',
			args={
				'fn': view.file_name(),
				'src': src,
				'offset': pt,
			},
			default=[],
			cb=f,
			message='fetching docs'
		)

class GsBrowseDeclarationsCommand(sublime_plugin.WindowCommand):
	def is_enabled(self):
		return gs.is_go_source_view(self.window.active_view())

	def run(self, dir=''):
		win, view = gs.win_view(None, self.window)
		if view is None:
			return

		current = "Current Package"

		def f(im, _):
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

		margo.call(
			path='/import_paths',
			args={},
			cb=f,
			message='fetching imprt paths'
		)


	def present(self, vfn, src, pkg_dir):
		win = self.window
		if win is None:
			return

		def f(res, err):
			if err:
				gs.notice(DOMAIN, err)
				return

			decls = res.get('file_decls', [])
			for d in res.get('pkg_decls', []):
				if not vfn or d['fn'] != vfn:
					decls.append(d)

			for d in decls:
				dname = (d['repr'] or d['name'])
				trailer = []
				trailer.extend(GOOS_PAT.findall(d['fn']))
				trailer.extend(GOARCH_PAT.findall(d['fn']))
				if trailer:
					trailer = ' (%s)' % ', '.join(trailer)
				else:
					trailer = ''
				d['ent'] = '%s %s%s' % (d['kind'], dname, trailer)

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

		margo.call(
			path='/declarations',
			args={
				'fn': vfn,
				'src': src,
				'pkg_dir': pkg_dir,
			},
			default={},
			cb=f,
			message='fetching pkg declarations'
		)


class GsBrowsePackagesCommand(sublime_plugin.WindowCommand):
	def run(self):
		win = self.window
		def f(res, err):
			if err:
				gs.notice(DOMAIN, err)
				return

			m = {}
			for root, dirs in res.iteritems():
				for dir, fn in dirs.iteritems():
					if not m.get(dir):
						m[dir] = fn
			ents = m.keys()
			if ents:
				ents.sort(key = lambda a: a.lower())
				def cb(i):
					if i >= 0:
						fn = m[ents[i]]
						gs.focus(fn, 0, 0, win)
				win.show_quick_panel(ents, cb)
			else:
				win.show_quick_panel([['', 'No source directories found']], lambda x: None)

		margo.call(
			path='/pkgdirs',
			args={},
			default={},
			cb=f,
			message='fetching pkg dirs'
		)

class GsBrowseFilesCommand(sublime_plugin.WindowCommand):
	def is_enabled(self):
		return gs.is_go_source_view(self.window.active_view())

	def run(self):
		win = self.window
		view = gs.active_valid_go_view(win=win)
		ents = []
		m = {}
		if view:
			def f(res, err):
				if err:
					gs.notice(DOMAIN, err)
					return

				if len(res) == 1:
					for pkgname, filenames in res.iteritems():
						for name, fn in filenames.iteritems():
							m[name] = fn
							ents.append(name)
				else:
					for pkgname, filenames in res.iteritems():
						for name, fn in filenames.iteritems():
							s = '(%s) %s' % (pkgname, name)
							m[s] = fn
							ents.append(s)

				if ents:
					ents.sort(key = lambda a: a.lower())
					def cb(i):
						if i >= 0:
							gs.focus(m[ents[i]], 0, 0, win)
					win.show_quick_panel(ents, cb)
				else:
					win.show_quick_panel([['', 'No files found']], lambda x: None)

			margo.call(
				path='/pkgfiles',
				args={
					'path': gs.basedir_or_cwd(view.file_name()),
				},
				default={},
				cb=f,
				message='fetching pkg files'
			)


