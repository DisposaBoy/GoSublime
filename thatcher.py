import re

class Editor(object):
	def match_line(self, line_index, content):
		return False
	
	def insert_line(self, line_index, content):
		return False
	
	def delete_line(self, line_index, content):
		return False

class ListEditor(Editor):
	lst = []
	def __init__(self, lst):
		self.lst = lst
	
	def match_line(self, line_index, content):
		if line_index < len(self.lst):
			return self.lst[line_index] == content
		return False
	
	def insert_line(self, line_index, content):
		if line_index <= len(self.lst):
			self.lst.insert(line_index, content)
			return True
		return False
	
	def delete_line(self, line_index):
		if line_index < len(self.lst):
			del self.lst[line_index]
			return True
		return False
	
	def __repr__(self):
		return '\n'.join(self.lst)

px = re.compile(r'^([+][+][+]|[-][-][-]|[ ]|[@][@]|[+]|[-])(.*)$', re.MULTILINE)
def patch(editor, diff):
	patch = px.findall(diff)
	ep = len(patch)
	if ep > 2 and patch[0][0] == '---' and patch[1][0] == '+++':
		sp = 2
		while sp < ep:
			p, ln = patch[sp]
			if p == '@@':
				ln = ln.strip('@ ').split(' ')
				if len(ln) == 2 and ln[0][0] == '-' and ln[1][0] == '+':
					ed_index = ln[1][1:].split(',')
					ed_index = int(ed_index[0]) - 1
					sp += 1
					while sp < ep:
						p, ln = patch[sp]
						if p == ' ':
							if not editor.match_line(ed_index, ln):
								return 'Contextual match failed'
							ed_index += 1
						elif p == '+':
							if not editor.insert_line(ed_index, ln):
								return 'Failed to insert ``%s"' % ln
							ed_index += 1
						elif p == '-':
							if not editor.delete_line(ed_index):
								return 'Failed to delete line %d' % (ed_index+1)
						else:
							# we expect a new hunk
							break
						sp += 1
				else:
					return 'Invalid range'
			else:
				return 'Expected hunk, got ``%s"' % ln
		return ''
	return 'Invalid diff'
