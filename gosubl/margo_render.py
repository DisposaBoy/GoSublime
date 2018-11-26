from . import about
from . import _dbg
from . import gs
from . import gspatch
from .margo_state import ViewPathName
import sublime


STATUS_KEY = '#mg.Status'
STATUS_PFX = '  '
STATUS_SFX = '  '
STATUS_SEP = '    '

def render(*, mg, view, state, status):
	def cb():
		_render_tooltips(view, state.tooltips)
		_render_status(view, status + state.status)
		_render_issues(view, state.issues)
		_render_hud(mg=mg, state=state, view=view)

	sublime.set_timeout(cb)

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
	scope = 'region.redish',
	icon = 'Packages/GoSublime/images/issue.png',
	flags = sublime.DRAW_SQUIGGLY_UNDERLINE | sublime.DRAW_NO_OUTLINE | sublime.DRAW_NO_FILL,
)
issue_cfg_warning = IssueCfg(
	key = issue_key_pfx + 'warning',
	scope = 'region.orangish',
	icon = issue_cfg_error.icon,
	flags = issue_cfg_error.flags,
)
issue_cfg_notice = IssueCfg(
	key = issue_key_pfx + 'notice',
	scope = 'region.greenish',
	icon = issue_cfg_error.icon,
	flags = issue_cfg_error.flags,
)
issue_cfg_default = issue_cfg_error
issue_cfgs = {
	'error': issue_cfg_error,
	'warning': issue_cfg_warning,
	'notice': issue_cfg_notice,
}

def _render_issues(view, issues):
	regions = {cfg.key: (cfg, []) for cfg in issue_cfgs.values()}
	vp = ViewPathName(view)
	for isu in issues:
		if isu.match(vp):
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

def _render_tooltips(view, tooltips):
	if not tooltips:
		return

	def ren(t):
		return '''<p>%s</p>''' % (t.content)

	content = '''
		<body>
			<style>
				body {
					font-family: system;
				}
				h1 {
					font-size: 1.1rem;
					font-weight: bold;
					margin: 0 0 0.25em 0;
				}
				p {
					font-size: 1.05rem;
					margin: 0;
				}
			</style>
			%s
		</body>
	''' % ''.join(ren(t) for t in tooltips)

	flags = sublime.COOPERATE_WITH_AUTO_COMPLETE | sublime.HIDE_ON_MOUSE_MOVE_AWAY
	location = -1
	max_width = 640
	max_height = 480

	def on_navigate(href):
		pass

	def on_hide():
		pass

	view.show_popup(
		content,
		flags=flags,
		location=location,
		max_width=max_width,
		max_height=max_height,
		on_navigate=on_navigate,
		on_hide=on_hide
	)

def _render_hud(*, mg, state, view):
	html = '''
		<body id="%s">
			<style>
				body {
					padding: 0.25rem;
				}
				ul, ol {
					padding: 0 0 0 1.1rem;
				}
				ul, ol, li {
					margin: 0;
				}
				.header{
					font-size: 0.6rem;
				}
				.header,
				.header a {
					color: color(var(--foreground) alpha(0.50))
				}
				.footer {}
				.articles {}
				.article, .header {
					margin-bottom: 0.5rem;
				}
				.article {
					font-size: 0.8rem;
				}
				.article .heading,
				.article .heading a {
					font-weight: bold;
					color: color(var(--foreground) alpha(0.50))
				}
				.spacer {
					padding: 0 0.5rem;
				}
				.highlight {
					font-weight: bold;
					background-color: color(var(--foreground) alpha(0.10));
				}
			</style>

			<div class="header">
				GoSublime/HUD
				<span class="spacer">·</span>
				<a href="https://margo.sh/cl/%s?_r=gs-hud">margo.sh/cl/%s</a>
			</div>

			<div class="articles">
				%s
			</div>

		</body>
	''' % (
		mg.hud_id,
		about.VERSION,
		about.VERSION,
		''.join(state.hud.articles),
	)
	def ren(win):
		v, phantoms = mg.hud_panel(win)
		phantom = sublime.Phantom(
			sublime.Region(v.size()),
			html,
			sublime.LAYOUT_INLINE,
			lambda href: mg.navigate(href, view=view),
		)
		vp = v.viewport_position()
		phantoms.update([phantom])
		v.set_viewport_position(vp)

	for w in sublime.windows():
		ren(w)
