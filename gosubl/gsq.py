from . import gs
import sublime
import threading

DOMAIN = 'GsQ'

class Launcher(threading.Thread):
	def __init__(self, domain, f):
		threading.Thread.__init__(self)
		self.daemon = True
		self.domain = domain
		self.f = f

	def run(self):
		try:
			self.f()
		except Exception:
			gs.notice(self.domain, gs.traceback())

class Runner(threading.Thread):
	def __init__(self, domain, f, msg='', set_status=False):
		threading.Thread.__init__(self)
		self.daemon = True
		self.domain = domain
		self.f = f
		self.msg = msg
		self.set_status = set_status

	def run(self):
		tid = gs.begin(self.domain, self.msg, self.set_status)
		try:
			self.f()
		except Exception:
			gs.notice(self.domain, gs.traceback())
		finally:
			gs.end(tid)

class GsQ(threading.Thread):
	def __init__(self, domain):
		threading.Thread.__init__(self)
		self.daemon = True
		self.q = gs.queue.Queue()
		self.domain = domain

	def run(self):
		while True:
			try:
				f, msg, set_status = self.q.get()
				tid = gs.begin(self.domain, msg, set_status)

				try:
					f()
				except Exception:
					gs.notice(self.domain, gs.traceback())
			except:
				pass

			gs.end(tid)

	def dispatch(self, f, msg, set_status=False):
		try:
			self.q.put((f, msg, set_status))
		except Exception:
			gs.notice(self.domain, traceback())

try:
	m
except:
	m = {}

def dispatch(domain, f, msg='', set_status=False):
	global m

	q = m.get(domain, None)
	if not (q and q.is_alive()):
		q = GsQ(domain)
		q.start()
		m[domain] = q

	q.dispatch(f, msg, set_status)

def do(domain, f, msg='', set_status=False):
	Runner(domain, f, msg, set_status).start()

def launch(domain, f):
	Launcher(domain, f).start()
