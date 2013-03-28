import os
import sublime
import sys

st2 = (sys.version_info[0] == 2)
dist_dir = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, dist_dir)


def plugin_loaded():
	from gosubl import about
	from gosubl import gs
	from gosubl import mg9

	gs.gs_init()
	mg9.gs_init()

	gs.set_attr('about.version', about.VERSION)
	gs.set_attr('about.ann', about.ANN)

if st2:
	sublime.set_timeout(plugin_loaded, 0)
