from . import gs
from . import sh
import hashlib
import os
import re
import signal
import string
import sublime
import subprocess
import threading
import time
import traceback

DOMAIN = "GsShell"
GO_RUN_PAT = re.compile(r'^go\s+(run|play)$', re.IGNORECASE)
GO_SHARE_PAT = re.compile(r'^go\s+share$', re.IGNORECASE)
GO_PLAY_PAT = re.compile(r'(\b)go\s+play(\b)', re.IGNORECASE)

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
		cmd_str = ' '.join(cmd)
		sh = gs.setting('shell')
		if not sh:
			return (shell, gs.astr(cmd_str))

		shell = False
		cmd_map = {'CMD': cmd_str}
		cmd = []
		for v in sh:
			if v:
				cmd.append(string.Template(v).safe_substitute(cmd_map))

	return (shell, [gs.astr(v) for v in cmd])

def proc(cmd, shell=False, env={}, cwd=None, input=None, stdout=subprocess.PIPE, stderr=subprocess.PIPE, stdin=subprocess.PIPE, bufsize=0):
	env = sh.env(env)
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
		self.q = gs.queue.Queue()
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
		except gs.queue.Empty:
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
		except gs.queue.Empty:
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
		except gs.queue.Empty:
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

	def write_lines(self, view, lines):
		try:
			view.run_command('gs_insert_content', {
				'content': '\n'.join(lines),
				'pos': view.size(),
			})
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
