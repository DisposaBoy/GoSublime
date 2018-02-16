from . import gs
from collections import deque
import threading
import sublime
import time

class OutputLogger(object):
	def __init__(self, domain, parent=None):
		self.domain = domain
		self.parent = parent

	def println(self, *a):
		s = '%s: %s' % (self.domain, ' '.join((str(v) for v in a)))
		if self.parent:
			self.parent.println(s)
			return

		prefix = time.strftime('[%H:%M:%S]')
		lines = s.split('\n')
		if len(lines) == 1:
			print('%s %s' % (prefix, lines[0]))
			return

		for s in lines:
			print('  %s' % s.strip())

class TokenCounter(object):
	def __init__(self, name, format='{}#{}', start=0):
		self.name = name
		self.format = format
		self.n = start
		self.lock = threading.Lock()

	def next(self):
		with self.lock:
			self.n += 1
			return self.n, self.format.format(self.name, self.n)

class Chan(object):
	def __init__(self, zero=None):
		self.lock = threading.Lock()
		self.ev = threading.Event()
		self.dq = deque()
		self.closed = False
		self.zero = zero

	def put(self, v):
		with self.lock:
			if self.closed:
				return False

			self.dq.append(v)
			self.ev.set()
			return True

	def get(self):
		while True:
			self.ev.wait()
			with self.lock:
				if len(self.dq) != 0:
					v = self.dq.popleft()
					if len(self.dq) == 0:
						self.ev.clear()

					return (v, True)

				if self.closed:
					return (self.zero, False)

				self.ev.clear()

	def close(self):
		with self.lock:
			self.closed = True
			self.ev.set()

	def __iter__(self):
		return self

	def __next__(self):
		v, ok = self.get()
		if ok:
			return v

		raise StopIteration

class NS(object):
	def __init__(self, **fields):
		self.__dict__ = fields

class Debounce(object):
	def __init__(self, cb, delay):
		self.cb = cb
		self.args = []
		self.kwargs = {}
		self.time = 0
		self.delay = delay
		self.waiting = False

	def __call__(self, *args, **kwargs):
		self.args = args
		self.kwargs = kwargs
		self.time = time.time() + self.delay
		if not self.waiting:
			self._sched()

	def _sched(self):
		d = self.time - time.time()
		self.waiting = d > 0
		if self.waiting:
			sublime.set_timeout_async(self._sched, d)
		else:
			cb, args, kwargs = self.cb, self.args, self.kwargs
			sublime.set_timeout_async(lambda: cb(*args, **kwargs), 0)
