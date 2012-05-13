import sublime, sublime_plugin
from sublime import set_timeout
import gscommon as gs
import re, threading, subprocess, time, traceback
from os import unlink, listdir, chdir, getcwd, devnull
from os.path import dirname, basename, join as pathjoin

LEADING_COMMENTS_PAT = re.compile(r'(^(\s*//.*?[\r\n]+|\s*/\*.*?\*/)+)', re.DOTALL)
PACKAGE_NAME_PAT = re.compile(r'package\s+(\w+)', re.UNICODE)
LINE_INDENT_PAT = re.compile(r'[\r\n]+[ \t]+')

class GsLint(sublime_plugin.EventListener):
    def on_close(self, view):
        if gs.is_go_source_view(view):
            vid = view.id()
            def cb():
                try:
                    del gs.l_vsyncs[vid]
                    del gs.l_lsyncs[vid]
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
        self.clear(True)

    def clear(self, clear_src=False):
        self.view_real_path = ""
        if clear_src:
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
        return

def describe_errors():
    view = gs.active_valid_go_view()
    if view:
        sel = view.sel()[0].begin()
        row, _ = view.rowcol(sel)
        gs.l_lsyncs[view.id()] = row
        er = gs.l_errors.get(view.id(), {}).get(row, ErrorReport(0, 0, ''))
        view.set_status('GsLint', ('GsLint: ' + er.err) if er.err else '')

def vsync():
    delay = 1000
    view = gs.active_valid_go_view()
    if view:
        if gs.setting('gslint_enabled', False):
            delay = 250
            vid = view.id()
            src = view.substr(sublime.Region(0, view.size()))
            tm = gs.l_vsyncs.get(vid, 0.0)
            if gs.l_lt.view_src != src:
                with gs.l_lt.sem:
                    gs.l_lt.view_src = src
                gs.l_vsyncs[vid] = time.time()
            elif tm > 0.0:
                timeout = int(gs.setting('gslint_timeout', 500))
                delta = int((time.time() - tm) * 1000.0)
                if delta >= timeout:
                    with gs.l_lt.sem:
                        gs.l_vsyncs[vid] = 0.0
                        gs.l_lt.view_real_path = view.file_name()
                        gs.l_lt.view_id = vid
                        gs.l_lt.view = view
                        gs.l_lt.notify()
                        gs.l_lt.cmd = gs.setting('gslint_cmd', [])
                    return

            row, _ = view.rowcol(view.sel()[0].begin())
            rw = gs.l_lsyncs.get(vid, -1)
            if row != rw:
                gs.l_lsyncs[vid] = row
                describe_errors()
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
    gs.l_lsyncs = {}
    gs.l_errors = {}

    vsync()

