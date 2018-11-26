from . import _dbg
from . import gs, sh, about
from .margo_common import NS
from os.path import basename, splitext
import os
import re
import sublime

actions = NS(**{k: {'Name': k} for k in (
	'QueryCompletions',
	'QueryCmdCompletions',
	'QueryTooltips',
	'QueryIssues',
	'QueryUserCmds',
	'QueryTestCmds',
	'ViewActivated',
	'ViewModified',
	'ViewPosChanged',
	'ViewFmt',
	'ViewPreSave',
	'ViewSaved',
	'ViewLoaded',
	'RunCmd',
)})

client_actions = NS(**{k: k for k in (
	'Activate',
	'Restart',
	'Shutdown',
	'CmdOutput',
	'DisplayIssues',
)})

class MgView(sublime.View):
	def __init__(self, *, mg, view):
		self.mg = mg
		self.is_9o = False
		self.is_file = False
		self.is_widget = False
		self.sync(view=view)

	def sync(self, *, view):
		if view is None:
			return

		_pf=_dbg.pf(dot=self.id)
		self.id = view.id()
		self.view = view
		self.name = view_name(view)
		self.is_file = self.id in self.mg.file_ids
		self.is_widget = not self.is_file

	def __eq__(self, v):
		return self.view == v

	def __hash__(self):
		return self.id

	def __repr__(self):
		return repr(vars(self))

	def name(self):
		return view_name(self.view)

class Config(object):
	def __init__(self, m):
		efl = m.get('EnabledForLangs')
		if m and (not isinstance(efl, list) or len(efl) == 0):
			print('MARGO BUG: EnabledForLangs is invalid.\nIt must be a non-empty list, not `%s: %s`\nconfig data: %s' % (type(efl), efl, m))

		self.override_settings = m.get('OverrideSettings') or {}
		self.enabled_for_langs = efl or ['*']
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
		self.errors = v.get('Errors') or []
		self.status = v.get('Status') or []
		self.view = ResView(v=v.get('View') or {})
		self.completions = [Completion(c) for c in (v.get('Completions') or [])]
		self.tooltips = [Tooltip(t) for t in (v.get('Tooltips') or [])]
		self.issues = [Issue(l) for l in (v.get('Issues') or [])]
		self.user_cmds = [UserCmd(c) for c in (v.get('UserCmds') or [])]
		self.hud = HUD(v=v.get('HUD') or {})

		self.client_actions = []
		for ca in (v.get('ClientActions') or []):
			CA = client_action_creators.get(ca.get('Name') or '') or ClientAction
			self.client_actions.append(CA(v=ca))

	def __repr__(self):
		return repr(self.__dict__)

class ClientAction(object):
	def __init__(self, v={}):
		self.action_name = v.get('Name') or ''
		self.action_data = v.get('Data') or {}

	def __repr__(self):
		return repr(vars(self))

class ClientAction_Output(ClientAction):
	def __init__(self, v):
		super().__init__(v=v)
		ad = self.action_data

		self.fd = ad.get('Fd') or ''
		self.output = ad.get('Output') or ''
		self.close = ad.get('Close') or False
		self.fd = ad.get('Fd') or ''

	def __repr__(self):
		return repr(vars(self))

class ClientAction_Activate(ClientAction):
	def __init__(self, v):
		super().__init__(v=v)
		ad = self.action_data

		self.path = ad.get('Path') or ''
		self.name = ad.get('Name') or ''
		self.row = ad.get('Row') or 0
		self.col = ad.get('Col') or 0

	def __repr__(self):
		return repr(vars(self))

client_action_creators = {
	client_actions.CmdOutput: ClientAction_Output,
	client_actions.Activate: ClientAction_Activate,
}

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
		self.content = v.get('Content') or ''

	def __repr__(self):
		return repr(self.__dict__)

class PathName(object):
	def __init__(self, *, path, name):
		self.path = path or ''
		self.name = name or ''

	def match(self, p):
		if self.path and self.path == p.path:
			return True

		if self.name and self.name == p.name:
			return True

		return False

	def __repr__(self):
		return repr(vars(self))

class ViewPathName(PathName):
	def __init__(self, view):
		super().__init__(
			path = view_path(view),
			name = view_name(view),
		)

class Issue(PathName):
	def __init__(self, v):
		super().__init__(
			path = v.get('Path') or '',
			name = v.get('Name') or '',
		)
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

	def basename(self):
		if not self.path:
			return self.name

		return os.path.basename(self.path)

class ResView(object):
	def __init__(self, v={}):
		self.name = v.get('Name') or ''
		self.src = v.get('Src') or ''
		if isinstance(self.src, bytes):
			self.src = self.src.decode('utf-8')

class UserCmd(object):
	def __init__(self, v={}):
		self.title = v.get('Title') or ''
		self.desc = v.get('Desc') or ''
		self.name = v.get('Name') or ''
		self.args = v.get('Args') or []
		self.prompts = v.get('Prompts') or []

class HUD(object):
	def __init__(self, v={}):
		self.articles = v.get('Articles') or []

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
		'Client': {
			'Name': 'gosublime',
			'Tag': about.TAG,
		},
		'Settings': sett,
	}

def view_is_9o(view):
	return view is not None and view.settings().get('9o')

def _view_props(view):
	was_9o = view_is_9o(view)
	if was_9o:
		view = gs.active_view()
	else:
		view = gs.active_view(view=view)

	if view is None:
		return {}

	pos = gs.sel(view).begin()
	scope, lang, fn, props = _view_header(view, pos)
	wd = gs.getwd() or gs.basedir_or_cwd(fn)
	src = _view_src(view, lang)

	props.update({
		'Wd': wd,
		'Pos': pos,
		'Dirty': view.is_dirty(),
		'Src': src,
	})

	return props

_sanitize_view_name_pat = re.compile(r'[^-~,.@\w]')

def view_name(view, ext='', lang=''):
	if view is None:
		return '_._'

	nm = basename(view.file_name() or view.name() or '_')
	nm, nm_ext = splitext(nm)
	if not ext:
		ext = _view_ext(view, lang=lang) or nm_ext or '._'
	nm = 'view@%s,%s%s' % (_view_id(view), nm, ext)
	nm = _sanitize_view_name_pat.sub('', nm)
	return nm

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

_scope_lang_pat = re.compile(r'(?:source\.\w+|source|text)[.]([^\s.]+)')
def _view_scope_lang(view, pos):
	if view is None:
		return ('', '')

	_pf=_dbg.pf()
	scope = view.scope_name(pos).strip().lower()

	if view_is_9o(view):
		return (scope, 'cmd-prompt')

	l = _scope_lang_pat.findall(scope)
	if not l:
		return (scope, '')

	blacklist = (
		'plain',
		'find-in-files',
	)
	lang = l[-1]
	if lang in blacklist:
		return (scope, '')

	return (scope, lang)

def _view_src(view, lang):
	if view is None:
		return ''

	if not lang:
		return ''

	if not view.is_dirty():
		return ''

	if view.is_loading():
		return ''

	if view.size() > MAX_VIEW_SIZE:
		return ''

	return gs.view_src(view)

