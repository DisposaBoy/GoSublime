from gosubl import gs
import os
import sublime_plugin

def _stx(v):
	old = [
		'GoSublime.tmLanguage',
		'GoSublime-next.tmLanguage',
	]

	fn = 'Packages/GoSublime/syntax/GoSublime-Go.tmLanguage'
	if not os.path.exists(gs.dist_path('syntax/GoSublime-Go.tmLanguage')):
		return

	stx = v.settings().get('syntax')
	if stx:
		name = stx.replace('\\', '/').split('/')[-1]
		if name in old:
			print('GoSublime: changing syntax of `%s` from `%s` to `%s`' % (
				(v.file_name() or ('view://%s' % v.id())),
				stx,
				fn
			))
			v.set_syntax_file(fn)


class Ev(sublime_plugin.EventListener):
	def on_load(self, view):
		_stx(view)

	def on_activated(self, view):
		_stx(view)
