import sublime, sublime_plugin
import gscommon as gs

class GsPaletteCommand(sublime_plugin.WindowCommand):
	def run(self):
		if not hasattr(self, 'items'):
			self.items = []
			self.bookmarks = []
		
		self.act_show_palette()

	def act_show_palette(self, _=None):
		view = gs.active_valid_go_view()
		if view:
			errors = gs.l_errors.get(view.id(), {})
			for k in errors:
				er = errors[k]
				self.add_item(["Error on line %d" % (er.row+1), er.err], self.act_goto_error, er)
			self.show()

	def show(self):
		view = gs.active_valid_go_view()
		if view:
			items = [[' ', 'GoSublime Palette']]
			actions = {}
			l = len(self.bookmarks)
			if l > 0:
				items[0][0] = u'\u2190 Go Back (%d)' % l
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

	def log_bookmark(self):
		view = gs.active_valid_go_view()
		if view:
			rc = view.rowcol(view.sel()[0].begin())
			if len(self.bookmarks) == 0 or self.bookmarks[-1] != rc:
				self.bookmarks.append(rc)
	
	def act_jump_back(self, _):
		try:
			view = gs.active_valid_go_view()
			row, col = self.bookmarks.pop()
			view.run_command("gs_goto_row_col", {"row": row, "col": col})
		except:
			pass
	
	def act_goto_error(self, er):
		view = gs.active_valid_go_view()
		if view:
			self.log_bookmark()
			view.run_command("gs_goto_row_col", {"row": er.row, "col": er.col})
	
