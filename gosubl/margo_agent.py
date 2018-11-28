from . import _dbg
from . import sh, gs, gsq
from .margo_common import TokenCounter, OutputLogger, Chan
from .margo_state import State, make_props, actions
from datetime import datetime
import os
import sublime
import subprocess
import threading
import time

ipc_codec = 'msgpack'
ipc_silent_exceptions = (
	EOFError,
	BrokenPipeError,
	ValueError,
)
if ipc_codec == 'msgpack':
	from .vendor import umsgpack
	ipc_loads = umsgpack.loads
	ipc_dec = umsgpack.load
	ipc_enc = umsgpack.dump
	ipc_silent_exceptions += (
		umsgpack.InsufficientDataException,
	)
elif ipc_codec == 'cbor':
	from .vendor.cbor_py import cbor
	ipc_loads = cbor.loads
	ipc_dec = cbor.load
	ipc_enc = cbor.dump
else:
	raise Exception('impossibru')

class MargoAgent(threading.Thread):
	def __init__(self, mg):
		threading.Thread.__init__(self)
		self.daemon = True

		self.mg = mg
		_, self.domain = mg.agent_tokens.next()
		self.cookies = TokenCounter('%s,request' % self.domain)
		self.proc = None
		self.lock = threading.Lock()
		self.out = OutputLogger(self.domain, parent=mg.out)
		self.global_handlers = {}
		self.req_handlers = {}
		self.req_chan = Chan()
		self.starting = threading.Event()
		self.starting.set()
		self.started = threading.Event()
		self.stopped = threading.Event()
		self._queue_ch = Chan(discard=1)
		self.ready = threading.Event()
		gopaths = (gs.user_path(), gs.dist_path())
		psep = sh.psep
		self.gopath = sh.getenv('GOPATH')
		self.data_dir = gs.user_path('margo.data')
		self._default_env = {
			'GOPATH': self.gopath,
			'MARGO_DATA_DIR': self.data_dir,
			'MARGO_AGENT_GO111MODULE': 'off',
			'MARGO_AGENT_GOPATH': psep.join(gopaths),
			'PATH': psep.join([os.path.join(p, 'bin') for p in gopaths]) + psep + os.environ.get('PATH'),
		}
		gs.mkdirp(self.data_dir)

		self._acts_lock = threading.Lock()
		self._acts = []

	def __del__(self):
		self.stop()

	def _env(self, m):
		e = self._default_env.copy()
		e.update(m)
		return e

	def stop(self):
		if self.stopped.is_set():
			return

		self.starting.clear()
		self.stopped.set()
		self._queue_ch.close()
		self.req_chan.close()
		self._stop_proc()
		self._release_handlers()
		self.mg.agent_stopped(self)

	def ok(self):
		return self.proc and self.proc.poll() is None

	def _release_handlers(self):
		with self.lock:
			hdls, self.req_handlers = self.req_handlers, {}

		rs = AgentRes(error='agent stopping. request aborted', agent=self)
		for rq in hdls.values():
			rq.done(rs)

	def run(self):
		self._start_proc()

	def _start_proc(self):
		_dbg.pf(dot=self.domain)

		self.mg.agent_starting(self)
		self.out.println('starting')

		gs_gobin = gs.dist_path('bin')
		mg_exe = 'margo.sh'
		install_cmd = ['go', 'install', '-v', mg_exe]
		cmd = sh.Command(install_cmd)
		cmd.env = self._env({
			'GOPATH': self._default_env['MARGO_AGENT_GOPATH'],
			'GO111MODULE': self._default_env['MARGO_AGENT_GO111MODULE'],
			'GOBIN': gs_gobin,
		})
		cr = cmd.run()
		for v in (cr.out, cr.err, cr.exc):
			if v:
				self.out.println('%s:\n%s' % (install_cmd, v))

		mg_cmd = [
			sh.which(mg_exe, m={'PATH': gs_gobin}) or mg_exe,
			'start', 'margo.sublime', '-codec', ipc_codec,
		]
		self.out.println(mg_cmd)
		cmd = sh.Command(mg_cmd)
		cmd.env = self._env({
			'PATH': gs_gobin,
		})
		pr = cmd.proc()
		if not pr.ok:
			self.stop()
			self.out.println('Cannot start margo: %s' % pr.exc)
			return

		stderr = pr.p.stderr
		self.proc = pr.p
		gsq.launch(self.domain, self._handle_send)
		gsq.launch(self.domain, self._handle_queue)
		gsq.launch(self.domain, self._handle_recv)
		gsq.launch(self.domain, self._handle_log)
		self.started.set()
		self.starting.clear()
		self.proc.wait()
		self._close_file(stderr)

	def _stop_proc(self):
		self.out.println('stopping')
		p = self.proc
		if not p:
			return

		# stderr is closed after .wait() returns
		for f in (p.stdin, p.stdout):
			self._close_file(f)

	def _close_file(self, f):
		if f is None:
			return

		try:
			f.close()
		except Exception as exc:
			self.out.println(exc)
			gs.error_traceback(self.domain)

	def _handle_send_ipc(self, rq):
		with self.lock:
			self.req_handlers[rq.cookie] = rq

		try:
			ipc_enc(rq.data(), self.proc.stdin)
			exc = None
		except Exception as e:
			exc = e
			if not self.stopped.is_set():
				gs.error_traceback(self.domain)

		if exc:
			with self.lock:
				self.req_handlers.pop(rq.cookie, None)

			rq.done(AgentRes(error='Exception: %s' % exc, rq=rq, agent=self))

	def _queued_acts(self, view):
		if view is None:
			return []

		with self._acts_lock:
			q, self._acts = self._acts, []

		acts = []
		for act, vid in q:
			if vid == view.id():
				acts.append(act)

		return acts

	def queue(self, *, actions=[], view=None, delay=-1):
		with self._acts_lock:
			for act in actions:
				p = (act, view.id())
				try:
					self._acts.remove(p)
				except ValueError:
					pass

				self._acts.append(p)

		self._queue_ch.put(delay)

	def send(self, *, actions=[], cb=None, view=None):
		view = gs.active_view(view=view)
		if not isinstance(actions, list):
			raise Exception('actions must be a list, not %s' % type(actions))
		acts = self._queued_acts(view) + actions
		rq = AgentReq(self, acts, cb=cb, view=view)
		timeout = 0.200
		if not self.started.wait(timeout):
			rq.done(AgentRes(error='margo has not started after %0.3fs' % (timeout), timedout=timeout, rq=rq, agent=self))
			return rq

		if not self.req_chan.put(rq):
			rq.done(AgentRes(error='chan closed', rq=rq, agent=self))

		return rq

	def _send_acts(self):
		view = gs.active_view()
		acts = self._queued_acts(view)
		if acts:
			self.send(actions=acts, view=view).wait()

	def _handle_queue(self):
		for n in self._queue_ch:
			time.sleep(n if n >= 0 else 0.600)
			self._send_acts()

	def _handle_send(self):
		for rq in self.req_chan:
			self._handle_send_ipc(rq)

	def _nop_handler(self, rs):
		pass

	def _handler(self, rs):
		if not rs.cookie:
			return self._nop_handler

		with self.lock:
			rq = self.req_handlers.pop(rs.cookie, None)
			if rq:
				rs.set_rq(rq)
				return rq.done

		if rs.cookie in self.global_handlers:
			return self.global_handlers[rs.cookie]

		return lambda rs: self.out.println('unexpected response: %s' % rs)

	def _notify_ready(self):
		if self.ready.is_set():
			return

		self.ready.set()
		self.mg.agent_ready(self)

	def _handle_recv_ipc(self, v):
		self._notify_ready()
		rs = AgentRes(v=v, agent=self)
		# call the handler first. it might be on a timeout (like fmt)
		for handle in [self._handler(rs), self.mg.render]:
			try:
				handle(rs)
			except Exception:
				gs.error_traceback(self.domain)

	def _handle_recv(self):
		try:
			v = None
			while not self.stopped.is_set():
				v = ipc_dec(self.proc.stdout) or {}
				if v:
					self._handle_recv_ipc(v)
		except ipc_silent_exceptions:
			pass
		except Exception as e:
			self.out.println('ipc: recv: %s: %s' % (e, v))
			gs.error_traceback(self.domain)
		finally:
			self.stop()

	def _handle_log(self):
		try:
			for ln in self.proc.stderr:
				try:
					self.out.println('log: %s' % self._decode_ln(ln))
				except (ValueError, OSError):
					pass
				except Exception:
					gs.error_traceback(self.domain)
		except (ValueError, OSError):
			pass
		except Exception:
			gs.error_traceback(self.domain)

	def _decode_ln(self, ln):
		if isinstance(ln, bytes):
			ln = ln.decode('utf-8', 'replace')

		return ln.rstrip('\r\n')

class AgentRes(object):
	def __init__(self, v={}, error='', timedout=0, rq=None, agent=None):
		self.data = v
		self.cookie = v.get('Cookie')
		self.state = State(v=v.get('State') or {})
		self.error = v.get('Error') or error
		self.timedout = timedout
		self.agent = agent
		self.set_rq(rq)

	def set_rq(self, rq):
		if self.error and rq:
			self.error = 'actions: %s, error: %s' % (rq.actions_str, self.error)

	def get(self, k, default=None):
		return self.state.get(k, default)

class AgentReq(object):
	def __init__(self, agent, actions, cb=None, view=None):
		self.start_time = time.time()
		self.actions = actions
		self.actions_str = ' ~> '.join(a['Name'] for a in actions)
		_, cookie = agent.cookies.next()
		self.cookie = 'actions(%s),%s' % (self.actions_str, cookie)
		self.domain = self.cookie
		self.cb = cb
		self.props = make_props(view=view)
		self.rs = DEFAULT_RESPONSE
		self.lock = threading.Lock()
		self.ev = threading.Event()
		self.view = view

	def done(self, rs):
		with self.lock:
			if self.ev.is_set():
				return

			self.rs = rs
			self.ev.set()

		if self.cb:
			try:
				self.cb(self.rs)
			except Exception:
				gs.error_traceback(self.domain)

	def wait(self, timeout=None):
		if self.ev.wait(timeout):
			return self.rs

		return None

	def data(self):
		return {
			'Cookie': self.cookie,
			'Props': self.props,
			'Actions': self.actions,
			'Sent': datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%S.%f'),
		}

DEFAULT_RESPONSE = AgentRes(error='default agent response')
