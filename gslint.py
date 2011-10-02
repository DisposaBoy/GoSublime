import sublime, sublime_plugin
import gscommon as gs
import re, threading

LINE_PAT = re.compile(r':(\d+):(\d+):\s+(.+)\s*$', re.MULTILINE)

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
    
    def lint(self, view):
        self.rc -= 1

        if self.rc == 0:
            cmd = gs.setting('gslint_cmd', 'gotype')
            if cmd:
                _, err = gs.runcmd([cmd], view.substr(sublime.Region(0, view.size())))
            else:
                err = ''
            lines = LINE_PAT.findall(err)
            regions = []
            view_id = view.id()        
            self.errors[view_id] = {}
            if lines:
                for m in lines:
                    line, start, err = int(m[0])-1, int(m[1])-1, m[2]
                    self.errors[view_id][line] = err
                    lr = view.line(view.text_point(line, start))
                    regions.append(sublime.Region(lr.begin() + start, lr.end()))
            if regions:
                flags = sublime.DRAW_EMPTY_AS_OVERWRITE | sublime.DRAW_OUTLINED
                flags = sublime.DRAW_EMPTY_AS_OVERWRITE
                flags = sublime.DRAW_OUTLINED
                view.add_regions('GsLint-errors', regions, 'invalid.illegal', 'cross', flags)
            else:
                view.erase_regions('GsLint-errors')
        self.on_selection_modified(view)
