import sublime, sublime_plugin
import json, os
import gscommon as gs
import margo
from os.path import basename

AC_OPTS = sublime.INHIBIT_WORD_COMPLETIONS | sublime.INHIBIT_EXPLICIT_COMPLETIONS

class GoSublime(sublime_plugin.EventListener):
	gocode_set = False
	def on_query_completions(self, view, prefix, locations):
		pos = locations[0]
		scopes = view.scope_name(pos).split()
		if ('source.go' not in scopes) or (gs.setting('gscomplete_enabled', False) is not True):
			return []

		if gs.IGNORED_SCOPES.intersection(scopes):
			return ([], AC_OPTS)

		if not self.gocode_set:
			self.gocode_set = True
			# autostart the daemon
			gs.runcmd([gs.setting('gocode_cmd', 'gocode')])

		# gocode is case-sesitive so push the location back to the 'dot' so it gives
		# gives us everything then st2 can pick the matches for us
		offset = pos - len(prefix)
		src = view.substr(sublime.Region(0, view.size()))

		fn = view.file_name()
		if not src or not fn:
			return ([], AC_OPTS)


		cl = self.complete(fn, offset, src, view.substr(sublime.Region(pos, pos+1)) == '(')

		pc = view.substr(sublime.Region(pos-1, pos))
		if gs.setting('autocomplete_snippets', True) and (pc.isspace() or pc.isalpha()):
			if scopes[-1] == 'source.go':
				cl.extend(gs.GLOBAL_SNIPPETS)
			elif scopes[-1] == 'meta.block.go' and ('meta.function.plain.go' in scopes or 'meta.function.receiver.go' in scopes):
				cl.extend(gs.LOCAL_SNIPPETS)
		return (cl, AC_OPTS)

	def complete(self, fn, offset, src, func_name_only):
		comps = []
		cmd = gs.setting('gocode_cmd', 'gocode')
		offset = 'c%s' % offset
		args = [cmd, "-f=json", "autocomplete", fn, offset]
		js, err, _ = gs.runcmd(args, src)
		if err:
			gs.notice('GsComplete', err)
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
				gs.notice('GsComplete', 'Error while running gocode, possibly malformed data returned: %s' % e)
			except ValueError as e:
				gs.notice('GsComplete', "Error while decoding gocode output: %s" % e)
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
