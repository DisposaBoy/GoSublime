from gosubl import gs
from gosubl import gsq
from gosubl import mg9
import os
import re
import sublime
import sublime_plugin

DOMAIN = 'GsDoc'

GOOS_PAT = re.compile(r'_(%s)' % '|'.join(gs.GOOSES))
GOARCH_PAT = re.compile(r'_(%s)' % '|'.join(gs.GOARCHES))
EXT_EXCLUDE = [
	'out', 'exe', 'o', 'dll', 'so', 'a', 'dynlib', 'lib', 'com', 'bin', 'pyc', 'pyo', 'cache', 'db',
	'bak', 'png', 'gif', 'jpeg', 'jpg', 'gz', 'zip', '7z', 'rar', 'tar', '1', '2', '3', 'old', 'tgz',
	'pprof', 'prof', 'mem', 'cpu', 'swap',
]

class GsDocCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		return gs.is_go_source_view(self.view)

	def show_output(self, s):
		gs.show_output(DOMAIN+'-output', s, False, 'GsDoc')

	def run(self, _, mode=''):
		view = self.view
		if (not gs.is_go_source_view(view)) or (mode not in ['goto', 'hint']):
			return

		pt = gs.sel(view).begin()
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

		mg9.doc(view.file_name(), src, pt, f)

class GsBrowseDeclarationsCommand(sublime_plugin.WindowCommand):
	def run(self, dir=''):
		if dir == '.':
			self.present_current()
		elif dir:
			self.present('', '', dir)
		else:
			def f(res, err):
				if err:
					gs.notice(DOMAIN, err)
					return

				ents, m = handle_pkgdirs_res(res)
				if ents:
					ents.insert(0, "Current Package")

					def cb(i, win):
						if i == 0:
							self.present_current()
						elif i >= 1:
							self.present('', '', os.path.dirname(m[ents[i]]))

					gs.show_quick_panel(ents, cb)
				else:
					gs.show_quick_panel([['', 'No source directories found']])

			mg9.pkg_dirs(f)

	def present_current(self):
		pkg_dir = ''
		view = gs.active_valid_go_view(win=self.window, strict=False)
		if view:
			if view.file_name():
				pkg_dir = os.path.dirname(view.file_name())
			vfn = gs.view_fn(view)
			src = gs.view_src(view)
		else:
			vfn = ''
			src = ''
		self.present(vfn, src, pkg_dir)

	def present(self, vfn, src, pkg_dir):
		win = self.window
		if win is None:
			return

		def f(res, err):
			if err:
				gs.notify(DOMAIN, err)
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

			def cb(i, win):
				if i >= 0:
					d = decls[i]
					gs.focus(d['fn'], d['row'], d['col'], win)

			if ents:
				gs.show_quick_panel(ents, cb)
			else:
				gs.show_quick_panel([['', 'No declarations found']])

		mg9.declarations(vfn, src, pkg_dir, f)

def handle_pkgdirs_res(res):
	m = {}
	for root, dirs in res.items():
		for dir, fn in dirs.items():
			if not m.get(dir):
				m[dir] = fn
	ents = m.keys()
	ents.sort(key = lambda a: a.lower())
	return (ents, m)

class GsBrowsePackagesCommand(sublime_plugin.WindowCommand):
	def run(self):
		def f(res, err):
			if err:
				gs.notice(DOMAIN, err)
				return

			ents, m = handle_pkgdirs_res(res)
			if ents:
				def cb(i, win):
					if i >= 0:
						dirname = gs.basedir_or_cwd(m[ents[i]])
						win.run_command('gs_browse_files', {'dir': dirname})
				gs.show_quick_panel(ents, cb)
			else:
				gs.show_quick_panel([['', 'No source directories found']])

		mg9.pkg_dirs(f)

def ext_filter(pathname, basename, ext):
	if not ext:
		return basename == "makefile"

	if ext in EXT_EXCLUDE:
		return False

	if ext.endswith('~'):
		return False

	return True

def show_pkgfiles(dirname):
	ents = []
	m = {}

	try:
		dirname = os.path.abspath(dirname)
		for fn in gs.list_dir_tree(dirname, ext_filter, gs.setting('fn_exclude_prefixes', [])):
			name = os.path.relpath(fn, dirname).replace('\\', '/')
			m[name] = fn
			ents.append(name)
	except Exception as ex:
		gs.notice(DOMAIN, 'Error: %s' % ex)

	if ents:
		ents.sort(key = lambda a: a.lower())

		try:
			s = " ../  ( current: %s )" % dirname
			m[s] = os.path.join(dirname, "..")
			ents.insert(0, s)
		except Exception:
			pass

		def cb(i, win):
			if i >= 0:
				fn = m[ents[i]]
				if os.path.isdir(fn):
					win.run_command("gs_browse_files", {"dir": fn})
				else:
					gs.focus(fn, 0, 0, win)
		gs.show_quick_panel(ents, cb)
	else:
		gs.show_quick_panel([['', 'No files found']])

class GsBrowseFilesCommand(sublime_plugin.WindowCommand):
	def run(self, dir=''):
		if not dir:
			view = self.window.active_view()
			dir = gs.basedir_or_cwd(view.file_name() if view is not None else None)
		gsq.dispatch('*', lambda: show_pkgfiles(dir), 'scanning directory for package files')
