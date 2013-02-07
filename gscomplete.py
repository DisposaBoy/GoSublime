from gosubl import gs
from gosubl import mg9
from os.path import basename
from os.path import dirname
import json
import os
import re
import sublime
import sublime_plugin

AC_OPTS = sublime.INHIBIT_WORD_COMPLETIONS | sublime.INHIBIT_EXPLICIT_COMPLETIONS
REASONABLE_PKGNAME_PAT = re.compile(r'^\w+$')

last_gopath = ''
END_SELECTOR_PAT = re.compile(r'.*?((?:[\w.]+\.)?(\w+))$')
START_SELECTOR_PAT = re.compile(r'^([\w.]+)')
DOMAIN = 'GsComplete'
SNIPPET_VAR_PAT = re.compile(r'\$\{([a-zA-Z]\w*)\}')

def snippet_match(ctx, m):
	try:
		for k,p in m.get('match', {}).items():
			q = ctx.get(k, '')
			if p and gs.is_a_string(p):
				if not re.search(p, str(q)):
					return False
			elif p != q:
				return False
	except:
		gs.notice(DOMAIN, gs.traceback())
	return True

def expand_snippet_vars(vars, text, title, value):
	sub = lambda m: vars.get(m.group(1), '')
	return (
		SNIPPET_VAR_PAT.sub(sub, text),
		SNIPPET_VAR_PAT.sub(sub, title),
		SNIPPET_VAR_PAT.sub(sub, value)
	)

def resolve_snippets(ctx):
	cl = set()
	types = [''] if ctx.get('local') else ctx.get('types')
	vars = {}
	for k,v in ctx.items():
		if gs.is_a_string(v):
			vars[k] = v

	try:
		snips = []
		snips.extend(gs.setting('default_snippets', []))
		snips.extend(gs.setting('snippets', []))
		for m in snips:
			try:
				if snippet_match(ctx, m):
					for ent in m.get('snippets', []):
						text = ent.get('text', '')
						title = ent.get('title', '')
						value = ent.get('value', '')
						if text and value:
							for typename in types:
								vars['typename'] = typename
								if typename:
									if len(typename) > 1 and typename[0].islower() and typename[1].isupper():
										vars['typename_abbr'] = typename[1].lower()
									else:
										vars['typename_abbr'] = typename[0].lower()
								else:
									vars['typename_abbr'] = ''

								txt, ttl, val = expand_snippet_vars(vars, text, title, value)
								s = u'%s\t%s \u0282' % (txt, ttl)
								cl.add((s, val))
			except:
				gs.notice(DOMAIN, gs.traceback())
	except:
		gs.notice(DOMAIN, gs.traceback())
	return list(cl)

class GoSublime(sublime_plugin.EventListener):
	gocode_set = False
	def on_query_completions(self, view, prefix, locations):
		pos = locations[0]
		scopes = view.scope_name(pos).split()
		if ('source.go' not in scopes) or (gs.setting('gscomplete_enabled', False) is not True):
			return []

		if gs.IGNORED_SCOPES.intersection(scopes):
			return ([], AC_OPTS)

		types = []
		for r in view.find_by_selector('source.go keyword.control.go'):
			if view.substr(r) == 'type':
				end = r.end()
				r = view.find(r'\s+(\w+)', end)
				if r.begin() == end:
					types.append(view.substr(r).lstrip())


		try:
			default_pkgname = basename(dirname(view.file_name()))
		except Exception:
			default_pkgname = ''

		if not REASONABLE_PKGNAME_PAT.match(default_pkgname):
			default_pkgname = ''

		r = view.find('package\s+(\w+)', 0)
		pkgname = view.substr(view.word(r.end())) if r else ''

		if not default_pkgname:
			default_pkgname = pkgname if pkgname else 'main'

		ctx = {
			'global': True,
			'pkgname': pkgname,
			'types': types or [''],
			'has_types': len(types) > 0,
			'default_pkgname': default_pkgname,
			'fn': view.file_name() or '',
		}
		show_snippets = gs.setting('autocomplete_snippets', True) is True

		if not pkgname:
			return (resolve_snippets(ctx), AC_OPTS) if show_snippets else ([], AC_OPTS)

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
				cl.extend(resolve_snippets(ctx))
			elif scopes[-1] == 'meta.block.go' and ('meta.function.plain.go' in scopes or 'meta.function.receiver.go' in scopes):
				ctx['global'] = False
				ctx['local'] = True
				cl.extend(resolve_snippets(ctx))
		return (cl, AC_OPTS)

	def find_end_pt(self, view, pat, start, end, flags=sublime.LITERAL):
		r = view.find(pat, start, flags)
		return r.end() if r and r.end() < end else -1

	def complete(self, fn, offset, src, func_name_only):
		comps = []
		autocomplete_tests = gs.setting('autocomplete_tests', False)
		autocomplete_closures = gs.setting('autocomplete_closures', False)
		ents, err = mg9.complete(fn, src, offset)
		if err:
			gs.notice(DOMAIN, err)

		name_fx = None
		name_fx_pat = gs.setting('autocomplete_filter_name')
		if name_fx_pat:
			try:
				name_fx = re.compile(name_fx_pat)
			except Exception as ex:
				gs.notice(DOMAIN, 'Cannot filter completions: %s' % ex)

		for ent in ents:
			if name_fx and name_fx.search(ent['name']):
				continue

			tn = ent['type']
			cn = ent['class']
			nm = ent['name']
			is_func = (cn == 'func')
			is_func_type = (cn == 'type' and tn.startswith('func('))

			if is_func:
				if nm in ('main', 'init'):
					continue

				if not autocomplete_tests and nm.startswith(('Test', 'Benchmark', 'Example')):
					continue

			if is_func or is_func_type:
				s_sfx = u'\u0282'
				t_sfx = gs.CLASS_PREFIXES.get('type', '')
				f_sfx = gs.CLASS_PREFIXES.get('func', '')
				params, ret = declex(tn)
				decl = []
				for i, p in enumerate(params):
					n, t = p
					if t.startswith('...'):
						n = '...'
					decl.append('${%d:%s}' % (i+1, n))
				decl = ', '.join(decl)
				ret = ret.strip('() ')

				if is_func:
					if func_name_only:
						comps.append((
							'%s\t%s %s' % (nm, ret, f_sfx),
							nm,
						))
					else:
						comps.append((
							'%s\t%s %s' % (nm, ret, f_sfx),
							'%s(%s)' % (nm, decl),
						))
				else:
					comps.append((
						'%s\t%s %s' % (nm, tn, t_sfx),
						nm,
					))

					if autocomplete_closures:
						comps.append((
							'%s {}\tfunc() {...} %s' % (nm, s_sfx),
							'%s {\n\t${0}\n}' % tn,
						))
			elif cn != 'PANIC':
				comps.append((
					'%s\t%s %s' % (nm, tn, self.typeclass_prefix(cn, tn)),
					nm,
				))
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
		pt = gs.sel(view).begin()
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
		sel = m.group(1)
		name = m.group(2)
		candidates = []
		src = view.substr(sublime.Region(0, view.size()))
		fn = view.file_name()
		candidates, err = mg9.complete(fn, src, offset)
		if err:
			gs.notice(DOMAIN, err)
		else:
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
