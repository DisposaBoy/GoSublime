from . import _dbg
from . import gs, sh
from .margo_common import NS
import os
import re
import sublime

actions = NS(**{k: {'Name': k} for k in (
	'QueryCompletions',
	'QueryTooltips',
	'QueryIssues',
	'ViewActivated',
	'ViewModified',
	'ViewPosChanged',
	'ViewFmt',
	'ViewPreSave',
	'ViewSaved',
	'ViewLoaded',
	'ViewClosed',
)})

class Config(object):
	def __init__(self, m):
		self.override_settings = m.get('OverrideSettings') or {}
		self.enabled_for_langs = m.get('EnabledForLangs') or []
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
		self.view = ResView(v=v.get('View') or {})
		self.client_actions = [ClientAction(v=a) for a in (v.get('ClientActions') or [])]
		self.completions = [Completion(c) for c in (v.get('Completions') or [])]
		self.tooltips = [Tooltip(t) for t in (v.get('Tooltips') or [])]
		self.issues = [Issue(l) for l in (v.get('Issues') or [])]

	def __repr__(self):
		return repr(self.__dict__)

class ClientAction(object):
	def __init__(self, v={}):
		self.name = v.get('Name') or ''

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

class Issue(object):
	def __init__(self, v):
		self.path = v.get('Path') or ''
		self.name = v.get('Name') or ''
		self.hash = v.get('Hash') or ''
		self.row = v.get('Row') or 0
		self.col = v.get('Col') or 0
		self.end = v.get('End') or 0
		self.tag = v.get('Tag') or ''
		self.label = v.get('Label') or ''
		self.message = v.get('Message') or ''

	def __repr__(self):
		return repr(self.__dict__)

	def relpath(self, dir):
		if not self.path:
			return self.name

		if not dir:
			return self.path

		return os.path.relpath(self.path, dir)

class ResView(object):
	def __init__(self, v={}):
		self.name = v.get('Name') or ''
		self.src = v.get('Src') or ''
		if isinstance(self.src, bytes):
			self.src = self.src.decode('utf-8')

# in testing, we should be able to push 50MiB+ files constantly without noticing a performance problem
# but keep this number low (realistic source files sizes) at least until we optimize things
MAX_VIEW_SIZE = 512 << 10

# TODO: only send the content when it actually changes
# TODO: do chunked copying i.e. copy e.g. 1MiB at a time
#       testing in the past revealed that ST will choke under Python memory problems
#       if we attempt to copy large files because it has to convert into utf*
#       which could use up to x4 to convert into the string it gives us
#       and then we have to re-encode that into bytes to send it
def make_props(view=None):
	props = {
		'Editor': _editor_props(view),
		'Env': sh.env(),
		'View': _view_props(view),
	}

	return props

def _editor_props(view):
	sett = gs.setting('margo') or {}
	if view is not None:
		sett.update(view.settings().get('margo') or {})

	return {
		'Name': 'sublime',
		'Version': sublime.version(),
		'Settings': sett,
	}

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
		'Wd': wd,
		'Pos': pos,
		'Row': row,
		'Col': col,
		'Dirty': view.is_dirty(),
		'Src': src,
	})

	return props

def view_name(view, ext='', lang=''):
	if view is None:
		return ''

	if not ext:
		ext = _view_ext(view, lang=lang)

	return 'view#' + _view_id(view) + ext

def view_path(view):
	if view is None:
		return ''

	return view.file_name() or ''

def _view_ext(view, lang=''):
	if view is None:
		return ''

	if not lang:
		_, lang = _view_scope_lang(view, 0)

	return '.%s' % ((view.name() or view_path(view)).split('.')[-1] or lang)

def _view_header(view, pos):
	scope, lang = _view_scope_lang(view, pos)
	path = view_path(view)
	ext = _view_ext(view, lang=lang)
	return scope, lang, path, {
		'Path': path,
		'Name': view_name(view, ext=ext, lang=lang),
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

	if view.size() > MAX_VIEW_SIZE:
		return ''

	return gs.view_src(view)

