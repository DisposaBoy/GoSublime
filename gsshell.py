import sublime, sublime_plugin
import gscommon as gs
import re, os, httplib, hashlib, threading, Queue, traceback, subprocess, time, signal

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
				self.settings.set('cmd_hist', hist)
				sublime.save_settings('GoSublime-GsShell.sublime-settings')

			if GO_SHARE_PAT.match(s):
				s = ''
				host = "play.golang.org"
				warning = 'Are you sure you want to share this file. It will be public on %s' % host
				if not sublime.ok_cancel_dialog(warning):
					return

				try:
					c = httplib.HTTPConnection(host)
					src = self.view.substr(sublime.Region(0, self.view.size())).encode('utf-8')
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
							src = self.view.substr(sublime.Region(0, self.view.size())).encode('utf-8')
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
		if sh:
			shell = False
			cmd = []
			for v in sh:
				if v:
					cmd.append(str(v).replace('$CMD', cmd_str))
		else:
			cmd = [cmd_str]

	return (shell, [str(v) for v in cmd])

def run(cmd=[], shell=False, env={}, cwd=None, input=None):
	out = u""
	err = u""
	exc = None

	try:
		env = fix_env(env)
		shell, cmd = fix_shell_cmd(shell, cmd)
		p = gs.popen(cmd, shell=shell, stderr=subprocess.STDOUT, environ=env, cwd=cwd)
		if input is not None:
			input = input.encode('utf-8')
		out, err = p.communicate(input=input)
		out = out.decode('utf-8') if out else u''
		err = err.decode('utf-8') if err else u''
	except (Exception) as e:
		err = u'Error while running %s: %s' % (args[0], e)
		exc = e

	return (out, err, exc)

class Command(threading.Thread):
	def __init__(self, cmd=[], shell=False, env={}, cwd=None):
		super(Command, self).__init__()
		self.lck = threading.Lock()
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
		self.on_done = command_on_done
		self.env = fix_env(env)
		self.shell, self.cmd = fix_shell_cmd(shell, cmd)
		self.message = str(self.cmd)
		self.cwd = cwd if cwd else None

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
		with self.lck:
			if self.p:
				return self.p.poll()
		return False

	def cancel(self):
		if self.poll() is None:
			try:
				os.killpg(self.p.pid, signal.SIGTERM)
			except Exception:
				with self.lck:
					self.p.terminate()

			time.sleep(0.100)
			if not self.completed():
				time.sleep(0.500)
				if not self.completed():
					with self.lck:
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
			with self.lck:
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
					environ=self.env, cwd=self.cwd)

				while True:
					line = self.p.stdout.readline()

					if not line:
						self.close_stdout()
						break

					if not self.output_started:
						self.output_started = time.time()

					self.on_output(self, line.rstrip('\r\n').decode('utf-8'))
			except Exception as ex:
				self.x = ex
			finally:
				if self.p:
					with self.lck:
						self.rcode = self.p.wait()
				else:
					self.rcode = False
		finally:
			gs.end(tid)
			self.ended = time.time()
			self.on_done(self)

class ViewCommand(Command):
	def __init__(self, cmd=[], shell=False, env={}, cwd='', view=None):
		self.view = view
		super(ViewCommand, self).__init__(cmd=cmd, shell=shell, env=env, cwd=cwd)

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

		if not self.completed() or self.q.qsize() > 0:
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
			view.insert(edit, view.size(), u'%s\n' % ln.decode('utf-8'))
		view.show(view.line(view.size() - 1).begin())

	def on_done(self, c):
		ex = self.exception()
		if ex:
			self.on_output(c, 'Error: ' % ex)

		t = (max(0, c.ended - c.started), max(0, c.output_started - c.started))
		self.on_output(c, '[done: elapsed: %0.3fs, startup: %0.3fs]\n' % t)

	def cancel(self):
		discarded = super(ViewCommand, self).cancel()
		t = ((time.time() - self.started), discarded)
		self.on_output(self, ('\n[cancelled: elapsed: %0.3fs, discarded %d line(s)]\n' % t))

	def run(self):
		sublime.set_timeout(self.poll_output, 0)
		super(ViewCommand, self).run()
