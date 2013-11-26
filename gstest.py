from gosubl import gs
from gosubl import mg9
import os
import re
import sublime
import sublime_plugin

DOMAIN = 'GsTest'

TEST_PAT = re.compile(r'^((Test|Example|Benchmark)\w*)')
ALL_TESTS_TITLE = 'Run all tests and examples'
GOCHECK_PATTERN = '\(\*.*?\)\.%s$'

class GsTestCommand(sublime_plugin.WindowCommand):
	def is_enabled(self):
		return gs.is_go_source_view(self.window.active_view())

	def run(self):
		last_test = get_last_test()
		clear_quick_panel_on_test_run = gs.settings_obj().get("clear_quick_panel_on_test_run")

		def f(res, err):

			if err:
				gs.notify(DOMAIN, err)
				return

			# mats = {}
			args = {}
			gocheck = {}
			decls = res.get('file_decls', [])
			decls.extend(res.get('pkg_decls', []))
			for d in decls:
				name = d['name']
				prefix, _ =  match_prefix_name(name)
				if prefix and d['kind'] == 'func'  and name != 'Test':
					if d['repr'] == '':
						# mats[True] = prefix
						args[name] = name
						gocheck[name] = False
					elif re.search(GOCHECK_PATTERN % name, d['repr']) and name != 'Test':
						# mats[True] = prefix
						args[name] = name
						gocheck[name] = True

			names = sorted(args.keys())
			ents = [ALL_TESTS_TITLE]
			if last_test != None and last_test.get('name') != ALL_TESTS_TITLE:
				ents.insert(0, last_test.get('name'))

			# TODO: What is the purpose of this for loop? mats.get(k) will never be True
			# which is what it is set at above (mats[True] = prefix)
			# for k in ['Test', 'Benchmark', 'Example']:
			# 	if mats.get(k):
			# 		s = 'Run %ss Only' % k
			# 		ents.append(s)
			# 		if k == 'Benchmark':
			# 			args[s] = ['-test.run=none', '-test.bench="%s.*"' % k]
			# 		else:
			# 			args[s] = ['-test.run="%s.*"' % k]

			for k in names:
				ents.append(k)
				if k.startswith('Benchmark'):
					if gocheck[k]:
						args[k] = ['-gocheck.b "^%s$"' % k]
					else:
						args[k] = ['-test.run=none', '-test.bench="^%s$"' % k]
				else:
					if gocheck[k]:
						args[k] = ['-gocheck.f "^%s$"' % k]
					else:
						args[k] = ['-test.run="^%s$"' % k]

			def cb(i, win):
				if i >= 0:
					view = win.active_view()
					a = args.get(ents[i], [])
					wd = os.path.dirname(view.file_name())
					gocheck_present = gocheck.get(ents[i], False)
					if len(a) == 0 and last_test != None and ents[i] != ALL_TESTS_TITLE:
						name = last_test.get('name')
						wd = last_test.get('path')
						gocheck_present = last_test.get('gocheck')
						if gocheck_present:
							if name.startswith('Benchmark'):
								a = ['-gocheck.b "^%s$"' % name]
							else:
								a = ['-gocheck.f "^%s$"' % name]
						else:
							if name.startswith('Benchmark'):
								a = ['-test.run=none', '-test.bench="^%s$"' % name]
							else:
								a = ['-test.run="^%s$"' % name]

					save_last_test(ents[i], wd, gocheck_present)
					append_extra_test_args(a, gocheck_present)
					if clear_quick_panel_on_test_run:
						view.run_command('gs9o_open', {'run': ["clear"], 'wd' : wd})	
					view.run_command('gs9o_open', {'run': gs.lst('go', 'test', a), 'wd': wd})

			gs.show_quick_panel(ents, cb)

		win, view = gs.win_view(None, self.window)
		if view is None:
			return

		vfn = gs.view_fn(view)
		src = gs.view_src(view)
		pkg_dir = ''
		if view.file_name():
			pkg_dir = os.path.dirname(view.file_name())

		mg9.declarations(vfn, src, pkg_dir, f)

def last_test_hkey():
	return '9o.last_test'

def get_last_test():
	return gs.aso().get(last_test_hkey())

def save_last_test(name, path, gocheck_present):
	gs.aso().set(last_test_hkey(), { "name" : name, "path" : path, "gocheck": gocheck_present})
	gs.save_aso()

def append_extra_test_args(a, gocheck_present):
	settings = gs.settings_obj()
	for arg in settings.get('extra_test_args'):
		a.append(arg)
	if gocheck_present:
		for arg in settings.get('extra_gocheck_args'):
			a.append(arg)

def match_prefix_name(s):
	m = TEST_PAT.match(s)
	return (m.group(2), m.group(1)) if m else ('', '')

def handle_action(view, action):
	fn = view.file_name()
	prefix, name = match_prefix_name(view.substr(view.word(gs.sel(view))))
	ok = prefix and fn and fn.endswith('_test.go')
	if ok:
		# TODO use decls to determine gocheck presence
		full_line = view.substr(view.full_line(gs.sel(view)))
		gocheck_present = re.search("func\s+\(\s*\w+\s+\*\w+\s*\)\s+%s\s*\(\s*\w+\s+\*\w+\s*\)" % name, full_line)
		if action == 'right-click':
			pat = '^%s.*' % prefix
		else:
			pat = '^%s$' % name

		if prefix == 'Benchmark':
			if gocheck_present:
				cmd = ['go', 'test', '-gocheck.b "%s"' % pat]
			else:
				cmd = ['go', 'test', '-test.run=none', '-test.bench="%s"' % pat]
		else:
			if gocheck_present:
				cmd = ['go', 'test', '-gocheck.f "%s"' % pat]
			else:		
				cmd = ['go', 'test', '-test.run="%s"' % pat]

		append_extra_test_args(cmd, gocheck_present)
		save_last_test(re.sub('[\^\$]','', pat), os.path.dirname(view.file_name()), gocheck_present)

		if gs.settings_obj().get("clear_quick_panel_on_test_run"):
			view.run_command('gs9o_open', {'run': ["clear"]})	

		view.run_command('gs9o_open', {'run': cmd})

	return ok
