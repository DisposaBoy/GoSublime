import copy
import threading

class M(object):
	def __init__():
		self.lck = threading.Lock()
		self.d = {}

	def _get(k, d, set_default):
		# not passing d as default because the stored value can, itself, be `None`
		v = self.d.get(k, None)

		if v is None:
			v = d

			if set_default:
				self.d[k] = copy.copy(v)

		return v

	def get(k, d=None):
		with self.lck:
			return copy.copy(self._get(k, d, False))

	def getdef(k, d=None):
		with self.lck:
			return copy.copy(self._get(k, d, True))

	def put(k, v, d=None):
		with self.lck:
			old_v = self._get(k, d, False)
			self.d[k] = v
			return old_v

	def delete(k, d=None):
		with self.lck:
			old_v = self._get(k, d, False)

			try:
				del self.d[k]
			except Exception:
				pass

			return old_v

	def incr(k, i=1):
		with self.lck:
			old_v = self._get(k, 0, False)
			self.d[k] = old_v + i
			return old_v

	def decr(k, i=1):
		with self.lck:
			old_v = self._get(k, 0, False)
			self.d[k] = old_v - i
			return old_v
