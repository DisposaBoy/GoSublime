import gscommon as gs
import threading, Queue, traceback
import sublime

DOMAIN = 'GsQ'

class GsQ(threading.Thread):
	def __init__(self, domain):
		threading.Thread.__init__(self)
		self.daemon = True
		self.q = Queue.Queue()
		self.domain = domain

	def run(self):
		while True:
			try:
				f, msg = self.q.get()
				tid = gs.begin(self.domain, msg, False)

				try:
					f()
				except Exception:
					gs.notice(self.domain, traceback.format_exc())
			except:
				pass

			gs.end(tid)

	def dispatch(self, f, msg):
		try:
			self.q.put((f, msg))
		except Exception:
			gs.notice(self.domain, traceback.format_exc())

try:
	m
except:
	m = {}

def dispatch(domain, f, msg=''):
	global m

	q = m.get(domain, None)
	if not (q and q.is_alive()):
		q = GsQ(domain)
		q.start()
		m[domain] = q

	q.dispatch(f, msg)
