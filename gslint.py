import sublime, sublime_plugin
import gscommon as gs
import re, threading, subprocess, time, traceback, Queue
from os import unlink, listdir, chdir, getcwd
from os.path import dirname, basename, join as pathjoin

LEADING_COMMENTS_PAT = re.compile(r'(^(\s*//.*?[\r\n]+|\s*/\*.*?\*/)+)', re.DOTALL)
PACKAGE_NAME_PAT = re.compile(r'package\s+(\w+)', re.UNICODE)
LINE_INDENT_PAT = re.compile(r'[\r\n]+[ \t]+')

class GsLint(sublime_plugin.EventListener):
    _q = Queue.Queue()

    _errors = {}
    _errors_sem = threading.Semaphore()

    _linters = {}
    _linters_sem = threading.Semaphore()

    def __del__(self):
        for lt in self._linters:
            lt.stop()

    def errors(self, view, default={}):
        with self._errors_sem:
            return self._errors.get(view.id(), default)

    def set_errors(self, view, errors):
        with self._errors_sem:
            self._errors[view.id()] = errors

    def linter(self, view):
        with self._linters_sem:
            lt = self._linters.get(view.id())
            if not lt or not lt.is_alive():
                lt = GsLintThread(self, view)
                self._linters[view.id()] = lt
                lt.start()
            return lt

    def check(self, view):
        sel = view.sel()[0].begin()
        if view.score_selector(sel, 'source.go') > 0:
            def cb():
                self._q.get_nowait()
                if self._q.empty():
                    self.linter(view).notify()
            
            self._q.put(True)
            sublime.set_timeout(cb, int(gs.setting('gslint_timeout', 500)))

    def on_selection_modified(self, view):
        sel = view.sel()[0].begin()
        if view.score_selector(sel, 'source.go') > 0:
            line = view.rowcol(sel)[0]
            msg = self.errors(view).get(line, '')
            view.set_status('GsLint', ('GsLint: ' + msg) if msg else '')

    def on_modified(self, view):
        self.check(view)

    def on_load(self, view):
        self.check(view)

    def on_activated(self, view):
        self.check(view)

    def on_close(self, view):
        self.linter(view).stop()
        self.set_errors(view, {})

class GsLintThread(threading.Thread):
    ignored_scopes = frozenset([
        'string.quoted.double.go',
        'string.quoted.single.go',
        'string.quoted.raw.go',
        'comment.line.double-slash.go',
        'comment.block.go'
    ])

    def __init__(self, gslint, view):
        threading.Thread.__init__(self)

        self.daemon = True
        self.gslint = gslint
        self.view = view
        self.stop_ev = threading.Event()
        self.ready_ev = threading.Event()
    
    def notify(self):
        self.ready_ev.set()

    def stop(self):
        self.stop_ev.set()

    def run(self):
        while not self.stop_ev.is_set():
            try:
                self.ready_ev.wait()
                self.ready_ev.clear()
                self.lint()
            except Exception:
                gs.notice("GsLintThread: Loop", traceback.format_exc())
            # updat the view so the error is displayed without needing to move the cursor
            self.gslint.on_selection_modified(self.view)

    def lint(self):
        view = self.view
        pos = view.sel()[0].begin()
        scopes = set(self.view.scope_name(pos).split())
        if 'source.go' in scopes and self.ignored_scopes.isdisjoint(scopes):
            err = ''
            out = ''
            cmd = gs.setting('gslint_cmd', 'gotype')
            real_path = view.file_name()
            src = view.substr(sublime.Region(0, view.size())).encode('utf-8')

            if not real_path or not src:
                return
            
            pat_prefix = ''
            cwd = getcwd()
            pwd = dirname(real_path)
            chdir(pwd)
            real_fn = basename(real_path)
            # normalize the path so we can compare it below
            real_path = pathjoin(pwd, real_fn)
            tmp_path = pathjoin(pwd, '.GoSublime~tmp~%d~%s~' % (view.id(), real_fn))
            try:
                real_fn_lower = real_fn.lower()
                x = gs.GOOSARCHES_PAT.match(real_fn_lower)
                x = x.groups() if x else None
                if x and cmd:
                    files = [tmp_path]
                    for fn in listdir(pwd):
                        fn_lower = fn.lower()
                        y = gs.GOOSARCHES_PAT.match(fn_lower)
                        y = y.groups() if y else None
                        if y and fn_lower != real_fn_lower:
                            path = pathjoin(pwd, fn)
                            # attempt to resolve any os-specific file names...
                            # [0] => fn prefix, [1] => os, [2] => arch
                            if (x[0] != y[0]) or (x[1] == y[1] and (not x[2] or not y[2] or x[2] == y[2])):
                                files.append(path)
                    pkg = 'main'
                    m = LEADING_COMMENTS_PAT.match(src)
                    m = PACKAGE_NAME_PAT.search(src, m.end(1) if m else 0)
                    if m:
                        pat_prefix = '^' + re.escape(tmp_path)
                        with open(tmp_path, 'wb') as f:
                            f.write(src)
                                                
                        t = {
                            "$pkg": [m.group(1)],
                            "$files": files,
                            "$path": tmp_path,
                            "$real_path": real_path
                        }
                        args = []
                        for i in list(cmd):
                            args.extend(t.get(i, [i]))
                        out, err = gs.runcmd(args)
                        unlink(tmp_path)
            except Exception:
                gs.notice("GsLintThread: Cmd", traceback.format_exc())
            
            chdir(cwd)

            regions = []
            errors = {}

            if err:
                err = LINE_INDENT_PAT.sub(' ', err)
                for m in re.finditer(r'%s[:](\d+)(?:[:](\d+))?[:](.+)$' % pat_prefix, err, re.MULTILINE):
                    line = int(m.group(1))-1
                    start = 0 if m.group(2) == '' else int(m.group(2))-1
                    err = m.group(3).strip()
                    errors[line] = err
                    pos = view.line(view.text_point(line, 0)).begin() + start
                    if pos >= view.size():
                        pos = view.size() - 1
                    regions.append(sublime.Region(pos, pos))
            self.gslint.set_errors(view, errors)
            if regions:
                flags = sublime.DRAW_EMPTY_AS_OVERWRITE
                view.add_regions('GsLint-errors', regions, 'invalid.illegal', 'cross', flags)
            else:
                view.erase_regions('GsLint-errors')
