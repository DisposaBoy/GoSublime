from . import _dbg
from . import gs
from . import gspatch
from .margo_state import view_name, view_path
import sublime

STATUS_KEY = '#mg.Status'
STATUS_PFX = '• '
STATUS_SFX = ' •'
STATUS_SEP = ' •• '

def render(view, state, status=[]):
	sublime.set_timeout_async(lambda: _render(view, state, status), 0)

def _render(view, state, status):
	_render_status(view, status + state.status)
	_render_issues(view, state.issues)

def _render_status(view, status):
	if status:
		status_text = (STATUS_PFX +(
			STATUS_SEP.join(status)
		) + STATUS_SFX)
	else:
		status_text = ''

	for w in sublime.windows():
		for v in (w.views() or [w.active_view()]):
			v.set_status(STATUS_KEY, status_text)

def render_src(view, edit, src):
	_, err = gspatch.merge(view, view.size(), src, edit)
	if err:
		msg = 'PANIC: Cannot fmt file. Check your source for errors (and maybe undo any changes).'
		sublime.error_message("margo.render %s: Merge failure: `%s'" % (msg, err))


class IssueCfg(object):
	def __init__(self, *, key, scope, icon, flags):
		self.key = key
		self.scope = scope
		self.icon = icon
		self.flags = flags

can_use_colorish = int(sublime.version()) > 3143
issue_key_pfx = '#mg.Issue.'
issue_cfg_error = IssueCfg(
	key = issue_key_pfx + 'error',
	scope = 'keyword sublimelinter.mark.error region.redish',
	icon = 'Packages/GoSublime/images/issue.png',
	flags = sublime.DRAW_SQUIGGLY_UNDERLINE | sublime.DRAW_NO_OUTLINE | sublime.DRAW_NO_FILL,
)
issue_cfg_warning = IssueCfg(
	key = issue_key_pfx + 'warning',
	scope = 'entity sublimelinter.mark.warning region.orangish',
	icon = issue_cfg_error.icon,
	flags = issue_cfg_error.flags,
)
issue_cfg_default = issue_cfg_warning
issue_cfgs = {
	'error': issue_cfg_error,
	'warning': issue_cfg_warning,
}

def _render_issues(view, issues):
	regions = {cfg.key: (cfg, []) for cfg in issue_cfgs.values()}
	path = view_path(view)
	name = view_name(view)
	for isu in issues:
		if path == isu.path or name == isu.name:
			cfg = issue_cfgs.get(isu.tag) or issue_cfg_default
			regions[cfg.key][1].append(_render_issue(view, isu))

	for cfg, rl in regions.values():
		if rl:
			view.add_regions(cfg.key, rl, cfg.scope, cfg.icon, cfg.flags)
		else:
			view.erase_regions(cfg.key)

	for w in sublime.windows():
		for v in w.views():
			if v.id() != view.id():
				for cfg in issue_cfgs.values():
					v.erase_regions(cfg.key)

def _render_issue(view, isu):
	line = view.line(view.text_point(isu.row, 0))
	lb = line.begin()
	le = line.end()

	sp = lb + isu.col
	if sp >= le:
		sp = lb

	ep = min(lb + isu.end, le) if isu.end > 0 else le

	return sublime.Region(sp, ep)

