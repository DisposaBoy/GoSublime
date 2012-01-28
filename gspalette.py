import sublime, sublime_plugin
import gscommon as gs
from os.path import dirname, relpath

class GsPaletteCommand(sublime_plugin.WindowCommand):
	def run(self):
		if not hasattr(self, 'items'):
			self.items = []
			self.bookmarks = []
		
		self.act_show_palette()

	def act_show_palette(self, _=None):
		view = gs.active_valid_go_view(self.window)
		if view:
			errors = gs.l_errors.get(view.id(), {})
			for k in errors:
				er = errors[k]
				self.add_item(["Error on line %d" % (er.row+1), er.err], self.act_goto_error, (view, er))
			self.show()

	def show(self):
		view = gs.active_valid_go_view(self.window)
		if view:
			items = [[' ', 'GoSublime Palette']]
			actions = {}
			if len(self.bookmarks) > 0:
				b = self.bookmarks[-1]
				line = 'line %d' % (b[1] + 1)
				if view.file_name() == b[0]:
					fn = ''
				else:
					fn = relpath(b[0], dirname(b[0]))
					if fn.startswith('..'):
						fn = b[0]
					fn = '%s ' % fn
				items[0][0] = u'\u2190 Go Back (%s%s)' % (fn, line)
				actions[0] = (self.act_jump_back, None)
			
			self.items.sort()
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

	def log_bookmark(self, fn, row, col=0):
		view = gs.active_valid_go_view(self.window)
		if view and fn:
			bks = self.bookmarks
			if len(bks) == 0 or (bks[-1][1] != row and bks[-1][0] != view.file_name()):
				bks.append((fn, row, col))
	
	def act_jump_back(self, _):
		if len(self.bookmarks) > 0:
			fn, r, c = self.bookmarks.pop()
			self.window.open_file('%s:%d:%d' % (fn, r+1, c+1), sublime.ENCODED_POSITION)
	
	def act_goto_error(self, ve):
		view, er = ve
		row, col = gs.rowcol(view)
		if er.row != row:
			self.log_bookmark(view.file_name(), row, col)
		view.run_command("gs_goto_row_col", {"row": er.row, "col": er.col})
		
