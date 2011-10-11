import sublime, sublime_plugin
import gscommon as gs
import re, threading
from os import unlink, listdir
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
            cmd = gs.setting('gslint_cmd', 'gotype')
            real_path = view.file_name()
            pat_prefix = ''
            pwd = dirname(real_path)
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
                    if files:
                        # m = LEADING_COMMENTS_PAT.sub('', src)
                        m = LEADING_COMMENTS_PAT.match(src)
                        m = PACKAGE_NAME_PAT.search(src, m.end(1) if m else 0)
                        if m:
                            pat_prefix = '^' + re.escape(tmp_path)
                            with open(tmp_path, 'wb') as f:
                                f.write(src)
                            args = [cmd, '-p', m.group(1), tmp_path]
                            args.extend(files)
                            _, err = gs.runcmd(args)
                            unlink(tmp_path)
                        else:
                            sublime.status_message('Cannot find PackageName')
                    else:
                        _, err = gs.runcmd([cmd], src)
            except Exception as e:
                sublime.status_message(str(e))

            regions = []
            view_id = view.id()
            self.errors[view_id] = {}

            if err:
                err = LINE_INDENT_PAT.sub(' ', err)
                for m in re.finditer(r'%s:(\d+):(\d+):\s+(.+)\s*$' % pat_prefix, err, re.MULTILINE):
                    line, start, err = int(m.group(1))-1, int(m.group(2))-1, m.group(3)
                    self.errors[view_id][line] = err
                    pos = view.line(view.text_point(line, 0)).begin() + start
                    if pos >= view.size():
                        pos = view.size() - 1
                    regions.append(sublime.Region(pos, pos))

            if regions:
                flags = sublime.DRAW_EMPTY_AS_OVERWRITE
                view.add_regions('GsLint-errors', regions, 'invalid.illegal', 'cross', flags)
            else:
                view.erase_regions('GsLint-errors')
        self.on_selection_modified(view)
