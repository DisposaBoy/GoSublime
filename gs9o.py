import datetime
import gscommon as gs
import gsshell
import json
import mg9
import os
import re
import shlex
import sublime
import sublime_plugin
import uuid
import webbrowser

DOMAIN = "9o"
AC_OPTS = sublime.INHIBIT_WORD_COMPLETIONS | sublime.INHIBIT_EXPLICIT_COMPLETIONS
SPLIT_FN_POS_PAT = re.compile(r'(.+?)(?:[:](\d+))?(?:[:](\d+))?$')
URL_SCHEME_PAT = re.compile(r'^[\w.+-]+://')
URL_PATH_PAT = re.compile(r'^(?:[\w.+-]+://|(?:www|(?:\w+\.)*(?:golang|pkgdoc|gosublime)\.org))')

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
]
DEFAULT_CL = [(s, s) for s in DEFAULT_COMMANDS]

if not gs.checked(DOMAIN, '_vars'):
	stash = {}
	tid_alias = {}

def active_wd(win=None):
	_, v = gs.win_view(win=win)
	return gs.basedir_or_cwd(v.file_name() if v else '')

def wdid(wd):
	return '9o://%s' % wd


class EV(sublime_plugin.EventListener):
	def on_query_completions(self, view, prefix, locations):
		pos = gs.sel(view).begin()
		if view.score_selector(pos, 'text.9o') == 0:
			return []
		cl = []
		cl.extend(DEFAULT_CL)
		return (cl, AC_OPTS)

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


class Gs9oInitCommand(sublime_plugin.TextCommand):
	def run(self, edit, wd=None):
		v = self.view
		vs = v.settings()

		if not wd:
			wd = vs.get('9o.wd', active_wd(win=v.window()))

		was_empty = v.size() == 0
		s = '[ %s ] # \n' % wd

		if was_empty:
			v.insert(edit, 0, 'GoSublime %s 9o: type `help` for help and command documentation\n\n' % mg9.REV)

		if was_empty or v.substr(v.size()-1) == '\n':
			v.insert(edit, v.size(), s)
		else:
			v.insert(edit, v.size(), '\n'+s)

		v.sel().clear()
		n = v.size()-1
		v.sel().add(sublime.Region(n, n))
		vs.set("9o.wd", wd)
		vs.set("rulers", [])
		vs.set("fold_buttons", True)
		vs.set("fade_fold_buttons", False)
		vs.set("gutter", True)
		vs.set("margin", 0)
		# pad mostly so the completion menu shows on the first line
		vs.set("line_padding_top", 1)
		vs.set("line_padding_bottom", 1)
		vs.set("tab_size", 2)
		vs.set("word_wrap", True)
		vs.set("indent_subsequent_lines", True)
		vs.set("line_numbers", False)
		vs.set("auto_complete", True)
		vs.set("auto_complete_selector", "text")
		vs.set("highlight_line", True)
		vs.set("draw_indent_guides", True)
		vs.set("indent_guide_options", ["draw_normal", "draw_active"])
		v.set_syntax_file('Packages/GoSublime/9o.tmLanguage')

		if was_empty:
			v.show(0)
		else:
			v.show(v.size()-1)

class Gs9oOpenV(sublime_plugin.TextCommand):
	def run(self, edit, wd=None, run=[]):
		self.view.run_command('gs9o_open', {'wd': wd, 'run': run})

class Gs9oOpenCommand(sublime_plugin.TextCommand):
	def run(self, edit, wd=None, run=[]):
		win = self.view.window()
		wid = win.id()
		if not wd:
			wd = active_wd(win=win)

		id = wdid(wd)
		st = stash.setdefault(wid, {})
		v = st.get(id)
		if v is None:
			v = win.get_output_panel(id)
			st[id] = v

		win.run_command("show_panel", {"panel": ("output.%s" % id)})
		win.focus_view(v)
		v.run_command('gs9o_init', {'wd': wd})

		if run:
			cmd = ' '.join(run)
			v.insert(edit, v.line(v.size()-1).end(), cmd)
			v.sel().clear()
			v.sel().add(v.line(v.size()-1).end())
			v.run_command('gs9o_exec')

class Gs9oOpenSelectionCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		pos = gs.sel(self.view).begin()
		return self.view.score_selector(pos, 'text.9o') > 0

	def run(self, edit):
		v = self.view
		pos = gs.sel(v).begin()
		inscope = lambda p: v.score_selector(p, 'path.9o') > 0
		if not inscope(pos):
			pos -= 1
			if not inscope(pos):
				return

		path = v.substr(v.extract_scope(pos))
		if URL_PATH_PAT.match(path):
			if path.lower().startswith('gs.packages://'):
				path = os.path.join(sublime.packages_path(), path[14:])
			else:
				try:
					if not URL_SCHEME_PAT.match(path):
						path = 'http://%s' % path
					gs.notify(DOMAIN, 'open url: %s' % path)
					webbrowser.open_new_tab(path)
				except Exception:
					gs.error_traceback(DOMAIN)

				return

		wd = v.settings().get('9o.wd') or active_wd()
		m = SPLIT_FN_POS_PAT.match(path)
		path = gs.apath((m.group(1) if m else path), wd)
		row = max(0, int(m.group(2))-1 if (m and m.group(2)) else 0)
		col = max(0, int(m.group(3))-1 if (m and m.group(3)) else 0)

		if os.path.exists(path):
			gs.focus(path, row, col, win=self.view.window())
		else:
			gs.notify(DOMAIN, "Invalid path `%s'" % path)

class Gs9oExecCommand(sublime_plugin.TextCommand):
	def is_enabled(self):
		pos = gs.sel(self.view).begin()
		return self.view.score_selector(pos, 'text.9o') > 0

	def run(self, edit):
		view = self.view
		pos = gs.sel(view).begin()
		line = view.line(pos)
		wd = view.settings().get('9o.wd')

		ln = view.substr(line).split('#', 1)
		if len(ln) == 2:
			cmd = ln[1].strip()
			if cmd:
				vs = view.settings()
				lc_key = '%s.last_command' % DOMAIN
				if cmd.startswith('#'):
					rep = vs.get(lc_key, '')
					if rep:
						view.replace(edit, line, ('%s# %s %s' % (ln[0], rep, cmd.lstrip('# \t'))))
					return
				elif cmd == '!!':
					cmd = vs.get(lc_key, '')
				else:
					vs.set(lc_key, cmd)

			if not cmd:
				view.run_command('gs9o_init')
				return

			view.replace(edit, line, (u'[ %s %s ]' % (cmd, HOURGLASS)))
			rkey = '9o.exec.%s' % uuid.uuid4()
			view.add_regions(rkey, [sublime.Region(line.begin(), view.size())], '')
			view.run_command('gs9o_init')

			cli = cmd.split(' ', 1)

			# todo: move this into margo
			if cli[0] == 'sh':
				def on_done(c):
					out = gs.ustr('\n'.join(c.consume_outq()))
					sublime.set_timeout(lambda: push_output(view, rkey, out), 0)

				c = gsshell.Command(cmd=cli[1], shell=True, cwd=wd)
				c.on_done = on_done
				c.start()
				return

			f = globals().get('cmd_%s' % cli[0])
			if f:
				args = shlex.split(gs.astr(cli[1])) if len(cli) == 2 else []
				f(view, edit, args, wd, rkey)
			else:
				push_output(view, rkey, 'Invalid command %s' % cli)
		else:
			view.insert(edit, gs.sel(view).begin(), '\n')

def push_output(view, rkey, out, hourglass_repl=''):
	out = '\t%s' % out.strip().replace('\r', '').replace('\n', '\n\t')
	edit = view.begin_edit()
	try:
		regions = view.get_regions(rkey)
		if regions:
			line = view.line(regions[0].begin())
			lsrc = view.substr(line).replace(HOURGLASS, (hourglass_repl or '| done'))
			view.replace(edit, line, lsrc)
			if out.strip():
				line = view.line(regions[0].begin())
				view.insert(edit, line.end(), '\n%s' % out)
		else:
			view.insert(edit, view.size(), '\n%s' % out)
	finally:
		view.end_edit(edit)

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
	msg = '[ %s ] # 9 %s' % (wd, ' '.join(args))
	if not cid:
		cid = '9%s-%s' % (name, uuid.uuid4())
	tid = gs.begin(dmn, msg, set_status=False, cancel=lambda: mg9.acall('kill', {'cid': cid}, None))
	tid_alias['%s-%s' % (name, wd)] = tid

	def cb(res, err):
		out = '\n'.join(s for s in (res.get('out'), res.get('err'), err) if s)
		def f():
			gs.end(tid)
			push_output(view, rkey, out, hourglass_repl='| done: %s' % res.get('dur', ''))

		sublime.set_timeout(f, 0)

	return cid, cb

def cmd_reset(view, edit, args, wd, rkey):
	view.erase(edit, sublime.Region(0, view.size()))
	view.run_command('gs9o_init')

def cmd_clear(view, edit, args, wd, rkey):
	cmd_reset(view, edit, args, wd, rkey)

def cmd_go(view, edit, args, wd, rkey):
	_save_all(view.window(), wd)

	cid, cb = _9_begin_call('go', view, edit, args, wd, rkey, '9go-%s' % wd)
	a = {
		'cid': cid,
		'env': gs.env(),
		'cwd': wd,
		'cmd': {
			'name': 'go',
			'args': args,
		}
	}
	mg9.acall('sh', a, cb)

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
		'env': gs.env(),
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
					a['src'] = av.substr(sublime.Region(0, av.size()))

	mg9.acall('play', a, cb)

def cmd_tskill(view, edit, args, wd, rkey):
	if len(args) > 0:
		l = []
		for tid in args:
			tid = tid.lstrip('#')
			tid = tid_alias.get('%s-%s' % (tid, wd), tid)
			l.append('kill %s: %s' % (tid, ('yes' if gs.cancel_task(tid) else 'no')))

		push_output(view, rkey, '\n'.join(l))
		return

	try:
		now = datetime.datetime.now().replace(microsecond=0)
		with gs.sm_lck:
			tasks = sorted(gs.sm_tasks.iteritems())

		l = []
		for tid, t in tasks:
			if t['cancel']:
				pfx = '#%s' % tid
			else:
				pfx = '(uninterruptible)'

			l.append('%s %s %s: %s' % (pfx, (now - t['start'].replace(microsecond=0)), t['domain'], t['message']))

		s = '\n'.join(l)
	except Exception as ex:
		gs.error_traceback(DOMAIN)
		s = 'Error: %s' % ex
	push_output(view, rkey, s)

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
	_env_settings(gs.env(), view, edit, args, wd, rkey)



