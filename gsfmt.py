import sublime, sublime_plugin
import gscommon as gs, margo, gspatch

DOMAIN = 'GsFmt'

class GsFmtCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		return gs.setting('fmt_enabled', False) is True and gs.is_go_source_view(self.view)

	def run(self, edit):
		vsize = self.view.size()
		src = self.view.substr(sublime.Region(0, vsize))
		if not src.strip():
			return

		src, err = margo.fmt(self.view.file_name(), src)
		if err:
			gs.notice(DOMAIN, "cannot fmt file. error: `%s'" % err)
			return
		if not src.strip():
			gs.notice(DOMAIN, "cannot fmt file. it appears to contain syntax errors")
			return

		dirty, err = gspatch.merge(self.view, vsize, src)
		if err:
			gs.notice_undo(DOMAIN, "cannot fmt file. merge failure: `%s'" % err, self.view, dirty)
