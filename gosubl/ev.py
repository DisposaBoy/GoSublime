import threading
import traceback

class Event(object):
	def __init__(self):
		self.lst = []
		self.lck = threading.Lock()
		self.post_add = None

	def __call__(self, *args, **kwargs):
		with self.lck:
			l = self.lst[:]

		for f in l:
			try:
				f(*args, **kwargs)
			except Exception:
				print(traceback.format_exc())

		return self

	def __iadd__(self, f):
		with self.lck:
			self.lst.append(f)

		if self.post_add:
			try:
				self.post_add(self, f)
			except Exception:
				print(traceback.format_exc())

		return self

	def __isub__(self, f):
		with self.lck:
			self.lst.remove(f)

		return self

debug = Event()
init = Event()
