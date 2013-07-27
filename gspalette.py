from gosubl import gs
from gosubl import gspatch
from gosubl import mg9
from os.path import dirname, basename, relpath
import gslint
import re
import sublime
import sublime_plugin

DOMAIN = 'GsPalette'

class Loc(object):
	def __init__(self, fn, row, col=0):
		self.fn = fn
		self.row = row
		self.col = col

class GsPaletteCommand(sublime_plugin.WindowCommand):
	def is_enabled(self):
		return bool(gs.active_valid_go_view(self.window))

	def run(self, palette='auto', direct=False):
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

		if palette == 'jump_back':
			self.jump_back()
		elif palette == 'jump_to_imports':
			self.jump_to_imports()
		else:
			self.show_palette(palette, direct)

	def show_palette(self, palette, direct=False):
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
				gs.notice(DOMAIN, 'Invalid palette `%s`' % palette)
				palette = ''

		if not direct and len(self.bookmarks) > 0:
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

		if not direct and palette:
			self.add_item(u'@%s \u21B5' % palette.title(), self.show_palette, 'main')

		li1 = len(self.items)
		if pcb:
			pcb(view, direct)

		if not direct:
			for k in sorted(self.palettes.keys()):
				if k:
					if k != palette:
						ttl = '@' + k.title()
						if k == 'errors':
							fr = gslint.ref(view.file_name())
							if not fr or len(fr.reports) == 0:
								continue
							ttl = '%s (%d)' % (ttl, len(fr.reports))
						itm = ttl
						self.add_item(itm, self.show_palette, k)

	def do_show_panel(self):
		# todo cleanup this file and get rid of the old gspalette
		items = []
		actions = {}
		for tup in self.items:
			item, action, args = tup
			actions[len(items)] = (action, args)
			items.append(item)
		self.items = []

		def on_done(i, win):
			action, args = actions.get(i, (None, None))
			if i >= 0 and action:
				action(args)
		gs.show_quick_panel(items, on_done)

	def add_item(self, item, action=None, args=None):
		self.items.append((item, action, args))

	def log_bookmark(self, view, loc):
		bks = self.bookmarks
		if len(bks) == 0 or (bks[-1].row != loc.row and bks[-1].fn != view.file_name()):
			bks.append(loc)

	def goto(self, loc):
		gs.focus(loc.fn, loc.row, loc.col)

	def jump_to_imports(self):
		view = gs.active_valid_go_view()
		if not view:
			return

		last_import = gs.attr('last_import_path.%s' % gs.view_fn(view), '')
		r = None
		if last_import:
			offset = len(last_import) + 2
			last_import = re.escape(last_import)
			pat = '(?s)import.*?(?:"%s"|`%s`)' % (last_import, last_import)
			r = view.find(pat, 0)

		if not r:
			offset = 1
			pat = '(?s)import.*?["`]'
			r = view.find(pat, 0)

		if not r:
			gs.notice(DOMAIN, "cannot find import declarations")
			return

		pt = r.end() - offset
		row, col = view.rowcol(pt)
		loc = Loc(view.file_name(), row, col)
		self.jump_to((view, loc))

	def jump_back(self, _=None):
		if len(self.bookmarks) > 0:
			self.goto(self.bookmarks.pop())

	def palette_errors(self, view, direct=False):
		indent = '' if direct else '    '
		reps = {}
		fr = gslint.ref(view.file_name())
		if fr:
			reps = fr.reports.copy()
		keys = sorted(reps.keys())
		if keys:
			for k in keys:
				r = reps[k]
				loc = Loc(view.file_name(), r.row, r.col)
				m = []
				m.append("%sline %d:" % (indent, r.row+1))
				lc = 0
				for ln in r.msg.split('\n'):
					if ln:
						lc += 1
						if len(ln) > 50:
							m.append('\t%d: %s -' % (lc, ln[:50]))
							m.append('\t  %s' % ln[50:])
						else:
							m.append('\t%d: %s' % (lc, ln))

				self.add_item(m, self.jump_to, (view, loc))
		else:
			self.add_item(['', 'No errors to report'])

		self.do_show_panel()


	def palette_imports(self, view, direct=False):
		indent = '' if direct else '    '
		src = view.substr(sublime.Region(0, view.size()))
		def f(im, err):
			if err:
				gs.notice(DOMAIN, err)
				return

			delete_imports = []
			add_imports = []
			paths = im.get('paths', {})
			for path in paths:
				skipAdd = False
				for i in im.get('imports', []):
					if i.get('path') == path:
						skipAdd = True
						name = i.get('name', '')
						if not name:
							name = basename(path)
						if name == path:
							delete_imports.append(('%sdelete: %s' % (indent, name), i))
						else:
							delete_imports.append(('%sdelete: %s ( %s )' % (indent, name, path), i))

				if not skipAdd:
					s = '%s%s' % (indent, path)
					m = {
						'path': path,
						'add': True,
					}

					nm = paths[path]
					if nm and nm != path and not path.endswith('/%s' % nm):
						s = '%s (%s)' % (s, nm)
						if gs.setting('use_named_imports') is True:
							m['name'] = nm

					add_imports.append((s, m))

			for i in sorted(delete_imports):
				self.add_item(i[0], self.toggle_import, (view, i[1]))
			if len(delete_imports) > 0:
				self.add_item(' ', self.show_palette, 'imports')
			for i in sorted(add_imports):
				self.add_item(i[0], self.toggle_import, (view, i[1]))

			self.do_show_panel()

		mg9.import_paths(view.file_name(), src, f)

	def toggle_import(self, a):
		view, decl = a
		im, err = mg9.imports(
			view.file_name(),
			view.substr(sublime.Region(0, view.size())),
			[decl]
		)

		if err:
			gs.notice(DOMAIN, err)
		else:
			src = im.get('src', '')
			line_ref = im.get('lineRef', 0)
			r = view.full_line(view.text_point(max(0, line_ref-1), 0))
			if not src or line_ref < 1 or not r:
				return

			view.run_command('gs_patch_imports', {
				'pos': r.end(),
				'content': src,
				'added_path': (decl.get('path') if decl.get('add') else '')
			})

	def jump_to(self, a):
		view, loc = a
		row, col = gs.rowcol(view)
		if loc.row != row:
			self.log_bookmark(view, Loc(view.file_name(), row, col))
		self.goto(loc)

	def palette_declarations(self, view, direct=False):
		def f(res, err):
			if err:
				gs.notify('GsDeclarations', err)
			else:
				decls = res.get('file_decls', [])
				decls.sort(key=lambda v: v.get('row', 0))
				added = 0
				for i, v in enumerate(decls):
					loc = Loc(v['fn'], v['row'], v['col'])
					s = '%s %s' % (v['kind'], (v['repr'] or v['name']))
					self.add_item(s, self.jump_to, (view, loc))
					added += 1

			if added < 1:
				self.add_item(['', 'No declarations found'])

			self.do_show_panel()

		mg9.declarations(gs.view_fn(view), gs.view_src(view), '', f)
