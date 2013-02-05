import os
import sublime
import sublime_plugin
import sys
import time
import traceback

start = time.time()
st2 = (sys.version_info[0] == 2)
dist_dir = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, dist_dir)

mods = [
	'gs',
	'mg9',
	'gs9o',
	'gscommands',
	'gscomplete',
	'gsdoc',
	'gsev',
	'gsfmt',
	'gsinfer',
	'gslint',
	'gspalette',
	'gspatch',
	'gspkgdoc',
	'gsq',
	'gsshell',
	'gstest',
]

gs_plugin_loaded_called = False
def import_mods():
	dwb = sys.dont_write_bytecode
	sys.dont_write_bytecode = True

	for mod in mods:
		try:
			if st2:
				sublime_plugin.reload_plugin(modpath(mod))
			else:
				sublime_plugin.reload_plugin(modname(mod))
		except Exception:
			print("GoSublime: import(%s) failed: %s" % (mod, traceback.format_exc()))
	sys.dont_write_bytecode = dwb

	sublime.set_timeout(init_mods, 0)

def init_mods():
	global start

	for mod in mods:
		try:
			if st2:
				m = sys.modules[mod]
			else:
				m = sys.modules[modname(mod)]
			m.gs_init()
		except Exception:
			print("GoSublime: init(%s) failed: %s" % (mod, traceback.format_exc()))

	dur = time.time() - start
	print('GoSublime: %d modules loaded in %0.3fs' % (len(mods), dur))

def modpath(mod):
	return os.path.join(dist_dir, 'gosubl', '%s.py' % mod)

def modname(mod):
	return 'gosubl.%s' % mod

def plugin_loaded():
	global gs_plugin_loaded_called
	if gs_plugin_loaded_called:
		return
	gs_plugin_loaded_called = True

	sublime.set_timeout(import_mods, 0)

if st2:
	plugin_loaded()
