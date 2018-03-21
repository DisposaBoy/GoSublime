from . import gs, gsq, sh
from .margo_agent import MargoAgent
from .margo_common import OutputLogger, TokenCounter, Debounce
from .margo_render import render, render_src
from .margo_state import State, actions, Config
from collections import namedtuple
import glob
import os
import sublime
import time

class MargoSingleton(object):
	def __init__(self):
		self.package_dir = os.path.dirname(os.path.abspath(__file__))
		self.out = OutputLogger('margo')
		self.agent_tokens = TokenCounter('agent', format='{}#{:03d}', start=6)
		self.agent = None
		self.on_modified = Debounce(self._on_modified, 1.000)
		self.on_selection_modified = Debounce(self._on_selection_modified, 0.500)
		self._trigger_events = False
		self.state = State()
		self.status = []

	def render(self, rs=None):
		if rs:
			self.state = rs.state
			cfg = rs.state.config

			if cfg.trigger_events:
				self._trigger_events = True

			if cfg.override_settings:
				gs._mg_override_settings = cfg.override_settings

			if rs.state.obsolete:
				self.out.println('restarting: agent is obsolete')
				self.restart()

		sublime.set_timeout_async(lambda: render(view=gs.active_view(), state=self.state, status=self.status), 0)

	def render_status(self, *a):
		self.status = list(a)
		self.render()

	def clear_status(self):
		self.render_status()

	def start(self):
		self.restart()

	def restart(self):
		ag = self.agent
		if ag:
			gsq.dispatch('mg.restart-stop', ag.stop)

		self.agent = MargoAgent(self)
		self.agent.start()

	def stop(self):
		a, self.agent = self.agent, None
		if a:
			a.stop()

	def enabled(self, view):
		return self._trigger_events and sh.init_done

	def can_trigger_event(self, view, allow_9o=False):
		if not self._trigger_events:
			return False

		if view is None:
			return False

		if view.is_loading():
			return False

		vs = view.settings()
		if allow_9o and vs.get('9o'):
			return True

		if vs.get('is_widget'):
			return False

		return True

	def event(self, name, view, handler, args):
		allow_9o = name in (
			'query_completions',
		)
		if not self.can_trigger_event(view, allow_9o=allow_9o):
			return None

		try:
			return handler(*args)
		except Exception:
			gs.error_traceback('mg.event:%s' % handler)
			return None

	def agent_starting(self, ag):
		if ag is not self.agent:
			return

		self.render_status('starting margo')

	def agent_ready(self, ag):
		if ag is not self.agent:
			return

		self.clear_status()
		self.on_activated(gs.active_view())

	def agent_stopped(self, ag):
		if ag is not self.agent:
			return

		self.agent = None
		self.clear_status()

	def send(self, action={}, cb=None, view=None):
		if not self.agent:
			self.start()

		return self.agent.send(action=action, cb=cb, view=view)

	def on_query_completions(self, view, prefix, locations):
		action = actions.QueryCompletions.copy()
		rs = self.send(view=view, action=action).wait(0.300)
		if not rs:
			self.out.println('aborting QueryCompletions. it did not respond in time')
			return None

		cl = [c.entry() for c in rs.state.completions]
		opts = rs.state.config.auto_complete_opts
		return (cl, opts) if opts != 0 else cl

	def on_hover(self, view, point, hover_zone):
		if hover_zone != sublime.HOVER_TEXT:
			return

	def on_activated(self, view):
		self.send(view=view, action=actions.ViewActivated)

	def _on_modified(self, view):
		self.send(view=view, action=actions.ViewModified)

	def _on_selection_modified(self, view):
		self.send(view=view, action=actions.ViewPosChanged)

	def fmt(self, view):
		return self._fmt_save(view=view, action=actions.ViewFmt, name='fmt', timeout=5.000)

	def on_pre_save(self, view):
		return self._fmt_save(view=view, action=actions.ViewPreSave, name='pre-save', timeout=2.000)

	def _fmt_save(self, *, view, action, name, timeout):
		id_nm = '%d: %s' % (view.id(), view.file_name() or view.name())
		rq = self.send(view=view, action=action)
		rs = rq.wait(timeout)
		if not rs:
			self.out.println('%s timedout on view %s' % (name, id_nm))
			return

		if rs.error:
			self.out.println('%s error in view %s: %s' % (name, id_nm, rs.error))
			return

		req = rq.props.get('View', {})
		res = rs.state.view
		req_name, req_src = req.get('Name'), req.get('Src')
		res_name, res_src = res.get('Name'), res.get('Src')

		if not res_name or not res_src:
			return

		if req_name != res_name:
			err = '\n'.join((
				'PANIC!!! FMT REQUEST RECEIVED A RESPONSE TO ANOTHER VIEW',
				'PANIC!!! THIS IS A BUG THAT SHOULD BE REPORTED ASAP',
			))
			self.out.println(err)
			gs.show_output('mg.PANIC', err)
			return

		view.run_command('margo_render_src', {'src': res_src})

	def on_post_save(self, view):
		self.send(view=view, action=actions.ViewSaved)

	def on_load(self, view):
		self.send(view=view, action=actions.ViewLoaded)

	def on_close(self, view):
		self.send(view=view, action=actions.ViewClosed)

	def example_extension_file(self):
		return gs.dist_path('src/disposa.blue/margo/extension-example/extension-example.go')

	def extension_file(self, install=False):
		src_dir = gs.user_path('src', 'margo')

		def ext_fn():
			l = sorted(glob.glob('%s/*.go' % src_dir))
			return l[0] if l else ''

		fn = ext_fn()
		if fn or not install:
			return fn

		try:
			gs.mkdirp(src_dir)
			with open('%s/margo.go' % src_dir, 'x') as f:
				s = open(self.example_extension_file(), 'r').read()
				f.write(s)
		except FileExistsError:
			pass
		except Exception:
			gs.error_traceback('mg.extension_file', status_txt='Cannot create default margo extension package')

		return ext_fn()


mg = MargoSingleton()

def gs_init(_):
	mg.start()

def gs_fini(_):
	mg.stop()

