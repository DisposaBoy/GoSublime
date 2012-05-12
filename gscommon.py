import sublime
import subprocess, re, os, threading
from subprocess import Popen, PIPE

try:
	STARTUP_INFO = subprocess.STARTUPINFO()
	STARTUP_INFO.dwFlags |= subprocess.STARTF_USESHOWWINDOW
	STARTUP_INFO.wShowWindow = subprocess.SW_HIDE
except (AttributeError):
	STARTUP_INFO = None

_sem = threading.Semaphore()
_settings = {
	"env": {},
	"gscomplete_enabled": False,
	"gocode_cmd": "",
	"gofmt_cmd": "",
	"fmt_enabled": False,
	"gslint_enabled": False,
	"gslint_cmd": [],
	"gslint_timeout": 0,
	"autocomplete_snippets": False,
	"autocomplete_tests": False,
	"margo_cmd": [],
	"margo_addr": ""
}

GLOBAL_SNIPPETS = [
	(u'func\tfunc {...} \u0282', 'func ${1:name}($2)$3 {\n\t$0\n}'),
	(u'func\tfunc ([receiver]) {...} \u0282', 'func (${1:receiver} ${2:type}) ${3:name}($4)$5 {\n\t$0\n}'),
	(u'var\tvar (...) \u0282', 'var (\n\t$1\n)'),
	(u'const\tconst (...) \u0282', 'const (\n\t$1\n)'),
	(u'import\timport (...) \u0282', 'import (\n\t"$1"\n)'),
	(u'package\tpackage [name] \u0282', 'package ${1:NAME}')
]

LOCAL_SNIPPETS = [
	(u'func\tfunc{...}() \u0282', 'func($1) {\n\t$0\n}($2)'),
	(u'var\tvar [name] [type] \u0282', 'var ${1:name} ${2:type}'),
]

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

def runcmd(args, input=None, stdout=PIPE, stderr=PIPE, shell=False):
	out = ""
	err = ""
	exc = None

	old_env = os.environ.copy()
	os.environ.update(env())
	try:
		p = Popen(args, stdout=stdout, stderr=stderr, stdin=PIPE,
			startupinfo=STARTUP_INFO, shell=shell)
		if isinstance(input, unicode):
			input = input.encode('utf-8')
		out, err = p.communicate(input=input)
		out = out.decode('utf-8') if out else ''
		err = err.decode('utf-8') if err else ''
	except (Exception) as e:
		err = u'Error while running %s: %s' % (args[0], e)
		exc = e
	os.environ.update(old_env)
	return (out, err, exc)

def settings_obj():
	return sublime.load_settings("GoSublime.sublime-settings")

def setting(key, default=None):
	with _sem:
		return _settings.get(key, default)

def notice(domain, txt):
	txt = "** %s: %s **" % (domain, txt)
	print(txt)
	sublime.set_timeout(lambda: sublime.status_message(txt), 0)

def notice_undo(domain, txt, view, should_undo):
	def cb():
		if should_undo:
			view.run_command('undo')
		notice(domain, txt)
	sublime.set_timeout(cb, 0)

def is_go_source_view(view):
	return view.score_selector(view.sel()[0].begin(), 'source.go') > 0

def active_valid_go_view(win=None):
	if not win:
		win = sublime.active_window()
	if win:
		view = win.active_view()
		if view and view.file_name() and is_go_source_view(view):
			return view
	return None

def rowcol(view):
	return view.rowcol(view.sel()[0].begin())

def env():
	e = os.environ.copy()
	e.update(setting('env', {}))

	roots = e.get('GOPATH', '').split(os.pathsep)
	roots.append(e.get('GOROOT', ''))
	add_path = e.get('PATH', '').split(os.pathsep)
	for s in roots:
		if s:
			s = os.path.join(s, 'bin')
			if s not in add_path:
				add_path.append(s)
	e['PATH'] = os.pathsep.join(add_path)
	return e

def sync_settings():
	global _settings
	so = settings_obj()
	with _sem:
		for k in _settings:
			v = so.get(k, None)
			if v is not None:
				# todo: check the type of `v`
				_settings[k] = v
		e = {}
		for k, v in _settings.get('env', {}).iteritems():
			e[k] = str(os.path.expandvars(os.path.expanduser(v)))
		_settings['env'] = e


settings_obj().clear_on_change("GoSublime.settings")
settings_obj().add_on_change("GoSublime.settings", sync_settings)
sync_settings()
