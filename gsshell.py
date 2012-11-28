import sublime
import sublime_plugin
import gscommon as gs
import re
import os
import httplib
import hashlib
import threading
import Queue
import traceback
import subprocess
import time
import signal
import string

DOMAIN = "GsShell"
GO_RUN_PAT = re.compile(r'^go\s+(run|play)$', re.IGNORECASE)
GO_SHARE_PAT = re.compile(r'^go\s+share$', re.IGNORECASE)
GO_PLAY_PAT = re.compile(r'(\b)go\s+play(\b)', re.IGNORECASE)

class Prompt(object):
	def __init__(self, view, change_history=True):
		self.view = view
		self.panel = None
		self.subcommands = [
			'go run', 'go build', 'go clean', 'go fix',
			'go install', 'go test', 'go fmt', 'go vet', 'go tool',
			'go share', 'go play',
		]
		self.settings = sublime.load_settings('GoSublime-GsShell.sublime-settings')
		self.change_history = change_history

	def on_done(self, s, fmt_save=True):
		fn = self.view.file_name()
		win = self.view.window()
		if fn and win:
			basedir = os.path.dirname(fn)
			if fmt_save:
				for v in win.views():
					vfn = v.file_name()
					if vfn and v.is_dirty() and os.path.dirname(vfn) == basedir and vfn.endswith('.go'):
						v.run_command('gs_fmt_save')
		elif fmt_save:
			self.view.run_command('gs_fmt')


		# above we do some saves - thus creating a race so push this back to the end of the queue
		def cb(s):
			file_name = self.view.file_name() or ''
			s = GO_PLAY_PAT.sub(r'\1go run\2', s)
			s = s.strip()
			if s and s.lower() != "go" and self.change_history:
				hist = self.settings.get('cmd_hist')
				if not gs.is_a(hist, {}):
					hist = {}
				basedir = gs.basedir_or_cwd(file_name)
				hist[basedir] = [s] # todo: store a list of historical commands
				hst = {}
				for k in hist:
					# :|
					hst[gs.ustr(k)] = gs.ustr(hist[k])
				self.settings.set('cmd_hist', hst)
				sublime.save_settings('GoSublime-GsShell.sublime-settings')

			if GO_SHARE_PAT.match(s):
				s = ''
				host = "play.golang.org"
				warning = 'Are you sure you want to share this file. It will be public on %s' % host
				if not sublime.ok_cancel_dialog(warning):
					return

				try:
					c = httplib.HTTPConnection(host)
					src = gs.astr(self.view.substr(sublime.Region(0, self.view.size())))
					c.request('POST', '/share', src, {'User-Agent': 'GoSublime'})
					s = 'http://%s/p/%s' % (host, c.getresponse().read())
				except Exception as ex:
					s = 'Error: %s' % ex

				self.show_output(s, focus=True)
				return

			if GO_RUN_PAT.match(s):
				if not file_name:
					# todo: clean this up after the command runs
					err = ''
					tdir, _ = gs.temp_dir('play')
					file_name = hashlib.sha1(gs.view_fn(self.view) or 'a').hexdigest()
					file_name = os.path.join(tdir, ('%s.go' % file_name))
					try:
						with open(file_name, 'w') as f:
							src = gs.astr(self.view.substr(sublime.Region(0, self.view.size())))
							f.write(src)
					except Exception as ex:
						err = str(ex)

					if err:
						self.show_output('Error: %s' % err)
						return

				s = ['go', 'run', file_name]

			self.view.window().run_command("exec", { 'kill': True })
			if gs.is_a(s, []):
				use_shell = False
			else:
				use_shell = True
				s = [s]
			gs.println('running %s' % ' '.join(s))
			self.view.window().run_command("exec", {
				'shell': use_shell,
				'env': gs.env(),
				'cmd': s,
				'file_regex': '^(.+\.go):([0-9]+):(?:([0-9]+):)?\s*(.*)',
			})

		sublime.set_timeout(lambda: cb(s), 0)

	def show_output(self, s, focus=False):
		panel_name = DOMAIN+'-share'
		win = self.view.window()
		panel = win.get_output_panel(panel_name)
		edit = panel.begin_edit()
		try:
			panel.set_read_only(False)
			panel.sel().clear()
			panel.replace(edit, sublime.Region(0, panel.size()), s)
			panel.sel().add(sublime.Region(0, panel.size()))
			panel.set_read_only(True)
		finally:
			panel.end_edit(edit)
		print('%s output: %s' % (DOMAIN, s))
		self.view.window().run_command("show_panel", {"panel": "output.%s" % panel_name})
		if focus:
			sublime.set_timeout(lambda: win.focus_view(panel), 0)

	def on_change(self, s):
		if self.panel:
			size = self.view.size()
			if s.endswith('\t'):
				basedir = gs.basedir_or_cwd(self.view.file_name())
				lc = 'go '
				hist = self.settings.get('cmd_hist')
				if gs.is_a(hist, {}):
					hist = hist.get(basedir)
					if hist and gs.is_a(hist, []):
						lc = hist[-1]
				s = s.strip()
				if s and s not in ('', 'go'):
					l = []
					for i in self.subcommands:
						if i.startswith(s):
							l.append(i)
					if len(l) == 1:
						s = '%s ' % l[0]
				elif lc:
					s = '%s ' % lc
				edit = self.panel.begin_edit()
				try:
					self.panel.replace(edit, sublime.Region(0, self.panel.size()), s)
				finally:
					self.panel.end_edit(edit)

class GsShellCommand(sublime_plugin.WindowCommand):
	def is_enabled(self):
		view = gs.active_valid_go_view(self.window)
		return bool(view)

	def run(self, prompt="go ", run="", fmt_save=True):
		view = gs.active_valid_go_view(self.window)
		if not view:
			gs.notice(DOMAIN, "this not a source.go view")
			return

		run = run.strip()
		p = Prompt(view, run == "")
		if run:
			p.on_done(run, fmt_save)
		else:
			p.panel = self.window.show_input_panel("GsShell", prompt, p.on_done, p.on_change, None)


def command_on_output(c, line):
	c.outq().put(line)

def command_on_done(c):
	pass

def fix_env(env):
	e = {}
	for k,v in env.items():
		e[k] = str(v)
	return e

def fix_shell_cmd(shell, cmd):
	if not gs.is_a(cmd, []):
		cmd = [cmd]

	if shell:
		sh = gs.setting('shell')
		cmd_str = ' '.join(cmd)
		cmd_map = {'CMD': cmd_str}
		if sh:
			shell = False
			cmd = []
			for v in sh:
				if v:
					cmd.append(string.Template(v).safe_substitute(cmd_map))
		else:
			cmd = [cmd_str]

	return (shell, [gs.astr(v) for v in cmd])

def proc(cmd, shell=False, env={}, cwd=None, input=None, stdout=subprocess.PIPE, stderr=subprocess.PIPE, stdin=subprocess.PIPE, bufsize=0):
	env = gs.env(env)
	shell, cmd = fix_shell_cmd(shell, cmd)

	if input is not None:
		input = gs.astr(input)

	if cwd:
		try:
			os.makedirs(cwd)
		except Exception:
			pass
	else:
		# an empty string isn't a valid value so just always set it None
		cwd = None

	try:
		setsid = os.setsid
	except Exception:
		setsid = None

	opts = {
		'cmd': cmd,
		'shell': shell,
		'env': env,
		'input': input,
	}

	p = None
	err = ''
	try:
		p = subprocess.Popen(
			cmd,
			stdout=stdout,
			stderr=stderr,
			stdin=stdin,
			startupinfo=gs.STARTUP_INFO,
			shell=shell,
			env=env,
			cwd=cwd,
			preexec_fn=setsid,
			bufsize=bufsize
		)
	except Exception:
		err = 'Error running command %s: %s' % (cmd, gs.traceback())

	return (p, opts, err)

def run(cmd=[], shell=False, env={}, cwd=None, input=None, stderr=subprocess.STDOUT):
	out = u""
	err = u""
	exc = None

	try:
		p, opts, err = proc(cmd, input=input, shell=shell, stderr=stderr, env=env, cwd=cwd)
		if p:
			out, _ = p.communicate(input=opts.get('input'))
			out = gs.ustr(out) if out else u''
	except Exception as ex:
		err = u'Error communicating with command %s: %s' % (opts.get('cmd'), gs.traceback())
		exc = ex

	return (out, err, exc)

class CommandStdoutReader(threading.Thread):
	def __init__(self, c, stdout):
		super(CommandStdoutReader, self).__init__()
		self.daemon = True
		self.stdout = stdout
		self.c = c

	def run(self):
		try:
			while True:
				line = self.stdout.readline()

				if not line:
					self.c.close_stdout()
					break

				if not self.c.output_started:
					self.c.output_started = time.time()

				try:
					self.c.on_output(self.c, gs.ustr(line.rstrip('\r\n')))
				except Exception:
					gs.println(gs.traceback(DOMAIN))
		except Exception:
			gs.println(gs.traceback(DOMAIN))


class Command(threading.Thread):
	def __init__(self, cmd=[], shell=False, env={}, cwd=None):
		super(Command, self).__init__()
		self.daemon = True
		self.cancelled = False
		self.q = Queue.Queue()
		self.p = None
		self.x = None
		self.rcode = None
		self.started = 0
		self.output_started = 0
		self.ended = 0
		self.on_output = command_on_output
		self.env = fix_env(env)
		self.shell, self.cmd = fix_shell_cmd(shell, cmd)
		self.message = str(self.cmd)
		self.cwd = cwd if cwd else None
		self.on_done = command_on_done
		self.done = []

	def outq(self):
		return self.q

	def process(self):
		return self.p

	def exception(self):
		return self.x

	def return_code(self):
		return self.rcode

	def consume_outq(self):
		l = []
		try:
			while True:
				l.append(self.q.get_nowait())
		except Queue.Empty:
			pass
		return l

	def poll(self):
		if self.p:
			return self.p.poll()
		return False

	def cancel(self):
		if self.poll() is None:
			try:
				os.killpg(self.p.pid, signal.SIGTERM)
			except Exception:
				self.p.terminate()

			time.sleep(0.100)
			if not self.completed():
				time.sleep(0.500)
				if not self.completed():
					try:
						os.killpg(self.p.pid, signal.SIGKILL)
					except Exception:
						try:
							self.p.kill()
						except Exception:
							pass
					self.close_stdout()

		discarded = 0
		try:
			while True:
				self.q.get_nowait()
				discarded += 1
		except Queue.Empty:
			pass

		return discarded

	def close_stdout(self):
		try:
			if self.p:
				self.p.stdout.close()
		except Exception:
			pass

	def completed(self):
		return self.return_code() is not None

	def run(self):
		self.started = time.time()
		tid = gs.begin(DOMAIN, self.message, set_status=False, cancel=self.cancel)
		try:
			try:
				self.p = gs.popen(self.cmd, shell=self.shell, stderr=subprocess.STDOUT,
					environ=self.env, cwd=self.cwd, bufsize=1)

				CommandStdoutReader(self, self.p.stdout).start()
			except Exception as ex:
				self.x = ex
			finally:
				self.rcode = self.p.wait() if self.p else False
		finally:
			gs.end(tid)
			self.ended = time.time()
			self.on_done(self)

			for f in self.done:
				try:
					f(self)
				except Exception:
					gs.notice(DOMAIN, gs.traceback())

class ViewCommand(Command):
	def __init__(self, cmd=[], shell=False, env={}, cwd=None, view=None):
		self.view = view
		super(ViewCommand, self).__init__(cmd=cmd, shell=shell, env=env, cwd=cwd)

		self.output_done = []
		self.show_summary = False

		if not self.cwd and view is not None:
			try:
				self.cwd = gs.basedir_or_cwd(view.file_name())
			except Exception:
				self.cwd = None

	def poll_output(self):
		l = []
		try:
			for i in range(500):
				l.append(self.q.get_nowait())
		except Queue.Empty:
			pass

		if l:
			self.do_insert(l)

		if self.completed() and self.q.qsize() == 0:
			self.on_output_done()
		else:
			sublime.set_timeout(self.poll_output, 100)

	def do_insert(self, lines):
		if self.view is not None:
			edit = self.view.begin_edit()
			try:
				self.write_lines(self.view, edit, lines)
			finally:
				self.view.end_edit(edit)

	def write_lines(self, view, edit, lines):
		for ln in lines:
			try:
				view.insert(edit, view.size(), u'%s\n' % ln)
			except Exception:
				gs.println(gs.traceback(DOMAIN))
		view.show(view.line(view.size() - 1).begin())

	def on_output_done(self):
		ex = self.exception()
		if ex:
			self.on_output(self, 'Error: ' % ex)

		if self.show_summary:
			t = (max(0, self.ended - self.started), max(0, self.output_started - self.started))
			self.do_insert(['[ elapsed: %0.3fs, startup: %0.3fs ]\n' % t])

		for f in self.output_done:
			try:
				f(self)
			except Exception:
				gs.notice(DOMAIN, gs.traceback())

	def cancel(self):
		discarded = super(ViewCommand, self).cancel()
		t = ((time.time() - self.started), discarded)
		self.on_output(self, ('\n[ cancelled: elapsed: %0.3fs, discarded %d line(s) ]\n' % t))

	def run(self):
		sublime.set_timeout(self.poll_output, 0)
		super(ViewCommand, self).run()
