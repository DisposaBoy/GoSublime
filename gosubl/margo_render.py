from . import gspatch
import sublime

STATUS_KEY = '#mg.Status'
STATUS_PFX = '{• '
STATUS_SFX = ' •}'
STATUS_SEP = ' •• '
STATUS_DEF = ['_']

def render(view, state):
	sublime.set_timeout_async(lambda: _render(view, state), 0)

def _render(view, state):
	_render_status(view, state.status)

def _render_status(view, status):
	status_text = (STATUS_PFX +(
		STATUS_SEP.join(status or STATUS_DEF)
	) + STATUS_SFX)

	for w in sublime.windows():
		for v in w.views():
			v.set_status(STATUS_KEY, status_text)

def render_src(view, edit, src):
	_, err = gspatch.merge(view, view.size(), src, edit)
	if err:
		msg = 'PANIC: Cannot fmt file. Check your source for errors (and maybe undo any changes).'
		sublime.error_message("%s: %s: Merge failure: `%s'" % (domain, msg, err))
