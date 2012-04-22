import sublime, sublime_plugin
import gspatch, margo, gscommon as gs
from os.path import dirname, relpath, basename

class Loc(object):
	def __init__(self, fn, row, col=0):
		self.fn = fn
		self.row = row
		self.col = col

class GsPaletteCommand(sublime_plugin.WindowCommand):
	def run(self, palette='auto'):
		if not hasattr(self, 'items'):
			self.items = []
			self.bookmarks = []
			self.last_activate_palette = ''
			self.requires_margo = ['declarations', 'imports']
			self.palettes = {
				'declarations': self.palette_declarations,
				'imports': self.palette_imports,
				'errors': self.palette_errors,
			}

		self.show_palette(palette)

	def show_palette(self, palette):
		view = gs.active_valid_go_view(self.window)
		if not view:
			return

		palette = palette.lower().strip()
		if palette == 'auto':
			palette = self.last_activate_palette
		elif palette == 'main':
			palette = ''

		pcb = None
		if palette:
			pcb = self.palettes.get(palette)
			if pcb:
				self.last_activate_palette = palette
			else:
				gs.notice('GsPalette', 'Invalid palette `%s`' % palette)
				palette = ''

		if len(self.bookmarks) > 0:
			loc = self.bookmarks[-1]
			line = 'line %d' % (loc.row + 1)
			if view.file_name() == loc.fn:
				fn = ''
			else:
				fn = relpath(loc.fn, dirname(loc.fn))
				if fn.startswith('..'):
					fn = loc.fn
				fn = '%s ' % fn
			self.add_item(u'\u2190 Go Back (%s%s)' % (fn, line), self.jump_back, None)

		if palette:
			self.add_item(u'@%s \u21B5' % palette.title(), self.show_palette, 'main')

		li1 = len(self.items)
		if pcb:
			pcb(view)

		for k in sorted(self.palettes.keys()):
			if k:
				if k != palette:
					ttl = '@' + k.title()
					if k == 'errors':
						l = len(gs.l_errors.get(view.id(), {}))
						if l == 0:
							continue
						ttl = '%s (%d)' % (ttl, l)
					itm = ttl
					self.add_item(itm, self.show_palette, k)

		items = []
		actions = {}
		for tup in self.items:
			item, action, args = tup
			actions[len(items)] = (action, args)
			items.append(item)
		self.items = []

		def on_done(i):
			action, args = actions.get(i, (None, None))
			if i >= 0 and action:
				action(args)
		self.window.show_quick_panel(items, on_done)

	def add_item(self, item, action=None, args=None):
		self.items.append((item, action, args))

	def log_bookmark(self, view, loc):
		bks = self.bookmarks
		if len(bks) == 0 or (bks[-1].row != loc.row and bks[-1].fn != view.file_name()):
			bks.append(loc)

	def goto(self, loc):
		self.window.open_file('%s:%d:%d' % (loc.fn, loc.row+1, loc.col+1), sublime.ENCODED_POSITION)

	def jump_back(self, _):
		if len(self.bookmarks) > 0:
			self.goto(self.bookmarks.pop())

	def palette_errors(self, view):
		errors = gs.l_errors.get(view.id(), {})
		for k in sorted(errors.keys()):
			er = errors[k]
			loc = Loc(view.file_name(), er.row, er.col)
			self.add_item("    line %d: %s" % ((er.row+1), er.err), self.jump_to, (view, loc))

	def palette_imports(self, view):
		im, err = margo.imports(
			view.file_name(),
			view.substr(sublime.Region(0, view.size())),
			True,
			[]
		)
		if err:
			gs.notice('GsPalette', err)

		delete_imports = []
		add_imports = []
		imports = im.get('file_imports', [])
		for path in im.get('import_paths', []):
			skipAdd = False
			for i in imports:
				if i.get('path') == path:
					skipAdd = True
					name = i.get('name', '')
					if not name:
						name = basename(path)
					if name == path:
						delete_imports.append(('    %s - ( delete )' % name, i))
					else:
						delete_imports.append(('    %s - ( delete %s )' % (name, path), i))

			if not skipAdd:
				add_imports.append(('    %s' % path, {'path': path}))
		for i in sorted(delete_imports):
			self.add_item(i[0], self.toggle_import, (view, i[1]))
		self.add_item('    -', self.show_palette, 'imports')
		for i in sorted(add_imports):
			self.add_item(i[0], self.toggle_import, (view, i[1]))

	def toggle_import(self, a):
		view, decl = a
		im, err = margo.imports(
			view.file_name(),
			view.substr(sublime.Region(0, view.size())),
			False,
			[decl]
		)
		if err:
			gs.notice('GsImports', err)
		else:
			src = im.get('src', '')
			size_ref = im.get('size_ref', 0)
			if src and size_ref > 0:
				dirty, err = gspatch.merge(view, size_ref, src)
				if err:
					gs.notice_undo('GsImports', err, view, dirty)
				elif dirty:
					gs.notice('GsImports', 'imports updated...')

	def jump_to(self, a):
		view, loc = a
		row, col = gs.rowcol(view)
		if loc.row != row:
			self.log_bookmark(view, Loc(view.file_name(), row, col))
		self.goto(loc)

	def palette_declarations(self, view):
		decls, err = margo.declarations(
			view.file_name(),
			view.substr(sublime.Region(0, view.size()))
		)
		if err:
			gs.notice('GsDeclarations', err)
		decls.sort(key=lambda v: v['line'])
		for i, v in enumerate(decls):
			if v['name'] == '_':
				continue
			loc = Loc(v['filename'], v['line']-1, v['column']-1)
			prefix = u'    %s \u00B7   ' % gs.CLASS_PREFIXES.get(v['kind'], '')
			self.add_item(prefix+v['name'], self.jump_to, (view, loc))
