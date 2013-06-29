from gosubl import gs
import sublime
import sys

def plugin_loaded():
	old = [
		'GoSublime.tmLanguage',
		'GoSublime-next.tmLanguage',
	]

	fn = 'Packages/GoSublime/syntax/GoSublime-Go.tmLanguage'

	for w in sublime.windows():
		for v in w.views():
			stx = v.settings().get('syntax')
			if stx:
				name = stx.replace('\\', '/').split('/')[-1]
				if name in old:
					print('GoSublime: changing syntax of `%s` from `%s` to `%s`' % (
						gs.view_fn(v),
						stx,
						fn
					))
					v.set_syntax_file(fn)


st2 = (sys.version_info[0] == 2)
if st2:
	sublime.set_timeout(plugin_loaded, 0)
