import sublime, sublime_plugin
import json, os, re
import gscommon as gs
from os.path import basename

AC_OPTS = sublime.INHIBIT_WORD_COMPLETIONS | sublime.INHIBIT_EXPLICIT_COMPLETIONS

last_gopath = ''
END_SELECTOR_PAT = re.compile(r'.*?((?:[\w.]+\.)?(\w+))$')
START_SELECTOR_PAT = re.compile(r'^([\w.]+)')
DOMAIN = 'GsComplete'

class GoSublime(sublime_plugin.EventListener):
	gocode_set = False
	def on_query_completions(self, view, prefix, locations):
		pos = locations[0]
		scopes = view.scope_name(pos).split()
		if ('source.go' not in scopes) or (gs.setting('gscomplete_enabled', False) is not True):
			return []

		if gs.IGNORED_SCOPES.intersection(scopes):
			return ([], AC_OPTS)

		show_snippets = gs.setting('autocomplete_snippets', True) is True

		package_end_pt = self.find_end_pt(view, 'package', 0, pos)
		if package_end_pt < 0:
			return (gs.GLOBAL_SNIPPET_PACKAGE, AC_OPTS) if show_snippets else ([], AC_OPTS)

		# gocode is case-sesitive so push the location back to the 'dot' so it gives
		# gives us everything then st2 can pick the matches for us
		offset = pos - len(prefix)
		src = view.substr(sublime.Region(0, view.size()))

		fn = view.file_name() or '<stdin>'
		if not src:
			return ([], AC_OPTS)

		nc = view.substr(sublime.Region(pos, pos+1))
		cl = self.complete(fn, offset, src, nc.isalpha() or nc == "(")

		pc = view.substr(sublime.Region(pos-1, pos))
		if show_snippets and (pc.isspace() or pc.isalpha()):
			if scopes[-1] == 'source.go':
				cl.extend(gs.GLOBAL_SNIPPETS)
			elif scopes[-1] == 'meta.block.go' and ('meta.function.plain.go' in scopes or 'meta.function.receiver.go' in scopes):
				cl.extend(gs.LOCAL_SNIPPETS)
		return (cl, AC_OPTS)

	def find_end_pt(self, view, pat, start, end, flags=sublime.LITERAL):
		r = view.find(pat, start, flags)
		return r.end() if r and r.end() < end else -1

	def complete(self, fn, offset, src, func_name_only):
		global last_gopath
		gopath = gs.env().get('GOPATH')
		if gopath and gopath != last_gopath:
			out, _, _ = gs.runcmd(['go', 'env', 'GOOS', 'GOARCH'])
			vars = out.strip().split()
			if len(vars) == 2:
				last_gopath = gopath
				fn =  os.path.join(gopath, 'pkg', '_'.join(vars))
				gs.runcmd(['gocode', 'set', 'lib-path', fn])

		comps = []
		cmd = gs.setting('gocode_cmd', 'gocode')
		offset = 'c%s' % offset
		args = [cmd, "-f=json", "autocomplete", fn, offset]
		js, err, _ = gs.runcmd(args, src)
		if err:
			gs.notice(DOMAIN, err)
		else:
			try:
				js = json.loads(js)
				if js and js[1]:
					for ent in js[1]:
						tn = ent['type']
						cn = ent['class']
						nm = ent['name']
						sfx = self.typeclass_prefix(cn, tn)
						if cn == 'func':
							if nm in ('main', 'init'):
								continue
							act = gs.setting('autocomplete_tests', False)
							if not act and nm.startswith(('Test', 'Benchmark', 'Example')):
								continue

							params, ret = declex(tn)
							ret = ret.strip('() ')
							if func_name_only:
								a = nm
							else:
								a = []
								for i, p in enumerate(params):
									n, t = p
									if t.startswith('...'):
										n = '...'
									a.append('${%d:%s}' % (i+1, n))
								a = '%s(%s)' % (nm, ', '.join(a))
							comps.append(('%s\t%s %s' % (nm, ret, sfx), a))
						elif cn != 'PANIC':
							comps.append(('%s\t%s %s' % (nm, tn, sfx), nm))
			except KeyError as e:
				gs.notice(DOMAIN, 'Error while running gocode, possibly malformed data returned: %s' % e)
			except ValueError as e:
				gs.notice(DOMAIN, "Error while decoding gocode output: %s" % e)
		return comps

	def typeclass_prefix(self, typeclass, typename):
		return gs.NAME_PREFIXES.get(typename, gs.CLASS_PREFIXES.get(typeclass, ' '))


def declex(s):
	params = []
	ret = ''
	if s.startswith('func('):
		lp = len(s)
		sp = 5
		ep = sp
		dc = 1
		names = []
		while ep < lp and dc > 0:
			c = s[ep]
			if dc == 1 and c in (',', ')'):
				if sp < ep:
					n, _, t = s[sp:ep].strip().partition(' ')
					t = t.strip()
					if t:
						for name in names:
							params.append((name, t))
						names = []
						params.append((n, t))
					else:
						names.append(n)
					sp = ep + 1
			if c == '(':
				dc += 1
			elif c == ')':
				dc -= 1
			ep += 1
		ret = s[ep:].strip() if ep < lp else ''
	return (params, ret)

class GsShowCallTip(sublime_plugin.TextCommand):
	def is_enabled(self):
		return gs.is_go_source_view(self.view)

	def show_hint(self, s):
		dmn = '%s.completion-hint' % DOMAIN
		gs.show_output(dmn, s, print_output=False, syntax_file='GsDoc')

	def run(self, edit):
		view = self.view
		pt = view.sel()[0].begin()
		if view.substr(sublime.Region(pt-1, pt)) == '(':
			depth = 1
		else:
			depth = 0
		c = ''
		while True:
			line = view.line(pt)
			scope = view.scope_name(pt)
			if 'string' in scope or 'comment' in scope:
				pt = view.extract_scope(pt).begin() - 1
				continue

			c = view.substr(sublime.Region(pt-1, pt))
			if not c:
				pt = -1
				break

			if c.isalpha() and depth >= 0:
				while c.isalpha() or c == '.':
					pt += 1
					c = view.substr(sublime.Region(pt-1, pt))

				# curly braces ftw
				break # break outer while loop
			if c == ')':
				depth -= 1
			elif c == '(':
				depth += 1
				i = pt
				while True:
					pc = view.substr(sublime.Region(i-1, i))
					if pc == '.' or pc.isalpha():
						i -= 1
					else:
						break

				if i != pt:
					pt = i
					continue

			pt -= 1
			if pt <= line.begin():
				pt = -1
				break

		while not c.isalpha() and pt > 0:
			pt -= 1
			c = view.substr(sublime.Region(pt-1, pt))

		if pt <= 0 or view.scope_name(pt).strip() == 'source.go':
			self.show_hint("// can't find selector")
			return

		line = view.line(pt)
		line_start = line.begin()

		s = view.substr(line)
		if not s:
			self.show_hint('// no source')
			return

		scopes = [
			'support.function.any-method.go',
			'meta.function-call.go',
			'support.function.builtin.go',
		]
		found = False
		while True:
			scope = view.scope_name(pt).strip()
			for s in scopes:
				if scope.endswith(s):
					found = True
					break

			if found or pt <= line_start:
				break

			pt -= 1

		if not found:
			self.show_hint("// can't find function call")
			return

		s = view.substr(sublime.Region(line_start, pt))
		m = END_SELECTOR_PAT.match(s)
		if not m:
			self.show_hint("// can't match selector")
			return

		offset = (line_start + m.end())
		coffset = 'c%d' % offset
		sel = m.group(1)
		name = m.group(2)
		candidates = []
		src = view.substr(sublime.Region(0, view.size()))
		fn = view.file_name() or '<stdin>'
		cmd = gs.setting('gocode_cmd', 'gocode')
		args = [cmd, "-f=json", "autocomplete", fn, coffset]
		js, err, _ = gs.runcmd(args, src)
		if err:
			gs.notice(DOMAIN, err)
		else:
			try:
				js = json.loads(js)
				if js and js[1]:
					candidates = js[1]
			except:
				pass

		c = {}
		for i in candidates:
			if i['name'] == name:
				if c:
					c = None
					break
				c = i

		if not c:
			self.show_hint('// no candidates found')
			return

		s = '// %s %s\n%s' % (c['name'], c['class'], c['type'])
		self.show_hint(s)


