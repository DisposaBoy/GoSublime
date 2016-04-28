from gosubl import about
from gosubl import gs
from gosubl import gsq
from gosubl import gsshell
from gosubl import mg9
from gosubl import sh
import datetime
import json
import os
import re
import shlex
import string
import sublime
import sublime_plugin
import uuid
import webbrowser

DOMAIN = "9o"
AC_OPTS = sublime.INHIBIT_WORD_COMPLETIONS | sublime.INHIBIT_EXPLICIT_COMPLETIONS
SPLIT_FN_POS_PAT = re.compile(r'(.+?)(?:[:](\d+))?(?:[:](\d+))?$')
URL_SCHEME_PAT = re.compile(r'^[\w.+-]+://')
URL_PATH_PAT = re.compile(r'^(?:[\w.+-]+://|(?:www|(?:\w+\.)*(?:golang|pkgdoc|gosublime)\.org))')
HIST_EXPAND_PAT = re.compile(r'^(\^+)\s*(\d+)$')

HOURGLASS = u'\u231B'

DEFAULT_COMMANDS = [
	'help',
	'run',
	'build',
	'replay',
	'clear',
	'tskill',
	'tskill replay',
	'tskill go',
	'go',
	'go build',
	'go clean',
	'go doc',
	'go env',
	'go fix',
	'go fmt',
	'go get',
	'go install',
	'go list',
	'go run',
	'go test',
	'go tool',
	'go version',
	'go vet',
	'go help',
	'settings',
	'env',
	'share',
	'hist',
	'hist erase',
	'cd',
]
DEFAULT_CL = [(s, s+' ') for s in DEFAULT_COMMANDS]

stash = {}
tid_alias = {}

def active_wd(win=None):
	_, v = gs.win_view(win=win)
	return gs.basedir_or_cwd(v.file_name() if v else '')

def _hkey(wd):
	name = gs.setting("9o_instance")
	if name:
		wd = name
	return '9o.hist.%s' % wd

def _wdid(wd):
	name = gs.setting("9o_instance")
	if name:
		return name
	return '9o://%s' % wd


class EV(sublime_plugin.EventListener):
	def on_query_completions(self, view, prefix, locations):
		pos = gs.sel(view).begin()
		if view.score_selector(pos, 'text.9o') == 0:
			return []

		cl = set()

		hkey = _hkey(view.settings().get('9o.wd', ''))
		cl.update((k, k+' ') for k in gs.dval(gs.aso().get(hkey), []))
		cl.update((k, k+' ') for k in aliases())
		cl.update((k, k+' ') for k in builtins())
		cl.update(DEFAULT_CL)

		return ([cl_esc(e) for e in sorted(cl)], AC_OPTS)

def cl_esc(e):
	return (e[0], e[1].replace('$', '\\$'))

class Gs9oBuildCommand(sublime_plugin.WindowCommand):
	def is_enabled(self):
		view = gs.active_valid_go_view(self.window)
		return view is not None

	def run(self):
		view = self.window.active_view()
		args = {'run': gs.setting('build_command', ['^1'])} if gs.is_pkg_view(view) else {}
		view.run_command('gs9o_open', args)

class Gs9oInsertLineCommand(sublime_plugin.TextCommand):
	def run(self, edit, after=True):
		insln = lambda: self.view.insert(edit, gs.sel(self.view).begin(), "\n")
		if after:
			self.view.run_command("move_to", {"to": "hardeol"})
			insln()
		else:
			self.view.run_command("move_to", {"to": "hardbol"})
			insln()
			self.view.run_command("move", {"by": "lines", "forward": False})


class Gs9oMoveHist(sublime_plugin.TextCommand):
	def run(self, edit, up):
		view = self.view
		pos = gs.sel(view).begin()
		if view.score_selector(pos, 'prompt.9o') <= 0:
			return

		aso = gs.aso()
		vs = view.settings()
		wd = vs.get('9o.wd')
		hkey = _hkey(wd)
		hist = [s for s in gs.dval(aso.get(hkey), []) if s.strip()]
		if not hist:
			return

		r = view.extract_scope(pos)
		cmd = view.substr(r).strip('#').strip()
		try:
			idx = hist.index(cmd) + (-1 if up else 1)
			found = True
		except Exception:
			idx = -1
			found = False

		if cmd and not found:
			hist.append(cmd)
			aso.set(hkey, hist)
			gs.save_aso()

		if idx >= 0 and idx < len(hist):
			cmd = hist[idx]
		elif up:
			if not found:
				cmd = hist[-1]
		else:
			cmd = ''

		view.replace(edit, r, '# %s \n' % cmd)
		n = view.line(r.begin()).end()
		view.sel().clear()
		view.sel().add(sublime.Region(n, n))

class Gs9oInitCommand(sublime_plugin.TextCommand):
	def run(self, edit, wd=None):
		v = self.view
		vs = v.settings()

		if not wd:
			wd = vs.get('9o.wd', active_wd(win=v.window()))

		was_empty = v.size() == 0
		s = '[ %s ] # \n' % gs.simple_fn(wd).replace('#', '~')

		if was_empty:
			v.insert(edit, 0, 'GoSublime %s 9o: type `help` for help and command documentation\n\n' % about.VERSION)

		if was_empty or v.substr(v.size()-1) == '\n':
			v.insert(edit, v.size(), s)
		else:
			v.insert(edit, v.size(), '\n'+s)

		v.sel().clear()
		n = v.size()-1
		v.sel().add(sublime.Region(n, n))

		opts = {
			"rulers": [],
			"fold_buttons": True,
			"fade_fold_buttons": False,
			"gutter": True,
			"margin": 0,
			# pad mostly so the completion menu shows on the first line
			"line_padding_top": 1,
			"line_padding_bottom": 1,
			"tab_size": 2,
			"word_wrap": True,
			"indent_subsequent_lines": True,
			"line_numbers": False,
			"auto_complete": True,
			"auto_complete_selector": "text",
			"highlight_line": True,
			"draw_indent_guides": True,
			"scroll_past_end": True,
			"indent_guide_options": ["draw_normal", "draw_active"],
			"word_separators": "./\\()\"'-:,.;<>~!@#$%&*|+=[]{}`~?",
		}
		opts.update(gs.setting('9o_settings'))

		for opt in opts:
			vs.set(opt, opts[opt])

		vs.set("9o", True)
		vs.set("9o.wd", wd)

		color_scheme = gs.setting("9o_color_scheme", "")
		if color_scheme:
			if color_scheme == "default":
				vs.erase("color_scheme")
			else:
				vs.set("color_scheme", color_scheme)
		else:
			vs.set("color_scheme", "")

		v.set_syntax_file(gs.tm_path('9o'))

		if was_empty:
			v.show(0)
		else:
			v.show(v.size()-1)

		os.chdir(wd)

class Gs9oOpenCommand(sublime_plugin.TextCommand):
	def run(self, edit, wd=None, run=[], save_hist=False, focus_view=True):
		self.view.window().run_command('gs9o_win_open', {
			'wd': wd,
			'run': run,
			'save_hist': save_hist,
			'focus_view': focus_view,
		})

class Gs9oWinOpenCommand(sublime_plugin.WindowCommand):
	def run(self, wd=None, run=[], save_hist=False, focus_view=True):
		win = self.window
		wid = win.id()
		if not wd:
			wd = active_wd(win=win)

		id = _wdid(wd)
		st = stash.setdefault(wid, {})
		v = st.get(id)
		if v is None:
			v = win.get_output_panel(id)
			st[id] = v

		win.run_command("show_panel", {"panel": ("output.%s" % id)})

		if focus_view:
			win.focus_view(v)

		v.run_command('gs9o_init', {'wd': wd})

		if run:
			v.run_command('gs9o_paste_exec', {'cmd': ' '.join(run), 'save_hist': save_hist})

class Gs9oPasteExecCommand(sublime_plugin.TextCommand):
	def run(self, edit, cmd, save_hist=False):
		view = self.view
		view.insert(edit, view.line(view.size()-1).end(), cmd)
		view.sel().clear()
		view.sel().add(view.line(view.size()-1).end())
		view.run_command('gs9o_exec', {'save_hist': save_hist})

class Gs9oOpenSelectionCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		pos = gs.sel(self.view).begin()
		return self.view.score_selector(pos, 'text.9o') > 0

	def run(self, edit):
		actions = []
		v = self.view
		sel = gs.sel(v)
		if (sel.end() - sel.begin()) == 0:
			pos = sel.begin()
			inscope = lambda p: v.score_selector(p, 'path.9o') > 0
			if inscope(pos):
				actions.append(v.substr(v.extract_scope(pos)))
			else:
				pos -= 1
				if inscope(pos):
					actions.append(v.substr(v.extract_scope(pos)))
				else:
					line = v.line(pos)
					for cr in v.find_by_selector('path.9o'):
						if line.contains(cr):
							actions.append(v.substr(cr))
		else:
			actions.append(v.substr(sel))

		act_on(v, actions)

def act_on(view, actions):
	for a in actions:
		if act_on_path(view, a):
			break

def act_on_path(view, path):
	row = 0
	col = 0

	m = gs.VFN_ID_PAT.match(path)
	if m:
		path = 'gs.view://%s' % m.group(1)
		m2 = gs.ROWCOL_PAT.match(m.group(2))
		if m2:
			row = int(m2.group(1))-1 if m2.group(1) else 0
			col = int(m2.group(2))-1 if m2.group(2) else 0
	else:
		if URL_PATH_PAT.match(path):
			if path.lower().startswith('gs.packages://'):
				path = os.path.join(gs.packages_dir(), path[14:])
			else:
				try:
					if not URL_SCHEME_PAT.match(path):
						path = 'http://%s' % path
					gs.notify(DOMAIN, 'open url: %s' % path)
					webbrowser.open_new_tab(path)
					return True
				except Exception:
					gs.error_traceback(DOMAIN)

				return False

		wd = view.settings().get('9o.wd') or active_wd()
		m = SPLIT_FN_POS_PAT.match(path)
		path = gs.apath((m.group(1) if m else path), wd)
		row = max(0, int(m.group(2))-1 if (m and m.group(2)) else 0)
		col = max(0, int(m.group(3))-1 if (m and m.group(3)) else 0)

	if m or os.path.exists(path):
		gs.focus(path, row, col, win=view.window())
		return True
	else:
		gs.notify(DOMAIN, "Invalid path `%s'" % path)

	return False


def _exparg(s, m):
	s = string.Template(s).safe_substitute(m)
	s = os.path.expanduser(s)
	return s

class Gs9oExecCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		pos = gs.sel(self.view).begin()
		return self.view.score_selector(pos, 'text.9o') > 0

	def run(self, edit, save_hist=False):
		view = self.view
		pos = gs.sel(view).begin()
		line = view.line(pos)
		wd = view.settings().get('9o.wd')

		try:
			os.chdir(wd)
		except Exception:
			gs.error_traceback(DOMAIN)

		ln = view.substr(line).split('#', 1)
		if len(ln) == 2:
			cmd = ln[1].strip()
			if cmd:
				vs = view.settings()
				aso = gs.aso()
				hkey = _hkey(wd)
				hist = gs.dval(aso.get(hkey), [])

				m = HIST_EXPAND_PAT.match(cmd)
				if m:
					pfx = m.group(1)
					hl = len(hist)
					idx = hl - int(m.group(2))
					cmd = ''
					if idx >= 0 and idx < hl:
						cmd = hist[idx]

					if pfx == '^' or not cmd:
						view.replace(edit, line, ('%s# %s' % (ln[0], cmd)))
						return
				elif save_hist:
					try:
						hist.remove(cmd)
					except ValueError:
						pass
					hist.append(cmd)
					aso.set(hkey, hist)
					gs.save_aso()

			if not cmd:
				view.run_command('gs9o_init')
				return

			view.replace(edit, line, (u'[ `%s` %s ]' % (cmd, HOURGLASS)))
			rkey = '9o.exec.%s' % uuid.uuid4()
			view.add_regions(rkey, [sublime.Region(line.begin(), view.size())], '')
			view.run_command('gs9o_init')

			nv = sh.env()
			anv = nv.copy()
			seen = {}
			am = aliases()
			while True:
				cli = cmd.split(' ', 1)
				nm = cli[0]
				if not nm:
					break

				ag = cli[1].strip() if len(cli) == 2 else ''

				alias = am.get(nm, '')
				if not alias:
					break

				if alias in seen:
					gs.error(DOMAIN, 'recursive alias detected: `%s`' % alias)
					break

				seen[alias] = True
				anv['_args'] = ag
				cmd = string.Template(alias).safe_substitute(anv)

			if nm != 'sh':
				f = builtins().get(nm)
				if f:
					args = []
					if ag:
						args = [_exparg(s, nv) for s in shlex.split(gs.astr(ag))]

					f(view, edit, args, wd, rkey)
					return

			if nm == 'sh':
				args = sh.cmd(ag)
			else:
				args = sh.cmd(cmd)

			cmd_sh(view, edit, args, wd, rkey)
		else:
			view.insert(edit, gs.sel(view).begin(), '\n')

class Gs9oPushOutput(sublime_plugin.TextCommand):
	def run(self, edit, rkey, output, hourglass_repl=''):
		view = self.view
		output = '\t%s' % gs.ustr(output).strip().replace('\r', '').replace('\n', '\n\t')
		regions = view.get_regions(rkey)
		if regions:
			line = view.line(regions[0].begin())
			lsrc = view.substr(line).replace(HOURGLASS, (hourglass_repl or '| done'))
			view.replace(edit, line, lsrc)
			r = line
			if output.strip():
				line = view.line(regions[0].begin())
				view.insert(edit, line.end(), '\n%s' % output)
				r = view.get_regions(rkey)[0]
		else:
			n = view.size()
			view.insert(edit, n, '\n%s' % output)
			r = sublime.Region(n, view.size())

		if gs.setting('9o_show_end') is True:
			view.show(r.end())
		else:
			view.show(r.begin())

class Gs9oRunManyCommand(sublime_plugin.TextCommand):
	def run(self, edit, wd=None, commands=[], save_hist=False, focus_view=False):
		for run in commands:
			self.view.run_command("gs9o_open", {
				'run': run,
				'wd': wd,
				'save_hist': save_hist,
				'focus_view': focus_view,
			})

def aliases():
	return gs.setting('9o_aliases', {}).copy()

def builtins():
	m = gs.gs9o.copy()

	g = globals()
	for k, v in g.items():
		if k.startswith('cmd_'):
			k = k[4:].replace('_', '-')
			if k and k not in m:
				m[k] = v

	return m

def push_output(view, rkey, output, hourglass_repl=''):
	def f():
		view.run_command('gs9o_push_output', {
			'rkey': rkey,
			'output': output,
			'hourglass_repl': hourglass_repl,
		})

	sublime.set_timeout(f, 0)

def _save_all(win, wd):
	if gs.setting('autosave') is True and win is not None:
		for v in win.views():
			try:
				fn = v.file_name()
				if fn and v.is_dirty() and fn.endswith('.go') and os.path.dirname(fn) == wd:
					v.run_command('gs_fmt_save')
			except Exception:
				gs.error_traceback(DOMAIN)

def _9_begin_call(name, view, edit, args, wd, rkey, cid):
	dmn = '%s: 9 %s' % (DOMAIN, name)
	msg = '[ %s ] # 9 %s' % (gs.simple_fn(wd), ' '.join(args))
	if not cid:
		cid = '9%s-%s' % (name, uuid.uuid4())
	tid = gs.begin(dmn, msg, set_status=False, cancel=lambda: mg9.acall('kill', {'cid': cid}, None))
	tid_alias['%s-%s' % (name, wd)] = tid

	def cb(res, err):
		out = '\n'.join(s for s in (res.get('out'), res.get('err'), err) if s)

		tmp_fn = res.get('tmpFn')
		fn = res.get('fn')
		if fn and tmp_fn:
			bfn = os.path.basename(tmp_fn)
			repls = [
				'./%s' % bfn,
				'.\\%s' % bfn,
				tmp_fn,
			]
			for s in repls:
				out = out.replace(s, fn)

		def f():
			gs.end(tid)
			push_output(view, rkey, out, hourglass_repl='| done: %s' % res.get('dur', ''))

		sublime.set_timeout(f, 0)

	return cid, cb

def cmd_margo_reinstall(view, edit, args, wd, rkey):
	def cb():
		gs.del_attr(mg9._inst_name())
		out = mg9.install('', True, True)
		gs.notify(DOMAIN, 'MarGo re-installed done')
		push_output(view, rkey, out)

	gsq.launch(DOMAIN, cb)

def cmd_echo(view, edit, args, wd, rkey):
	push_output(view, rkey, ' '.join(args))

def cmd_which(view, edit, args, wd, rkey):
	l = []
	am = aliases()
	m = builtins()

	if not args:
		args = []
		args.extend(sorted(m.keys()))
		args.extend(sorted(am.keys()))

	fm = '%{0}s: %s'.format(max(len(s) for s in args))

	for k in args:
		if k == 'sh':
			v = '9o builtin: %s' % sh.cmd('${CMD}')
		elif k in ('go'):
			v = '9o builtin: %s' % sh.which(k)
		elif k in m:
			v = '9o builtin'
		elif k in am:
			v = '9o alias: `%s`' % am[k]
		else:
			v = sh.which(k)

		l.append(fm % (k, v))

	push_output(view, rkey, '\n'.join(l))

def cmd_cd(view, edit, args, wd, rkey):
	try:
		if args:
			wd = args[0]
			wd = string.Template(wd).safe_substitute(sh.env())
			wd = os.path.expanduser(wd)
			wd = os.path.abspath(wd)
		else:
			fn = view.window().active_view().file_name()
			if fn:
				wd = os.path.dirname(fn)

		os.chdir(wd)
	except Exception as ex:
		push_output(view, rkey, 'Cannot chdir: %s' % ex)
		return

	push_output(view, rkey, '')
	view.run_command('gs9o_init', {'wd': wd})

def cmd_reset(view, edit, args, wd, rkey):
	push_output(view, rkey, '')
	view.erase(edit, sublime.Region(0, view.size()))
	view.run_command('gs9o_init')

def cmd_clear(view, edit, args, wd, rkey):
	cmd_reset(view, edit, args, wd, rkey)

def cmd_go(view, edit, args, wd, rkey):
	_save_all(view.window(), wd)

	cid, cb = _9_begin_call('go', view, edit, args, wd, rkey, '9go-%s' % wd)
	a = {
		'cid': cid,
		'env': sh.env(),
		'cwd': wd,
		'cmd': {
			'name': 'go',
			'args': args,
		}
	}
	sublime.set_timeout(lambda: mg9.acall('sh', a, cb), 0)

def cmd_cancel_replay(view, edit, args, wd, rkey):
	cid = ''
	av = None
	win = view.window()
	if win is not None:
		av = win.active_view()

		if av is not None and not av.file_name():
			cid = '9replayv-%s' % av.id()

	if not cid:
		cid = '9replay-%s' % wd

	mg9.acall('kill', {'cid': cid}, None)
	push_output(view, rkey, '')

def cmd_sh(view, edit, args, wd, rkey):
	cid, cb = _9_begin_call('sh', view, edit, args, wd, rkey, '')
	a = {
		'cid': cid,
		'env': sh.env(),
		'cwd': wd,
		'cmd': {
			'name': args[0],
			'args': args[1:],
		}
	}
	sublime.set_timeout(lambda: mg9.acall('sh', a, cb), 0)

def cmd_share(view, edit, args, wd, rkey):
	av = gs.active_valid_go_view(win=view.window())
	if av is None:
		push_output(view, rkey, 'not sharing non-go src')
		return

	def f(res, err):
		s = '%s\n%s' % (err, res.get('url', ''))
		push_output(view, rkey, s.strip())

	mg9.share(gs.view_src(view.window().active_view()), f)

def cmd_help(view, edit, args, wd, rkey):
	gs.focus(gs.dist_path('9o.md'))
	push_output(view, rkey, '')

def cmd_run(view, edit, args, wd, rkey):
	cmd_9(view, edit, gs.lst('run', args), wd, rkey)

def cmd_replay(view, edit, args, wd, rkey):
	cmd_9(view, edit, gs.lst('replay', args), wd, rkey)

def cmd_build(view, edit, args, wd, rkey):
	cmd_9(view, edit, gs.lst('build', args), wd, rkey)

def cmd_9(view, edit, args, wd, rkey):
	if len(args) == 0 or args[0] not in ('run', 'replay', 'build'):
		push_output(view, rkey, ('9: invalid args %s' % args))
		return

	subcmd = args[0]
	cid = ''
	if subcmd == 'replay':
		cid = '9replay-%s' % wd
	cid, cb = _9_begin_call(subcmd, view, edit, args, wd, rkey, cid)

	a = {
		'cid': cid,
		'env': sh.env(),
		'dir': wd,
		'args': args[1:],
		'build_only': (subcmd == 'build'),
	}

	win = view.window()
	if win is not None:
		av = win.active_view()
		if av is not None:
			fn = av.file_name()
			if fn:
				_save_all(win, wd)
			else:
				if gs.is_go_source_view(av, False):
					a['fn'] = gs.view_fn(av)
					a['src'] = av.substr(sublime.Region(0, av.size()))

	sublime.set_timeout(lambda: mg9.acall('play', a, cb), 0)

def cmd_tskill(view, edit, args, wd, rkey):
	if len(args) == 0:
		sublime.set_timeout(lambda: sublime.active_window().run_command("gs_show_tasks"), 0)
		push_output(view, rkey, '')
		return

	l = []
	for tid in args:
		tid = tid.lstrip('#')
		tid = tid_alias.get('%s-%s' % (tid, wd), tid)
		l.append('kill %s: %s' % (tid, ('yes' if gs.cancel_task(tid) else 'no')))

	push_output(view, rkey, '\n'.join(l))

def _env_settings(d, view, edit, args, wd, rkey):
	if len(args) > 0:
		m = {}
		for k in args:
			m[k] = d.get(k)
	else:
		m = d

	s = '\n'.join((
		'Default Settings file: gs.packages://GoSublime/GoSublime.sublime-settings (do not edit this file)',
		'User settings file: gs.packages://User/GoSublime.sublime-settings (add/change your settings here)',
		json.dumps(m, sort_keys=True, indent=4),
	))
	push_output(view, rkey, s)

def cmd_settings(view, edit, args, wd, rkey):
	_env_settings(gs.settings_dict(), view, edit, args, wd, rkey)

def cmd_env(view, edit, args, wd, rkey):
	_env_settings(sh.env(), view, edit, args, wd, rkey)

def cmd_hist(view, edit, args, wd, rkey):
	aso = gs.aso()
	hkey = _hkey(wd)

	s = 'hist: invalid args: %s' % args

	if len(args) == 0:
		hist = gs.dval(aso.get(hkey), [])
		hist.reverse()
		hlen = len(hist)
		s = '\n'.join('^%d: %s' % (i+1, v) for i,v in enumerate(hist))
	elif len(args) == 1:
		if args[0] == 'erase':
			aso.erase(hkey)
			gs.save_aso()
			s = ''

	push_output(view, rkey, s)
