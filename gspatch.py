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
	return dirty

def merge(view, size, text):
	try:
		edit = view.begin_edit()
		dirty = False
		err = ''
		vs = view.settings()
		ttts = vs.get("translate_tabs_to_spaces")
		vs.set("translate_tabs_to_spaces", False)
		if size < 0:
			size = view.size()
		dirty = _merge(view, size, text, edit)
	except MergeException as (err, d):
		dirty = d
		err = "Could not merge changes into the buffer, edit aborted: %s" % err
	except Exception as ex:
		err = "where ma bees at?: %s" % ex
	finally:
		view.end_edit(edit)
		vs.set("translate_tabs_to_spaces", ttts)
		return (dirty, err)

