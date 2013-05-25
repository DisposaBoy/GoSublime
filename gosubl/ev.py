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
				tbck.format_exc()

		return self

	def __iadd__(self, f):
		with self.lck:
			self.lst.append(f)

		if self.post_add:
			self.post_add(self)

		return self

	def __isub__(self, f):
		with self.lck:
			self.lst.remove(f)

		return self

debug = Event()
