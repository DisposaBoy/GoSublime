import sublime, sublime_plugin
import gscommon as gs
import re, threading, subprocess
from os import unlink, listdir, chdir, getcwd
from os.path import dirname, basename, join as pathjoin

LEADING_COMMENTS_PAT = re.compile(r'(^(\s*//.*?[\r\n]+|\s*/\*.*?\*/)+)', re.DOTALL)
PACKAGE_NAME_PAT = re.compile(r'package\s+(\w+)', re.UNICODE)
LINE_INDENT_PAT = re.compile(r'[\r\n]+[ \t]+')

class GsLint(sublime_plugin.EventListener):
    rc = 0
    errors = {}

    def on_selection_modified(self, view):
        sel = view.sel()[0].begin()
        if view.score_selector(sel, 'source.go') > 0:
            line = view.rowcol(sel)[0]
            msg = self.errors.get(view.id(), {}).get(line, '')
            view.set_status('GsLint', ('GsLint: ' + msg) if msg else '')
    
    def on_modified(self, view):
        pos = view.sel()[0].begin()
        scopes = view.scope_name(pos).split()
        if 'source.go' in scopes:
            self.rc += 1

            should_run = (
                         'string.quoted.double.go' not in scopes and
                         'string.quoted.single.go' not in scopes and
                         'string.quoted.raw.go' not in scopes and
                         'comment.line.double-slash.go' not in scopes and
                         'comment.block.go' not in scopes
            )

            def cb():
                self.lint(view)
            
            if should_run:
                sublime.set_timeout(cb, int(gs.setting('gslint_timeout', 500)))
            else:
                # we want to cleanup if e.g settings changed or we caused an error entering an excluded scope
                sublime.set_timeout(cb, 1000)

    def on_load(self, view):
        self.on_modified(view)

    def on_activated(self, view):
        self.on_modified(view)

    def on_close(self, view):
        try:
            del self.errors[view.id()]
        except KeyError:
            pass
    
    def lint(self, view):
        self.rc -= 1
        if self.rc == 0:
            err = ''
            out = ''
            cmd = gs.setting('gslint_cmd', 'gotype')
            real_path = view.file_name()
            
            if not real_path:
                return
            
            pat_prefix = ''
            cwd = getcwd()
            pwd = dirname(real_path)
            chdir(pwd)
            fn = basename(real_path)
            # normalize the path so we can compare it below
            real_path = pathjoin(pwd, fn)
            tmp_path = pathjoin(pwd, '.GoSublime~tmp~%d~%s~' % (view.id(), fn))
            try:
                if cmd:
                    files = []
                    if real_path:
                        for fn in listdir(pwd):
                            if fn.lower().endswith('.go'):
                                fn = pathjoin(pwd, fn)
                                if fn != real_path:
                                    files.append(fn)

                    src = view.substr(sublime.Region(0, view.size())).encode('utf-8')
                    pkg = 'main'
                    m = LEADING_COMMENTS_PAT.match(src)
                    m = PACKAGE_NAME_PAT.search(src, m.end(1) if m else 0)
                    if m:
                        pat_prefix = '^' + re.escape(tmp_path)
                        with open(tmp_path, 'wb') as f:
                            f.write(src)
                        files.append(tmp_path)
                        
                        t = {
                            "$pkg": [m.group(1)],
                            "$files": files
                        }
                        args = []
                        for i in list(cmd):
                            args.extend(t.get(i, [i]))
                        out, err = gs.runcmd(args)
                        unlink(tmp_path)
                    else:
                        sublime.status_message('Cannot find PackageName')
            except Exception as e:
                sublime.status_message(str(e))

            chdir(cwd)

            regions = []
            view_id = view.id()
            self.errors[view_id] = {}

            if err:
                err = LINE_INDENT_PAT.sub(' ', err)
                self.errors[view_id] = {}
                for m in re.finditer(r'%s[:](\d+)(?:[:](\d+))?[:](.+)$' % pat_prefix, err, re.MULTILINE):
                    line = int(m.group(1))-1
                    start = 0 if m.group(2) == '' else int(m.group(2))-1
                    err = m.group(3).strip()
                    self.errors[view_id][line] = err
                    pos = view.line(view.text_point(line, 0)).begin() + start
                    if pos >= view.size():
                        pos = view.size() - 1
                    regions.append(sublime.Region(pos, pos))
                
                if len(self.errors[view_id]) == 0:
                    sublime.status_message(out + '\n' + err)
            if regions:
                flags = sublime.DRAW_EMPTY_AS_OVERWRITE
                view.add_regions('GsLint-errors', regions, 'invalid.illegal', 'cross', flags)
            else:
                view.erase_regions('GsLint-errors')
        self.on_selection_modified(view)
