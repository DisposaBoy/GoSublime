import os
import sublime
import sublime_plugin
import sys
import traceback

st2 = (sys.version_info[0] == 2)
dist_dir = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, dist_dir)

ANN = ''
VERSION = ''
MARGO_EXE = ''
fn = os.path.join(dist_dir, 'gosubl', 'about.py')
execErr = ''
try:
	with open(fn) as f:
		code = compile(f.read(), fn, 'exec')
		exec(code)
except Exception:
	execErr = "Error: failed to exec about.py: Exception: %s" % traceback.format_exc()
	print("GoSublime: %s" % execErr)

def loadable_mods():
	from .gosubl import gs
	from .gosubl import sh
	from .gosubl import margo
	from .gosubl import mg9

	return [
		('gs', gs),
		('sh', sh),
		('margo', margo),
		('mg9', mg9),
	]

def plugin_loaded():
	from .gosubl import about
	from .gosubl import sh
	from .gosubl import ev
	from .gosubl import gs

	if VERSION != about.VERSION:
		gs.show_output('GoSublime-main', '\n'.join([
			'GoSublime has been updated.',
			'New version: `%s`, current version: `%s`' % (VERSION, about.VERSION),
			'Please restart Sublime Text to complete the update.',
			execErr,
		]))
		return

	if gs.attr('about.version'):
		gs.show_output('GoSublime-main', '\n'.join([
			'GoSublime appears to have been updated.',
			'New ANNOUNCE: `%s`, current ANNOUNCE: `%s`' % (ANN, about.ANN),
			'You may need to restart Sublime Text.',
		]))
		return

	gs.set_attr('about.version', VERSION)
	gs.set_attr('about.ann', ANN)

	for mod_name, mod in loadable_mods():
		print('GoSublime %s: %s.init()' % (VERSION, mod_name))

		try:
			mod.gs_init({
				'version': VERSION,
				'ann': ANN,
				'margo_exe': MARGO_EXE,
			})
		except AttributeError:
			pass
		except TypeError:
			# old versions didn't take an arg
			mod.gs_init()

	ev.init.post_add = lambda e, f: f()
	ev.init()

	def cb():
		aso = gs.aso()
		old_version = aso.get('version', '')
		old_ann = aso.get('ann', '')
		if about.VERSION > old_version or about.ANN > old_ann:
			aso.set('version', about.VERSION)
			aso.set('ann', about.ANN)
			gs.save_aso()
			gs.focus(gs.dist_path('CHANGELOG.md'))

	sublime.set_timeout(cb, 0)

def plugin_unloaded():
	for mod_name, mod in loadable_mods():
		try:
			fini = mod.gs_fini
		except AttributeError:
			continue

		print('GoSublime %s: %s.fini()' % (VERSION, mod_name))
		fini({
		})

class GosublimeDoesntSupportSublimeText2(sublime_plugin.TextCommand):
	def run(self, edit):
		msg = '\n'.join([
			'Sublime Text 2 is no longer supported by GoSublime'+
			'',
			'See https://github.com/DisposaBoy/GoSublime/blob/master/SUPPORT.md#sublime-text',
			'',
			'If you have a *good* reason to not upgrade to Sublime Text 3,',
			'discuss it here https://github.com/DisposaBoy/GoSublime/issues/689',
			'',
		])
		self.view.set_scratch(True)
		self.view.set_syntax_file(gs.tm_path('9o'))
		self.view.set_name('GoSublime no longer supports Sublime Text 2')
		self.view.insert(edit, 0, msg)
		self.view.set_read_only(True)

if st2:
	def cb():
		view = sublime.active_window().new_file()
		view.run_command('gosublime_doesnt_support_sublime_text2')

	sublime.set_timeout(cb, 1000)
