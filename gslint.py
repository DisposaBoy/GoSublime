import sublime, sublime_plugin
from sublime import set_timeout
import gscommon as gs
import re, threading, subprocess, time, traceback
from os import unlink, listdir, chdir, getcwd
from os.path import dirname, basename, join as pathjoin

LEADING_COMMENTS_PAT = re.compile(r'(^(\s*//.*?[\r\n]+|\s*/\*.*?\*/)+)', re.DOTALL)
PACKAGE_NAME_PAT = re.compile(r'package\s+(\w+)', re.UNICODE)
LINE_INDENT_PAT = re.compile(r'[\r\n]+[ \t]+')

class GsLint(sublime_plugin.EventListener):
    def on_selection_modified(self, view):
        if gs.is_go_source_view(view):
            set_timeout(describe_errors, 0)
    
    def on_close(self, view):
        if gs.is_go_source_view(view):
            vid = view.id()
            def cb():
                try:
                    del gs.l_vsyncs[vid]
                    del gs.l_errors[vid]
                except:
                    pass
            set_timeout(cb, 0)

class ErrorReport(object):
    def __init__(self, row, col, err):
        self.row = row
        self.col = col
        self.err = err

class GsLintThread(threading.Thread):
    def __init__(self):
        threading.Thread.__init__(self)

        self.daemon = True
        self.stop_ev = threading.Event()
        self.ready_ev = threading.Event()
        self.sem = threading.Semaphore()
        self.clear()
    
    def clear(self):
        self.view_real_path = ""
        self.view_src = ""
        self.view_id = False
        self.view = None
        self.cmd = []
    
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

    def lint(self):
        err = ''
        out = ''
        with self.sem:
            cmd = self.cmd
            view_id = self.view_id
            real_path = self.view_real_path
            src = self.view_src.encode('UTF-8')
            view = self.view
            self.clear()

        if not (real_path and src):
            return
        
        pat_prefix = ''
        cwd = getcwd()
        pwd = dirname(real_path)
        chdir(pwd)
        real_fn = basename(real_path)
        # normalize the path so we can compare it below
        real_path = pathjoin(pwd, real_fn)
        tmp_path = pathjoin(pwd, '.GoSublime~tmp~%d~%s~' % (view_id, real_fn))
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

        errors = {}
        if err:
            err = LINE_INDENT_PAT.sub(' ', err)
            for m in re.finditer(r'%s[:](\d+)(?:[:](\d+))?[:](.+)$' % pat_prefix, err, re.MULTILINE):
                row = int(m.group(1))-1
                col = 0 if m.group(2) == '' else int(m.group(2))-1
                err = m.group(3).strip()
                errors[row] = ErrorReport(row, col, err)
        
        def cb():
            regions = []
            for k in errors:
                er = errors[k]
                line = view.line(view.text_point(er.row, 0))
                pos = line.begin() + er.col
                if pos >= line.end():
                    pos = line.end()
                regions.append(sublime.Region(pos, pos))
            gs.l_errors[view.id()] = errors
            if regions:
                flags = sublime.DRAW_EMPTY_AS_OVERWRITE
                view.add_regions('GsLint-errors', regions, 'invalid.illegal', 'cross', flags)
            else:
                view.erase_regions('GsLint-errors')
            # update the view so the error is displayed without needing to move the cursor
            describe_errors()
            set_timeout(vsync, 250)

        set_timeout(cb, 0)

def describe_errors():
    view = gs.active_valid_go_view()
    if view:
        sel = view.sel()[0].begin()
        row, _ = view.rowcol(sel)
        er = gs.l_errors.get(view.id(), {}).get(row, ErrorReport(0, 0, ''))
        view.set_status('GsLint', ('GsLint: ' + er.err) if er.err else '')

def vsync():
    delay = 1000
    view = gs.active_valid_go_view()
    if view:
        if gs.setting('gslint_enabled', False):
            delay = 250
            vid = view.id()
            tm, sz = gs.l_vsyncs.get(vid, (0.0, -1))
            if sz != view.size():
                gs.l_vsyncs[vid] = (time.time(), view.size())
            elif tm > 0.0 and sz == view.size():
                timeout = int(gs.setting('gslint_timeout', 500))
                delta = int((time.time() - tm) * 1000.0)
                if delta >= timeout:
                    gs.l_vsyncs[vid] = (0.0, view.size())
                    with gs.l_lt.sem:
                        gs.l_lt.view_real_path = view.file_name()
                        gs.l_lt.view_src = view.substr(sublime.Region(0, view.size()))
                        gs.l_lt.view_id = vid
                        gs.l_lt.view = view
                        gs.l_lt.notify()
                        gs.l_lt.cmd = gs.setting('gslint_cmd', [])
                    return
        else:
            delay = 5000
            gs.l_errors = {}
            describe_errors()
            view.erase_regions('GsLint-errors')
    set_timeout(vsync, delay)

try:
    # only init once
    gs.l_lt
except AttributeError:
    gs.l_lt = GsLintThread()
    gs.l_lt.start()
    gs.l_vsyncs = {}
    gs.l_errors = {}

    vsync()

