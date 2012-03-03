import sublime, sublime_plugin
import gspatch, margo, gscommon as gs
from os.path import dirname, relpath, basename

class Loc(object):
	def __init__(self, fn, row, col=0):
		self.fn = fn
		self.row = row
		self.col = col

class GsPaletteCommand(sublime_plugin.WindowCommand):
	def run(self, palette='main'):
		if not hasattr(self, 'items'):
			self.items = []
			self.bookmarks = []
		
		palettes = {
			'main': self.act_show_main_palette,
			'declarations': self.act_list_declarations,
		}
		
		p = palettes.get(palette)
		if p:
			p()
		else:
			gs.notice('GsPalette', 'Invalid palette `%s`' % palette)
			palettes['main']()

	def act_show_main_palette(self, _=None):
		view = gs.active_valid_go_view(self.window)
		if view:
			errors = gs.l_errors.get(view.id(), {})
			for k in sorted(errors.keys()):
				er = errors[k]
				loc = Loc(view.file_name(), er.row, er.col)
				self.add_item(["Error on line %d" % (er.row+1), er.err], self.act_jump_to, loc)
			
			if gs.setting('margo_enabled', False):
				self.add_item("Add/Remove imports", self.act_show_import_palette)
				self.add_item("List Declarations", self.act_list_declarations)
			self.show_palette()

	def show_palette(self):
		view = gs.active_valid_go_view(self.window)
		if view:
			items = [[' ', 'GoSublime Palette']]
			actions = {}
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
				items[0][0] = u'\u2190 Go Back (%s%s)' % (fn, line)
				actions[0] = (self.act_jump_back, None)
			
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

	def log_bookmark(self, loc):
		view = gs.active_valid_go_view(self.window)
		if view:
			bks = self.bookmarks
			if len(bks) == 0 or (bks[-1].row != loc.row and bks[-1].fn != view.file_name()):
				bks.append(loc)
	
	def goto(self, loc):
		self.window.open_file('%s:%d:%d' % (loc.fn, loc.row+1, loc.col+1), sublime.ENCODED_POSITION)

	def act_jump_back(self, _):
		if len(self.bookmarks) > 0:
			self.goto(self.bookmarks.pop())

	def act_show_import_palette(self, _):
		view = gs.active_valid_go_view(self.window)
		if view:
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
						delete_imports.append(([name, 'delete import `%s`' % path], i))

				if not skipAdd:
					add_imports.append(([path], {'path': path}))
			for i in sorted(delete_imports):
				self.add_item(i[0], self.toggle_import, (view, i[1]))
			for i in sorted(add_imports):
				self.add_item(i[0], self.toggle_import, (view, i[1]))
			self.show_palette()

	def toggle_import(self, a):
		view, decl = a
		im, err = margo.imports(
			view.file_name(),
			view.substr(sublime.Region(0, view.size())),
			False,
			[decl]
		)
		if err:
			gs.notice('GsPalette', err)
		else:
			src = im.get('src', '')
			size_ref = im.get('size_ref', 0)
			if src and size_ref > 0:
				if gspatch.merge(view, size_ref, src) == '':
					gs.notice('GsPalette', 'imports ammended...')

	def act_jump_to(self, loc):
		view = gs.active_valid_go_view(self.window)
		if view:
			row, col = gs.rowcol(view)
			if loc.row != row:
				self.log_bookmark(Loc(view.file_name(), row, col))
			self.goto(loc)

	def act_list_declarations(self, _=None):
		view = gs.active_valid_go_view(self.window)
		if view:
			decls, err = margo.declarations(
				view.file_name(),
				view.substr(sublime.Region(0, view.size()))
			)
			if err:
				gs.notice('GsPalette', err)
			decls.sort(key=lambda v: v['line'])
			for i, v in enumerate(decls):
				if v['name'] in ('main', 'init', '_'):
					continue
				loc = Loc(v['filename'], v['line']-1, v['column']-1)
				prefix = u' %s \u00B7   ' % gs.CLASS_PREFIXES.get(v['kind'], '')
				self.add_item([prefix+v['name'], v['doc']], self.act_jump_to, loc)
			self.show_palette()
