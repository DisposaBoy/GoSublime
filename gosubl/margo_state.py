from . import gs, sh
from .margo_common import NS
import re
import sublime

actions = NS(**{k: {'Name': k} for k in (
	'QueryCompletions',
	'QueryTooltips',
	'ViewActivated',
	'ViewModified',
	'ViewPosChanged',
	'ViewFmt',
	'ViewSaved',
	'ViewLoaded',
	'ViewClosed',
)})

class Config(object):
	def __init__(self, m):
		self.override_settings = m.get('OverrideSettings') or {}
		self.trigger_events = m.get('Enabled') is True
		self.inhibit_explicit_completions = m.get('InhibitExplicitCompletions') is True
		self.inhibit_word_completions = m.get('InhibitWordCompletions') is True
		self.auto_complete_opts = 0
		if self.inhibit_word_completions:
			self.auto_complete_opts |= sublime.INHIBIT_WORD_COMPLETIONS
		if self.inhibit_explicit_completions:
			self.auto_complete_opts |= sublime.INHIBIT_EXPLICIT_COMPLETIONS

	def __repr__(self):
		return repr(self.__dict__)

class State(object):
	def __init__(self, v={}):
		self.config = Config(v.get('Config') or {})
		self.status = v.get('Status') or []
		self.view = v.get('View') or {}
		self.obsolete = v.get('Obsolete') is True
		self.completions = [Completion(c) for c in (v.get('Completions') or [])]
		self.tooltips = [Tooltip(t) for t in (v.get('Tooltips') or [])]

	def __repr__(self):
		return repr(self.__dict__)

class Completion(object):
	def __init__(self, v):
		self.query = v.get('Query') or ''
		self.title = v.get('Title') or ''
		self.src = v.get('Src') or ''
		self.tag = v.get('Tag') or ''

	def entry(self):
		return (
			'%s\t%s %s' % (self.query, self.title, self.tag),
			self.src,
		)

	def __repr__(self):
		return repr(self.__dict__)

class Tooltip(object):
	def __init__(self, v):
		pass

	def __repr__(self):
		return repr(self.__dict__)

def make_props(view=None):
	props = {
		'Editor': {
			'Name': 'sublime',
			'Version': sublime.version(),
		},
		'Env': sh.env(),
		'View': _view_props(view),
	}

	return props

def _view_props(view):
	view = gs.active_view(view=view)
	if view is None:
		return {}

	pos = gs.sel(view).begin()
	row, col = view.rowcol(pos)
	scope, lang, fn, props = _view_header(view, pos)
	wd = gs.basedir_or_cwd(fn)

	if lang == '9o':
		if 'prompt.9o' in scope:
			r = view.extract_scope(pos)
			pos -= r.begin()
			s = view.substr(r)
			src = s.lstrip().lstrip('#').lstrip()
			pos -= len(s) - len(src)
			src = src.rstrip()
		else:
			pos = 0
			src = ''

		wd = view.settings().get('9o.wd') or wd
		props['Path'] = '_.9o'
	else:
		src = _view_src(view)

	props.update({
		'Dir': wd,
		'Pos': pos,
		'Row': row,
		'Col': col,
		'Dirty': view.is_dirty(),
		'Src': src,
	})

	return props


def _view_header(view, pos):
	scope, lang = _view_scope_lang(view, pos)
	fn = view.file_name() or ''
	ext = '.%s' % ((view.name() or fn).split('.')[-1] or lang)
	return scope, lang, fn, {
		'Path': fn,
		'Name': 'view#' + _view_id(view) + ext,
		'Ext': ext,
		'Hash': _view_hash(view),
		'Lang': lang,
		'Scope': scope,
	}

def _view_id(view):
	if view is None:
		return ''

	return str(view.id())

def _view_hash(view):
	if view is None:
		return ''

	return 'id=%s,change=%d' % (_view_id(view), view.change_count())

_scope_lang_pat = re.compile(r'source[.]([^\s.]+)')
def _view_scope_lang(view, pos):
	if view is None:
		return ('', '')

	scope = view.scope_name(pos).strip().lower()
	l = _scope_lang_pat.findall(scope)
	lang = l[-1] if l else scope.split('.')[-1]
	return (scope, lang)

def _view_src(view):
	if view is None:
		return ''

	if not view.is_dirty():
		return ''

	if view.size() > 10<<20:
		return ''

	return gs.view_src(view)

