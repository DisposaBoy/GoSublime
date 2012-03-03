import sublime
from something_borrowed.diff_match_patch.diff_match_patch import diff_match_patch
import gscommon as gs

class MergeException(Exception):
	pass

def _merge(view, size, text, edit):
	def ss(start, end):
		return view.substr(sublime.Region(start, end))
	dmp = diff_match_patch()
	diffs = dmp.diff_main(ss(0, size), text)
	dmp.diff_cleanupEfficiency(diffs)
	i = 0
	dirty = False
	for d in diffs:
		k, s = d
		l = len(s)
		if k == 0:
			# match
			l = len(s)
			if ss(i, i+l) != s:
				raise MergeException('mismatch', dirty)
			i += l
		else:
			dirty = True
			if k > 0:
				# insert
				view.insert(edit, i, s)
				i += l
			else:
				# delete
				if ss(i, i+l) != s:
					raise MergeException('mismatch', dirty)
				view.erase(edit, sublime.Region(i, i+l))

def merge(view, size, text):
	if size < 0:
		size = view.size()
	edit = view.begin_edit()
	try:
		_merge(view, size, text, edit)
	except MergeException as (err, dirty):
		def cb():
			if dirty:
				view.run_command('undo')
			gs.notice("GsPatch", "Could not merge changes into the buffer, edit aborted: %s" % err)
		sublime.set_timeout(cb, 0)
		return err
	finally:
		view.end_edit(edit)
	return ''

