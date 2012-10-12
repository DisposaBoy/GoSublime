import json
import subprocess
import threading
import time

class Receiver(threading.Thread):
	def __init__(self, stdout):
		super(Receiver, self).__init__()
		self.daemon = True
		self.stdout = stdout
		self.lines = 0

	def run(self):
		try:
			for line in self.stdout:
				line = line.strip()
				if line:
					self.lines += 1
					res = json.loads(line)
					if res['token'] == 'margo.bye-ni':
						print self.lines, res
					# print self.lines, res
		except Exception:
			pass

		# print 'recv', self.lines

start = time.time()

p = subprocess.Popen(['margo9'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, bufsize=1)
r = Receiver(p.stdout)
r.start()

N = 100000

body = json.dumps({})
for i in range(N):
	header = json.dumps({'token':('i-%s' % i), 'method': 'ping'})
	p.stdin.write('%s\t%s\n' % (header, body))

p.stdin.close()
p.wait()

# print '%0.3f' % ((time.time() - start) / 1000)

raw_input('...\n')
