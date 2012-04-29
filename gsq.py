import gscommon as gs
import threading, Queue, traceback

DOMAIN = 'GsQ'

class GsQ(threading.Thread):
	def __init__(self):
		threading.Thread.__init__(self)

		self.q = Queue.PriorityQueue()

	def run(self):
		while True:
			try:
				_, f = self.q.get()
				f()
			except Exception:
				gs.notice(DOMAIN, traceback.format_exc())

	def dispatch(self, f, p=0):
		try:
			self.q.put((p, f))
		except Exception:
			gs.notice(DOMAIN, traceback.format_exc())

Q = None

def dispatch(f, p=0):
	global Q
	if not Q:
		Q = GsQ()
		Q.start()
	Q.dispatch(f, p)
