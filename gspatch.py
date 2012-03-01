import sublime
import gscommon as gs
from difflib import SequenceMatcher

class MergeException(Exception):
	pass

def merge(view, offset, size, dst):
	if size < 0:
		size = view.size()
	edit = view.begin_edit()
	try:
		_merge(view, edit, offset, size, dst)
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

def _merge(view, edit, offset, size, dst):
	def src(start, end):
		return view.substr(sublime.Region(start, end))
	dirty = False
	sm = SequenceMatcher(None, src(offset, offset+size), dst)
	for tag, s1, s2, d1, d2 in sm.get_opcodes():
		s1 += offset
		s2 += offset
		if tag == 'equal':
			if src(s1, s2) != dst[d1:d2]:
				raise MergeException('mismatch', dirty)
		else:
			dirty = True
			if tag == 'replace':
				view.replace(edit, sublime.Region(s1, s2), dst[d1:d2])
			elif tag == 'insert':
				view.insert(edit, s1, dst[d1:d2])
			elif tag == 'delete':
				view.erase(edit, sublime.Region(s1, s2))
			else:
				raise MergeException('unreachable', dirty)
