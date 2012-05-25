import sublime, sublime_plugin
import gscommon as gs, margo, gspatch

DOMAIN = 'GsFmt'

class GsFmtCommand(sublime_plugin.TextCommand):
	def run(self, edit):
		if not (gs.setting('fmt_enabled', False) is True and gs.is_go_source_view(self.view)):
			return

		vsize = self.view.size()
		src = self.view.substr(sublime.Region(0, vsize))
		if not src.strip():
			return

		src, err = margo.fmt(self.view.file_name(), src)
		if err or not src.strip():
			return

		dirty, err = gspatch.merge(self.view, vsize, src)
		if err:
			gs.notice_undo(DOMAIN, err, self.view, dirty)
