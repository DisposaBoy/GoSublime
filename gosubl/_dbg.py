import sys
import time

class pf(object):
	def __init__(self, print='{name}: {dur}', name='', dot='', gt=0.020):
		self.start = time.time()
		self.end = 0
		self.duration = 0
		self.dur = ''
		self.print = print
		self.gt = gt
		try:
			self.caller = sys._getframe(1).f_code.co_name
		except ValueError:
			self.caller = 'unknown'
		self.name = name or self.caller
		self.dot = dot
		if self.dot:
			self.name += '.' + self.dot

	def __del__(self):
		self.end = time.time()
		self.duration = self.end - self.start
		self.dur = '%0.3fs' % self.duration

		if self.duration <= self.gt:
			return

		if self.print:
			print(self.print.format(**self.__dict__))
