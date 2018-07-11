import inspect
import sys
import time

pf_enabled = False
print_enabled = False
gt_default = 0.020

class pf(object):
	def __init__(self, *, format='slow op: {name}: {dur}', name='', dot='', gt=gt_default):
		self.start = time.time()
		self.end = 0
		self.duration = 0
		self.dur = ''
		self.format = format
		self.gt = gt
		self.caller = self._caller_name()
		self.name = name or self.caller
		self.dot = dot
		if self.dot:
			self.name = '%s..%s' % (self.name, self.dot)

	def _caller_name(self):
		if not pf_enabled:
			return ''

		try:
			frame = sys._getframe(2)
		except AttributeError:
			return ''

		try:
			klass = frame.f_locals['self'].__class__
		except KeyError:
			klass = ''

		return '%s.%s' % (klass, frame.f_code.co_name)

	def __del__(self):
		self.end = time.time()
		self.duration = self.end - self.start
		self.dur = '%0.3fs' % self.duration

		if not pf_enabled:
			return

		if self.duration <= self.gt:
			return

		if self.format:
			_println(self.format.format(**self.__dict__))

def _println(*a):
	print('GoSublime_DBG:', *a)

def println(*a):
	if print_enabled:
		_println(*a)

