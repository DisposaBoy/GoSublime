# Sublime modelines - https://github.com/SublimeText/Modelines
# sublime: translate_tabs_to_spaces false; rulers [100,120]

from subprocess import Popen, PIPE
import copy
import datetime
import json
import os
import re
import string
import sublime
import subprocess
import sys
import tempfile
import threading
import traceback as tbck
import uuid

try:
	import Queue as queue
except ImportError:
	import queue

PY3K = (sys.version_info[0] == 3)

try:
	STARTUP_INFO = subprocess.STARTUPINFO()
	STARTUP_INFO.dwFlags |= subprocess.STARTF_USESHOWWINDOW
	STARTUP_INFO.wShowWindow = subprocess.SW_HIDE
except (AttributeError):
	STARTUP_INFO = None

NAME = 'GoSublime'

mg9_send_q = queue.Queue()
mg9_recv_q = queue.Queue()

_attr_lck = threading.Lock()
_attr = {}

_checked_lck = threading.Lock()
_checked = {}

environ9 = {}
_env_lck = threading.Lock()
_default_settings = {
	"_debug": False,
	"env": {},
	"gscomplete_enabled": False,
	"complete_builtins": False,
	"autocomplete_builtins": False,
	"fmt_enabled": False,
	"fmt_tab_indent": True,
	"fmt_tab_width": 8,
	"gslint_enabled": False,
	"comp_lint_enabled": False,
	"comp_lint_commands": [],
	"gslint_timeout": 0,
	"autocomplete_snippets": False,
	"autocomplete_tests": False,
	"autocomplete_closures": False,
	"autocomplete_filter_name": "",
	"autocomplete_suggest_imports": False,
	"margo_addr": "",
	"on_save": [],
	"shell": [],
	"default_snippets": [],
	"snippets": [],
	"fn_exclude_prefixes": [".", "_"],
	"autosave": True,
	"build_command": [],
}
_settings = copy.copy(_default_settings)


CLASS_PREFIXES = {
	'const': u'\u0196',
	'func': u'\u0192',
	'type': u'\u0288',
	'var':  u'\u03BD',
	'package': u'package \u03C1',
}

NAME_PREFIXES = {
	'interface': u'\u00A1',
}

GOARCHES = [
	'386',
	'amd64',
	'arm',
]

GOOSES = [
	'darwin',
	'freebsd',
	'linux',
	'netbsd',
	'openbsd',
	'plan9',
	'windows',
	'unix',
]

GOOSARCHES = []
for s in GOOSES:
	for arch in GOARCHES:
		GOOSARCHES.append('%s_%s' % (s, arch))

GOOSARCHES_PAT = re.compile(r'^(.+?)(?:_(%s))?(?:_(%s))?\.go$' % ('|'.join(GOOSES), '|'.join(GOARCHES)))

IGNORED_SCOPES = frozenset([
	'string.quoted.double.go',
	'string.quoted.single.go',
	'string.quoted.raw.go',
	'comment.line.double-slash.go',
	'comment.block.go'
])

VFN_ID_PAT = re.compile(r'^(?:gs\.)?view://(\d+)$', re.IGNORECASE)

def apath(fn, cwd=None):
	if not os.path.isabs(fn):
		if not cwd:
			cwd = os.getcwd()
		fn = os.path.join(cwd, fn)
	return os.path.normcase(os.path.normpath(fn))

def temp_dir(subdir=''):
	tmpdir = os.path.join(tempfile.gettempdir(), NAME, subdir)
	err = ''
	try:
		os.makedirs(tmpdir)
	except Exception as ex:
		err = str(ex)
	return (tmpdir, err)

def temp_file(suffix='', prefix='', delete=True):
	try:
		f = tempfile.NamedTemporaryFile(suffix=suffix, prefix=prefix, dir=temp_dir(), delete=delete)
	except Exception as ex:
		return (None, 'Error: %s' % ex)
	return (f, '')

def basedir_or_cwd(fn):
	if fn and not fn.startswith('gs.view://'):
		return os.path.dirname(fn)
	return os.getcwd()

def popen(args, stdout=PIPE, stderr=PIPE, shell=False, environ={}, cwd=None, bufsize=0):
	ev = env()
	for k,v in environ.items():
		ev[astr(k)] = astr(v)

	try:
		setsid = os.setsid
	except Exception:
		setsid = None

	return Popen(args, stdout=stdout, stderr=stderr, stdin=PIPE, startupinfo=STARTUP_INFO,
		shell=shell, env=ev, cwd=cwd, preexec_fn=setsid, bufsize=bufsize)

def is_a(v, base):
	return isinstance(v, type(base))

def is_a_string(v):
	try:
		return isinstance(v, basestring)
	except NameError:
		return isinstance(v, str)

def settings_obj():
	return sublime.load_settings("GoSublime.sublime-settings")

def aso():
	return sublime.load_settings("GoSublime-aux.sublime-settings")

def save_aso():
	return sublime.save_settings("GoSublime-aux.sublime-settings")

def settings_dict():
	m = copy.copy(_settings)

	for k in m:
		v = attr(k, None)
		if v is not None:
			m[k] = v

	nv = dval(copy.copy(_settings.get('env')), {})
	lpe = dval(attr('last_active_project_settings', {}).get('env'), {})
	nv.update(lpe)
	m['env'] = nv

	return m

def setting(k, d=None):
	return settings_dict().get(k, d)

def println(*a):
	l = []
	l.append('\n** %s **:' % datetime.datetime.now())
	for s in a:
		l.append(ustr(s).strip())
	l.append('--------------------------------')

	l = '%s\n' % '\n'.join(l)
	print(l)
	return l

def debug(domain, *a):
	if setting('_debug') is True:
		print('\n** DEBUG ** %s ** %s **:' % (domain, datetime.datetime.now()))
		for s in a:
			print(ustr(s).strip())
		print('--------------------------------')

def log(*a):
	try:
		LOGFILE.write(println(*a))
		LOGFILE.flush()
	except Exception:
		pass

def notify(domain, txt):
	txt = "%s: %s" % (domain, txt)
	status_message(txt)

def notice(domain, txt):
	error(domain, txt)

def error(domain, txt):
	txt = "%s: %s" % (domain, txt)
	log(txt)
	status_message(txt)

def error_traceback(domain, status_txt=''):
	tb = traceback().strip()
	if status_txt:
		prefix = '%s\n' % status_txt
	else:
		prefix = ''
		i = tb.rfind('\n')
		if i > 0:
			status_txt = tb[i:].strip()
		else:
			status_txt = tb

	log("%s: %s%s" % (domain, prefix, tb))
	status_message("%s: %s" % (domain, status_txt))

def notice_undo(domain, txt, view, should_undo):
	def cb():
		if should_undo:
			view.run_command('undo')
		notice(domain, txt)
	sublime.set_timeout(cb, 0)

def show_output(domain, s, print_output=True, syntax_file='', replace=True, merge_domain=False, scroll_end=False):
	def cb(domain, s, print_output, syntax_file):
		panel_name = '%s-output' % domain
		if merge_domain:
			s = '%s: %s' % (domain, s)
			if print_output:
				println(s)
		elif print_output:
			println('%s: %s' % (domain, s))

		win = sublime.active_window()
		if win:
			win.get_output_panel(panel_name).run_command('gs_set_output_panel_content', {
				'content': s,
				'syntax_file': syntax_file,
				'scroll_end': scroll_end,
				'replace': replace,
			})
			win.run_command("show_panel", {"panel": "output.%s" % panel_name})

	sublime.set_timeout(lambda: cb(domain, s, print_output, syntax_file), 0)

def is_pkg_view(view=None):
	# todo implement this fully
	return is_go_source_view(view, False)

def is_go_source_view(view=None, strict=True):
	if view is None:
		return False

	selector_match = view.score_selector(sel(view).begin(), 'source.go') > 0
	if selector_match:
		return True

	if strict:
		return False

	fn = view.file_name() or ''
	return fn.lower().endswith('.go')

def active_valid_go_view(win=None, strict=True):
	if not win:
		win = sublime.active_window()
	if win:
		view = win.active_view()
		if view and is_go_source_view(view, strict):
			return view
	return None

def rowcol(view):
	return view.rowcol(sel(view).begin())

def os_is_windows():
	return os.name == "nt"

def getenv(name, default='', m={}):
	return env(m).get(name, default)

def env(m={}):
	"""
	Assemble environment information needed for correct operation. In particular,
	ensure that directories containing binaries are included in PATH.
	"""
	e = os.environ.copy()
	e.update(environ9)
	e.update(m)

	roots = lst(e.get('GOPATH', '').split(os.pathsep), e.get('GOROOT', ''))
	lfn = attr('last_active_go_fn', '')
	comps = lfn.split(os.sep)
	gs_gopath = []
	for i, s in enumerate(comps):
		if s.lower() == "src":
			p = os.sep.join(comps[:i])
			if p not in roots:
				gs_gopath.append(p)
	gs_gopath.reverse()
	e['GS_GOPATH'] = os.pathsep.join(gs_gopath)

	uenv = setting('env', {})
	for k in uenv:
		try:
			uenv[k] = string.Template(uenv[k]).safe_substitute(e)
		except Exception as ex:
			println('%s: Cannot expand env var `%s`: %s' % (NAME, k, ex))

	e.update(uenv)
	e.update(m)

	# For custom values of GOPATH, installed binaries via go install
	# will go into the "bin" dir of the corresponding GOPATH path.
	# Therefore, make sure these paths are included in PATH.

	# do this again based on updated vars
	roots = lst(e.get('GOPATH', '').split(os.pathsep), e.get('GOROOT', ''))
	add_path = [home_path('bin')]
	add_path.extend(e.get('PATH', '').split(os.pathsep))
	for s in roots:
		if s:
			s = os.path.join(s, 'bin')
			if s not in add_path:
				add_path.append(s)

	if os_is_windows():
		l = [
			'~\\bin',
			'~\\go\\bin',
			'C:\\Go\\bin',
		]
	else:
		l = [
			'~/bin',
			'~/go/bin',
			'/usr/local/go/bin',
			'/usr/local/opt/go/bin',
			'/usr/local/bin',
			'/usr/bin',
		]

	for s in l:
		s = os.path.expanduser(s)
		if s not in add_path:
			add_path.append(s)

	e['PATH'] = os.pathsep.join(add_path)

	# Ensure no unicode objects leak through. The reason is twofold:
	# 	* On Windows, Python 2.6 (used by Sublime Text) subprocess.Popen
	# 	  can only take bytestrings as environment variables in the
	#	  "env"	parameter. Reference:
	# 	  https://github.com/DisposaBoy/GoSublime/issues/112
	# 	  http://stackoverflow.com/q/12253014/1670
	#   * Avoids issues with networking too.
	clean_env = {}
	for k, v in e.items():
		try:
			clean_env[astr(k)] = astr(v)
		except Exception as ex:
			println('%s: Bad env: %s' % (NAME, ex))

	return clean_env

def mirror_settings(so):
	m = {}
	for k in _default_settings:
		v = so.get(k, None)
		if v is not None:
			ok = False
			d = _default_settings[k]

			if is_a(d, []):
				if is_a(v, []):
					ok = True
			elif is_a(d, {}):
				if is_a(v, []):
					ok = True
			else:
				ok = True

			m[k] = copy.copy(v)
	return m

def sync_settings():
	_settings.update(mirror_settings(settings_obj()))

def view_fn(view):
	if view is not None:
		if view.file_name():
			return view.file_name()
		return 'gs.view://%s' % view.id()
	return ''

def view_src(view):
	if view:
		return view.substr(sublime.Region(0, view.size()))
	return ''

def win_view(vfn=None, win=None):
	if not win:
		win = sublime.active_window()

	view = None
	if win:
		m = VFN_ID_PAT.match(vfn or '')
		if m:
			try:
				vid = int(m.group(1))
				for v in win.views():
					if v.id() == vid:
						view = v
						break
			except Exception:
				gs.error_traceback(NAME)
		elif not vfn or vfn == "<stdin>":
			view = win.active_view()
		else:
			view = win.open_file(vfn)
	return (win, view)

def do_focus(fn, row, col, win=None, focus_pkg=True):
	win, view = win_view(fn, win)
	if win is None or view is None:
		notify(NAME, 'Cannot find file position %s:%s:%s' % (fn, row, col))
	elif view.is_loading():
		focus(fn, row, col, win, focus_pkg)
	else:
		win.focus_view(view)
		if row <= 0 and col <= 0 and focus_pkg:
			r = view.find('^package ', 0)
			if r:
				row, col = view.rowcol(r.begin())
		view.run_command("gs_goto_row_col", { "row": row, "col": col })

def focus(fn, row=0, col=0, win=None, timeout=100, focus_pkg=True):
	sublime.set_timeout(lambda: do_focus(fn, row, col, win, focus_pkg), timeout)

def sm_cb():
	global sm_text
	global sm_set_text
	global sm_frame

	with sm_lck:
		ntasks = len(sm_tasks)
		tm = sm_tm
		s = sm_text
		if s:
			delta = (datetime.datetime.now() - tm)
			if delta.seconds >= 5:
				sm_text = ''

	if ntasks > 0:
		if s:
			s = u'%s, %s' % (sm_frames[sm_frame], s)
		else:
			s = u'%s' % sm_frames[sm_frame]

		if ntasks > 1:
			s = '%d %s' % (ntasks, s)

		sm_frame = (sm_frame + 1) % len(sm_frames)

	if s != sm_set_text:
		sm_set_text = s
		st2_status_message(s)

	sched_sm_cb()


def sched_sm_cb():
	sublime.set_timeout(sm_cb, 250)

def status_message(s):
	global sm_text
	global sm_tm

	with sm_lck:
		sm_text = s
		sm_tm = datetime.datetime.now()

def begin(domain, message, set_status=True, cancel=None):
	global sm_task_counter

	if message and set_status:
		status_message('%s: %s' % (domain, message))

	with sm_lck:
		sm_task_counter += 1
		tid = 't%d' % sm_task_counter
		sm_tasks[tid] = {
			'start': datetime.datetime.now(),
			'domain': domain,
			'message': message,
			'cancel': cancel,
		}

	return tid

def end(task_id):
	with sm_lck:
		try:
			del(sm_tasks[task_id])
		except:
			pass

def task(task_id, default=None):
	with sm_lck:
		return sm_tasks.get(task_id, default)

def clear_tasks():
	with sm_lck:
		sm_tasks = {}

def task_list():
	with sm_lck:
		return sorted(sm_tasks.items())

def cancel_task(tid):
	t = task(tid)
	if t and t['cancel']:
		s = 'are you sure you want to end task: #%s %s: %s' % (tid, t['domain'], t['message'])
		if sublime.ok_cancel_dialog(s):
			t['cancel']()

		return True
	return False

def show_quick_panel(items, cb=None):
	def f():
		win = sublime.active_window()
		if win:
			win.show_quick_panel(items, (lambda i: cb(i, win)) if cb else (lambda i: None))
	sublime.set_timeout(f, 0)

def go_env_goroot():
	out, _, _ = runcmd(['go env GOROOT'], shell=True)
	return out.strip().encode('utf-8')

def list_dir_tree(dirname, filter, exclude_prefix=('.', '_')):
	lst = []

	try:
		for fn in os.listdir(dirname):
			if fn[0] in exclude_prefix:
				continue

			basename = fn.lower()
			fn = os.path.join(dirname, fn)

			if os.path.isdir(fn):
				lst.extend(list_dir_tree(fn, filter, exclude_prefix))
			else:
				if filter:
					pathname = fn.lower()
					_, ext = os.path.splitext(basename)
					ext = ext.lstrip('.')
					if filter(pathname, basename, ext):
						lst.append(fn)
				else:
					lst.append(fn)
	except Exception:
		pass

	return lst

def traceback(domain='GoSublime'):
	return '%s: %s' % (domain, tbck.format_exc())

def show_traceback(domain):
	show_output(domain, traceback(), replace=False, merge_domain=False)

def maybe_unicode_str(s):
	try:
		return isinstance(s, unicode)
	except NameError:
		return isinstance(s, str)

def ustr(s):
	if maybe_unicode_str(s):
		return s

	if PY3K:
		return str(s, 'utf-8')
	return str(s).decode('utf-8')

def astr(s):
	if maybe_unicode_str(s):
		if PY3K:
			return s
		return s.encode('utf-8')

	return str(s)

def lst(*a):
	l = []
	for v in a:
		if is_a([], v):
			l.extend(v)
		else:
			l.append(v)
	return l

def dval(v, d):
	if v is not None:
		if is_a_string(d) and is_a_string(v):
			return v

		if is_a(v, d):
			return v

	return d

def dist_path(*a):
	return os.path.join(sublime.packages_path(), 'GoSublime', *a)

def home_path(*a):
	p = os.path.join(sublime.packages_path(), 'User', 'GoSublime', '9')
	if not os.path.exists(p):
		try:
			os.makedirs(p)
		except:
			println('Error while creating 9/home dir', traceback())
	return os.path.join(p, *a)

def json_decode(s, default):
	try:
		res = json.loads(s)
		if is_a(res, default):
			return (res, '')
		return (res, 'Unexpected value type')
	except Exception as ex:
		return (default, 'Decode Error: %s' % ex)

def json_encode(a):
	try:
		return (json.dumps(a), '')
	except Exception as ex:
		return ('', 'Encode Error: %s' % ex)

def attr(k, d=None):
	with _attr_lck:
		v = _attr.get(k, None)
		return d if v is None else copy.copy(v)

def set_attr(k, v):
	with _attr_lck:
		_attr[k] = v

def del_attr(k):
	with _attr_lck:
		try:
			v = _attr[k]
		except Exception:
			v = None

		try:
			del _attr[k]
		except Exception:
			pass

		return v

# note: this functionality should not be used inside this module
# continue to use the try: X except: X=Y hack
def checked(domain, k):
	with _checked_lck:
		k = 'common.checked.%s.%s' % (domain, k)
		v = _checked.get(k, False)
		_checked[k] = True
	return v

def sel(view, i=0):
	try:
		# view.sel() is a sublime.RegionSet. we want actual a python list behaviour :|
		l = [r for r in view.sel()]
		return l[i]
	except Exception:
		return sublime.Region(0, 0)

try:
	st2_status_message
except:
	sm_lck = threading.Lock()
	sm_task_counter = 0
	sm_tasks = {}
	sm_frame = 0
	sm_frames = (
		u'\u25D2',
		u'\u25D1',
		u'\u25D3',
		u'\u25D0'
	)
	sm_tm = datetime.datetime.now()
	sm_text = ''
	sm_set_text = ''

	st2_status_message = sublime.status_message
	sublime.status_message = status_message

	sched_sm_cb()

	DEVNULL = open(os.devnull, 'w')
	try:
		LOGFILE = open(home_path('log.txt'), 'a+')
	except Exception as ex:
		LOGFILE = DEVNULL
		notice(NAME, 'Cannot create log file. Remote(margo) and persistent logging will be disabled. Error: %s' % ex)

def gs_init():
	# init
	settings_obj().clear_on_change("GoSublime.settings")
	settings_obj().add_on_change("GoSublime.settings", sync_settings)
	sync_settings()
