import sublime, sublime_plugin
import margo, gscommon as gs
from os.path import dirname, relpath

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
			for k in errors:
				er = errors[k]
				loc = Loc(view.file_name(), er.row, er.col)
				self.add_item(["Error on line %d" % (er.row+1), er.err], self.act_jump_to, loc)
			if gs.setting('margo_addr'):
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
			m = margo.request('/declarations', {
				'filename': view.file_name(),
				'src': view.substr(sublime.Region(0, view.size()))
			})
			if m:
				if m.has_key('error'):
					gs.notice('GsPalette', m['error'])
				for i, v in enumerate(m.get('declarations', [])):
					if v['name'] in ('main', 'init'):
						continue
					loc = Loc(v['filename'], v['line']-1, v['column']-1)
					prefix = u' %s \u00B7   ' % gs.CLASS_PREFIXES.get(v['kind'], '')
					self.add_item([prefix+v['name'], v['doc']], self.act_jump_to, loc)
				self.show_palette()
		
